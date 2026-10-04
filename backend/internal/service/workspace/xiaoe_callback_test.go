package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/xiaoemcp"
)

type xiaoeCallbackRepository struct {
	application.Repository
	connectorPackageRepository
	flow     domain.ConnectorAuthorizationAttempt
	revision domain.ConnectorRevision
	updates  int
}

func (r *xiaoeCallbackRepository) ListConnectorInstallations(_ context.Context, owner string) ([]domain.ConnectorInstallation, error) {
	if owner != r.flow.OwnerID {
		return nil, domain.ErrNotFound
	}
	return []domain.ConnectorInstallation{{ID: r.flow.InstallationID, OwnerID: owner, ActiveRevisionID: r.revision.ID, State: domain.ConnectorInstallationActive}}, nil
}
func (r *xiaoeCallbackRepository) GetConnectorRevision(context.Context, string) (domain.ConnectorRevision, error) {
	return r.revision, nil
}
func (r *xiaoeCallbackRepository) GetConnectorAuthorizationFlow(_ context.Context, owner, id string) (domain.ConnectorAuthorizationAttempt, error) {
	if owner != r.flow.OwnerID || id != r.flow.ID || !r.flow.ExpiresAt.After(time.Now()) {
		return domain.ConnectorAuthorizationAttempt{}, domain.ErrNotFound
	}
	return r.flow, nil
}
func (r *xiaoeCallbackRepository) UpdateConnectorAuthorizationFlow(_ context.Context, owner, id string, previous, next []byte, action string) error {
	if owner != r.flow.OwnerID || id != r.flow.ID || !bytes.Equal(previous, r.flow.DeviceCodeCiphertext) {
		return domain.ErrConflict
	}
	r.flow.DeviceCodeCiphertext = append([]byte(nil), next...)
	r.flow.ActionURL = action
	r.updates++
	return nil
}
func TestXiaoeCallbackBindsOwnerFlowAndEncryptedPKCEState(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := json.Marshal(xiaoemcp.Pending{ClientID: "client", RedirectURI: "https://workspace.test" + xiaoeOAuthCallbackPath, Verifier: strings.Repeat("v", 43), ExpiresAt: time.Now().Add(time.Minute)})
	cipher, err := box.Encrypt(pending, connectorAuthorizationFlowAAD("owner", "installation", "user"))
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := json.Marshal(connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "xiaoe"}, MCP: &connectorpackage.MCPManifest{Transport: "streamable_http", URL: xiaoemcp.Resource, EgressHosts: []string{"agent.xiaoe-tech.com"}}})
	repository := &xiaoeCallbackRepository{flow: domain.ConnectorAuthorizationAttempt{ID: "flow", OwnerID: "owner", InstallationID: "installation", Identity: "user", ActionURL: xiaoemcp.Issuer + "/oauth/authorize?client_id=client", DeviceCodeCiphertext: cipher, ExpiresAt: time.Now().Add(time.Minute)}, revision: domain.ConnectorRevision{ID: "revision", RuntimePolicy: policy}}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{box: box, workspace: app}
	flow, err := service.sealXiaoeCallback(context.Background(), repository, repository.flow)
	if err != nil {
		t.Fatal(err)
	}
	action, _ := url.Parse(flow.ActionURL)
	state := action.Query().Get("state")
	if strings.Contains(flow.ActionURL, strings.Repeat("v", 43)) || strings.Contains(state, "owner") {
		t.Fatal("plaintext PKCE/owner leaked into URL")
	}
	callback := func(values url.Values) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, xiaoeOAuthCallbackPath+"?"+values.Encode(), nil)
		service.xiaoeOAuthCallback(w, req)
		return w
	}
	good := url.Values{"state": {state}, "code": {"code-canary"}, "iss": {xiaoemcp.Issuer}}
	for _, test := range []struct{ name, key, value string }{{"tampered", "state", state[:len(state)-4] + "AAAA"}, {"issuer", "iss", "https://attacker.test"}, {"empty code", "code", ""}} {
		t.Run(test.name, func(t *testing.T) {
			q := url.Values{}
			for k, v := range good {
				q[k] = append([]string(nil), v...)
			}
			q.Set(test.key, test.value)
			w := callback(q)
			if w.Code != 400 || repository.updates != 1 || strings.Contains(w.Body.String(), "code-canary") {
				t.Fatal("invalid callback accepted or leaked code")
			}
		})
	}
	w := callback(good)
	if w.Code != 303 || w.Header().Get("Location") != "/resources?tab=connectors&xiaoe_auth=flow" || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe callback response: %d", w.Code)
	}
	decoded, err := box.Decrypt(repository.flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD("owner", "installation", "user"))
	if err != nil {
		t.Fatal(err)
	}
	var accepted xiaoemcp.Pending
	if json.Unmarshal(decoded, &accepted) != nil || accepted.Code != "code-canary" {
		t.Fatal("callback did not encrypt code for original owner")
	}
	if _, err := box.Decrypt(repository.flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD("other-owner", "installation", "user")); err == nil {
		t.Fatal("another owner can decrypt callback")
	}
	if w := callback(good); w.Code != 400 || repository.updates != 2 {
		t.Fatal("callback replay accepted")
	}
	// A newly started flow supersedes the sealed reference of the previous one.
	repository.flow.ID = "new-flow"
	if w := callback(good); w.Code != 400 {
		t.Fatal("superseded callback accepted")
	}
}

func TestXiaoeRefreshMaterialStaysOutsideRuntimeCredential(t *testing.T) {
	policy := connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "xiaoe"}, MCP: &connectorpackage.MCPManifest{Transport: "streamable_http", URL: xiaoemcp.Resource, EgressHosts: []string{"agent.xiaoe-tech.com"}}}
	fields := connectorAuthorizationCredentialFields(policy, connectorAuthorizationGrant{AccessToken: "access", RefreshToken: "platform-only-refresh", ClientID: "registered-client", ExpiresAt: time.Now().Add(time.Hour)})
	if len(fields) != 3 || fields["MCP_BEARER_TOKEN"] != "access" || fields["client_id"] != "registered-client" || fields["refresh_token"] != "" {
		t.Fatal("refresh material entered Runtime JSON")
	}
	if connectorAuthorizationMode(policy) != "interactive" {
		t.Fatal("OAuth package uses provided-token path")
	}
	policy.AuthMode = "cli"
	if connectorAuthorizationMode(policy) != "provided" {
		t.Fatal("historical package was silently converted")
	}
}

func TestXiaoeLoginRejectsEndpointOrPolicySubstitution(t *testing.T) {
	base := connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "xiaoe"}, MCP: &connectorpackage.MCPManifest{Transport: "streamable_http", URL: xiaoemcp.Resource, EgressHosts: []string{"agent.xiaoe-tech.com"}}}
	if connectorAuthorizationMode(base) != "interactive" || validateProvidedConnectorCredentials(base, nil) == nil {
		t.Fatal("Xiaoe accepted provided credentials")
	}
	for _, mutate := range []func(*connectorRevisionPolicy){
		func(p *connectorRevisionPolicy) { p.MCP.URL = "https://attacker.test/mcp" },
		func(p *connectorRevisionPolicy) { p.MCP.Headers = map[string]string{"Authorization": "${TOKEN}"} },
		func(p *connectorRevisionPolicy) {
			p.MCP.EgressHosts = []string{"agent.xiaoe-tech.com", "attacker.test"}
		},
		func(p *connectorRevisionPolicy) { p.CLI = &connectorpackage.CLIManifest{AuthenticationDriver: "none"} },
	} {
		p := base
		m := *base.MCP
		p.MCP = &m
		mutate(&p)
		if isXiaoeMCPLoginPolicy(p) {
			t.Fatal("substituted policy accepted")
		}
		if validatePlatformConnectorPackage(connectorpackage.Package{Metadata: p.Metadata, MCP: p.MCP, CLI: p.CLI}) == nil {
			t.Fatal("unsafe Xiaoe publication accepted")
		}
	}
	if validatePrivateConnectorPackage(connectorpackage.Package{Metadata: base.Metadata, MCP: base.MCP}) == nil {
		t.Fatal("private package claimed the provider adapter")
	}
}
