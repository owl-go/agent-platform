package workspace

import (
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/klingmcp"
	"agent-platform/backend/internal/secretcrypto"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestKlingCallbackBindsOwnerFlowAndEncryptedPKCEState(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := json.Marshal(klingmcp.Pending{ClientID: "client", RedirectURI: "https://workspace.test" + klingOAuthCallbackPath, Verifier: strings.Repeat("v", 43), ExpiresAt: time.Now().Add(time.Minute)})
	cipher, err := box.Encrypt(pending, connectorAuthorizationFlowAAD("owner", "installation", "user"))
	if err != nil {
		t.Fatal(err)
	}
	policy, _ := json.Marshal(connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "kling-ai"}, MCP: &connectorpackage.MCPManifest{Transport: "streamable_http", URL: klingmcp.Resource, EgressHosts: []string{"klingai.com"}}})
	repository := &teambitionCallbackRepository{flow: domain.ConnectorAuthorizationAttempt{ID: "flow", OwnerID: "owner", InstallationID: "installation", Identity: "user", ActionURL: klingmcp.Issuer + "/authorize?client_id=client", DeviceCodeCiphertext: cipher, ExpiresAt: time.Now().Add(time.Minute)}, revision: domain.ConnectorRevision{ID: "revision", RuntimePolicy: policy}}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{box: box, workspace: app}
	flow, err := service.sealBrowserOAuthCallback(context.Background(), repository, repository.flow, klingBrowserOAuth)
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
		req := httptest.NewRequest(http.MethodGet, klingOAuthCallbackPath+"?"+values.Encode(), nil)
		service.klingOAuthCallback(w, req)
		return w
	}
	good := url.Values{"state": {state}, "code": {"code-canary"}, "iss": {klingmcp.Issuer}}
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
	good.Del("iss") // Kling metadata does not require RFC 9207 issuer responses.
	w := callback(good)
	if w.Code != 303 || w.Header().Get("Location") != "/resources?tab=connectors&connector_auth=flow" || w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe callback response: %d", w.Code)
	}
	decoded, err := box.Decrypt(repository.flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD("owner", "installation", "user"))
	if err != nil {
		t.Fatal(err)
	}
	var accepted klingmcp.Pending
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

func TestKlingOAuthPolicyAndRuntimeCredentialsAreNarrow(t *testing.T) {
	policy := connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "kling-ai"}, MCP: &connectorpackage.MCPManifest{Transport: "streamable_http", URL: klingmcp.Resource, EgressHosts: []string{"klingai.com"}}}
	if connectorAuthorizationMode(policy) != "interactive" {
		t.Fatal("MCP fell back to manual credentials")
	}
	fields := connectorAuthorizationCredentialFields(policy, connectorAuthorizationGrant{AccessToken: "access", RefreshToken: "platform-only-refresh", ClientID: "client"})
	if len(fields) != 2 || fields["MCP_BEARER_TOKEN"] != "access" || fields["refresh_token"] != "" {
		t.Fatal("unsafe MCP credentials")
	}
	if err := validateProvidedConnectorCredentials(policy, nil); err == nil {
		t.Fatal("manual token bypass accepted")
	}
	policy.MCP.URL = "https://attacker.test/mcp"
	if isKlingMCPLoginPolicy(policy) {
		t.Fatal("unreviewed endpoint entered OAuth")
	}
	policy.MCP.URL = klingmcp.Resource
	policy.MCP.Headers = map[string]string{"Authorization": "${OTHER_TOKEN}"}
	if isKlingMCPLoginPolicy(policy) {
		t.Fatal("package header can override OAuth")
	}
}
