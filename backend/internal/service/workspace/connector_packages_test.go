package workspace

import (
	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
)

func TestConnectorInstallationResponseDoesNotTreatExpiredAuthorizationAsAuthorized(t *testing.T) {
	item := domain.ConnectorInstallation{ID: "installation-1", AuthorizationID: "expired-authorization", Authorized: false}
	if response := connectorInstallationResponse(item); response.Authorized {
		t.Fatal("expired authorization was reported as active")
	}
	item.Authorized = true
	if response := connectorInstallationResponse(item); !response.Authorized {
		t.Fatal("active authorization was reported as inactive")
	}
}

func TestConnectorRevisionFromPackageUsesImmutableChecksumKey(t *testing.T) {
	pkg := connectorpackage.Package{Metadata: connectorpackage.Metadata{Source: "example", Version: "1.2.3", Type: connectorpackage.TypeMCP, AuthMode: "oauth"}, SHA256: strings.Repeat("a", 64)}
	revision, key := connectorRevisionFromPackage(pkg)
	if revision.PackageSource != "example" || revision.Version != "1.2.3" || key != "connectors/example/1.2.3/"+strings.Repeat("a", 64)+".zip" {
		t.Fatalf("unexpected revision=%#v key=%q", revision, key)
	}
	var policy struct {
		AuthMode string `json:"auth_mode"`
	}
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.AuthMode != "oauth" {
		t.Fatalf("runtime policy = %s, error = %v", revision.RuntimePolicy, err)
	}
}

func TestBuildGuidedConnectorPackageProducesParserCompatibleZIP(t *testing.T) {
	archive, err := buildGuidedConnectorPackage(guidedConnectorInput{Source: "example", Version: "1.0.0", Type: "mcp", Name: "Example", Description: "Example", AuthMode: "none", MCPJSON: `{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30}`, SkillName: "query", SkillMarkdown: "# Query\nUse the tool."})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 4 {
		t.Fatalf("files = %d", len(reader.File))
	}
	if _, err := connectorpackage.Parse(archive); err != nil {
		t.Fatalf("guided archive does not pass shared validator: %v", err)
	}
	if !strings.Contains(string(mustZipFile(t, reader, "connector-meta.json")), `"source":"example"`) {
		t.Fatal("metadata missing")
	}
}

func TestPrivateConnectorPackageRejectsPlatformAuthenticationDriver(t *testing.T) {
	pkg := connectorpackage.Package{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "feishu"}}
	if err := validatePrivateConnectorPackage(pkg); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("validatePrivateConnectorPackage() error = %v", err)
	}
	pkg.CLI.AuthenticationDriver = "connector_package"
	if err := validatePrivateConnectorPackage(pkg); err != nil {
		t.Fatalf("generic private credentials were rejected: %v", err)
	}
	pkg.CLI.AuthenticationDriver = "dingtalk"
	if err := validatePrivateConnectorPackage(pkg); err == nil {
		t.Fatal("DingTalk platform authentication driver was accepted for a private package")
	}
}

func TestInteractiveConnectorAuthorizationRejectsOtherDrivers(t *testing.T) {
	for _, driver := range []string{"connector_package", "none"} {
		t.Run(driver, func(t *testing.T) {
			policy := connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: driver}}
			if err := validateInteractiveConnectorDriver(policy); err == nil || !strings.Contains(err.Error(), "no interactive authorization adapter") {
				t.Fatalf("driver %q error = %v", driver, err)
			}
		})
	}
	if err := validateInteractiveConnectorDriver(connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "feishu"}}); err != nil {
		t.Fatalf("Feishu driver rejected: %v", err)
	}
	if err := validateInteractiveConnectorDriver(connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "dingtalk"}}); err != nil {
		t.Fatalf("DingTalk driver rejected: %v", err)
	}
	if err := validateInteractiveConnectorDriver(connectorRevisionPolicy{MCP: &connectorpackage.MCPManifest{}}); err == nil {
		t.Fatal("MCP revision entered the CLI authorization flow")
	}
}

func TestProvidedConnectorCredentialsFollowReviewedPolicy(t *testing.T) {
	manual := connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{
		AuthenticationDriver: "connector_package",
		Capabilities:         []connectorpackage.CLICapability{{Scopes: []string{"tasks:write"}}},
	}}
	if err := validateProvidedConnectorCredentials(manual, []string{"tasks:write"}); err != nil {
		t.Fatalf("reviewed scope rejected: %v", err)
	}
	if err := validateProvidedConnectorCredentials(manual, []string{"admin:write"}); err == nil {
		t.Fatal("unreviewed scope accepted")
	}
	if err := validateProvidedConnectorCredentials(connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "feishu"}}, nil); err == nil {
		t.Fatal("provider-managed authorization accepted arbitrary JSON")
	}
	if err := validateProvidedConnectorCredentials(connectorRevisionPolicy{AuthMode: "oauth", MCP: &connectorpackage.MCPManifest{}}, nil); err != nil {
		t.Fatalf("MCP credentials rejected: %v", err)
	}
	if err := validateProvidedConnectorCredentials(connectorRevisionPolicy{AuthMode: "none", MCP: &connectorpackage.MCPManifest{}}, nil); err == nil {
		t.Fatal("unauthenticated MCP accepted credentials")
	}
}

func TestConnectorAuthorizationModeUsesRevisionPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy connectorRevisionPolicy
		want   string
	}{
		{"Feishu device flow", connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "feishu"}}, "interactive"},
		{"Notion CLI browser login", connectorRevisionPolicy{Metadata: connectorpackage.Metadata{Source: "notion", Version: "0.23.13"}, CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "connector_package"}}, "interactive"},
		{"reviewed CLI credentials", connectorRevisionPolicy{CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "connector_package"}}, "provided"},
		{"reviewed MCP credentials", connectorRevisionPolicy{AuthMode: "oauth", MCP: &connectorpackage.MCPManifest{}}, "provided"},
		{"no authorization", connectorRevisionPolicy{AuthMode: "none", MCP: &connectorpackage.MCPManifest{}}, "none"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := connectorAuthorizationMode(test.policy); got != test.want {
				t.Fatalf("mode = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNotionBrowserLoginStoresRuntimeTokenWithoutRefreshMetadata(t *testing.T) {
	policy := connectorRevisionPolicy{Metadata: connectorpackage.Metadata{Source: "notion"}, CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "connector_package"}}
	fields := connectorAuthorizationCredentialFields(policy, connectorAuthorizationGrant{AccessToken: "ntn_secret-value"})
	if len(fields) != 1 || fields["token"] != "ntn_secret-value" {
		t.Fatalf("Notion runtime credential fields = %v", fields)
	}
}

func TestConnectorRevisionResponseUsesActivationScopes(t *testing.T) {
	pkg := connectorpackage.Package{
		Metadata: connectorpackage.Metadata{Source: "feishu", Version: "1.0.95", Type: connectorpackage.TypeCLI, Name: "飞书", AuthMode: "oauth"},
		CLI: &connectorpackage.CLIManifest{
			AuthenticationDriver: "feishu",
			ActivationScopes:     []string{"docx:document:create"},
			Capabilities: []connectorpackage.CLICapability{
				{ID: "docs_create", Identities: []string{"user"}, Scopes: []string{"docx:document:create"}},
				{ID: "mail_send", Identities: []string{"user"}, Scopes: []string{"mail:mail:write"}},
			},
		},
	}
	revision, _ := connectorRevisionFromPackage(pkg)
	response := connectorRevisionResponse(revision)
	if len(response.RequiredScopes) != 1 || response.RequiredScopes[0] != "docx:document:create" {
		t.Fatalf("activation scopes = %v", response.RequiredScopes)
	}
}

func TestNotionConnectorResponsesUseProductNameForExistingRevision(t *testing.T) {
	pkg := connectorpackage.Package{Metadata: connectorpackage.Metadata{Source: "notion", Version: "0.23.13", Type: connectorpackage.TypeCLI, Name: "Notion CLI", Description: "Read pages with the pinned Notion CLI", AuthMode: "cli"}}
	revision, _ := connectorRevisionFromPackage(pkg)
	if got := connectorRevisionResponse(revision); got.Name != "Notion" || got.Description != "Read and manage Notion pages and query data sources" {
		t.Fatalf("Notion catalog display = %q, %q", got.Name, got.Description)
	}
	installation := connectorInstallationDetailsResponse(domain.ConnectorInstallation{PackageSource: "notion"}, revision, false)
	if installation.Name != "Notion" || installation.Description != "Read and manage Notion pages and query data sources" {
		t.Fatalf("Notion installation display = %q, %q", installation.Name, installation.Description)
	}
}

func mustZipFile(t *testing.T, reader *zip.Reader, name string) []byte {
	t.Helper()
	for _, entry := range reader.File {
		if entry.Name == name {
			stream, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer stream.Close()
			var buffer bytes.Buffer
			_, _ = buffer.ReadFrom(stream)
			return buffer.Bytes()
		}
	}
	t.Fatalf("missing %s", name)
	return nil
}

func TestConnectorDetailsExposeExamplesFromExactRevision(t *testing.T) {
	pkg := connectorpackage.Package{Metadata: connectorpackage.Metadata{Source: "example", Version: "1.0.0", Type: connectorpackage.TypeMCP, Name: "Example", ExamplesZH: []string{"查询示例数据"}, ExamplesEN: []string{"Query example data"}}, SHA256: strings.Repeat("a", 64)}
	revision, _ := connectorRevisionFromPackage(pkg)
	publication := connectorRevisionResponse(revision)
	installation := connectorInstallationDetailsResponse(domain.ConnectorInstallation{ID: "installation-1", PackageSource: "example"}, revision, true)
	if len(publication.ExamplesZh) != 1 || publication.ExamplesZh[0] != "查询示例数据" || len(publication.ExamplesEn) != 1 || publication.ExamplesEn[0] != "Query example data" {
		t.Fatalf("publication examples = %#v / %#v", publication.ExamplesZh, publication.ExamplesEn)
	}
	if len(installation.ExamplesZh) != 1 || installation.ExamplesZh[0] != publication.ExamplesZh[0] || len(installation.ExamplesEn) != 1 || installation.ExamplesEn[0] != publication.ExamplesEn[0] || installation.Mode != "mcp" {
		t.Fatalf("installation details = %#v", installation)
	}
}

func TestCamScannerUsesInteractiveLoginAndOnlyShortLivedRuntimeCredential(t *testing.T) {
	policy := connectorRevisionPolicy{AuthMode: "oauth", Metadata: connectorpackage.Metadata{Source: "camscanner"}, CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "connector_package"}}
	if connectorAuthorizationMode(policy) != "interactive" {
		t.Fatal("browser login unavailable")
	}
	if err := validateProvidedConnectorCredentials(policy, nil); err == nil {
		t.Fatal("manual token bypass accepted")
	}
	fields := connectorAuthorizationCredentialFields(policy, connectorAuthorizationGrant{ExternalID: "owner", AccessToken: "short-token", RefreshToken: "renewal-token", IsDomestic: "1", ExpiresAt: time.Now().Add(time.Hour)})
	if len(fields) != 4 || fields["user_id"] != "owner" || fields["is_domestic"] != "1" || fields["access_token"] != "short-token" || fields["refresh_token"] != "" {
		t.Fatal("incorrect credential materialization")
	}
	policy.AuthMode = "cli"
	if connectorAuthorizationMode(policy) != "provided" {
		t.Fatal("driver chosen without reviewed policy")
	}
}

func TestPKULawProvidedTokenBoundary(t *testing.T) {
	policy := connectorRevisionPolicy{Metadata: connectorpackage.Metadata{Source: "pkulaw"}}
	for _, test := range []struct {
		name, credentials string
		valid             bool
	}{
		{"valid", `{"MCP_BEARER_TOKEN":"fixture-token"}`, true},
		{"empty", `{"MCP_BEARER_TOKEN":""}`, false},
		{"wrong field", `{"token":"fixture-token"}`, false},
		{"non string", `{"MCP_BEARER_TOKEN":123}`, false},
		{"extra secret", `{"MCP_BEARER_TOKEN":"fixture-token","other":"secret"}`, false},
		{"header injection", `{"MCP_BEARER_TOKEN":"value\r\nInjected: yes"}`, false},
		{"bearer prefix", `{"MCP_BEARER_TOKEN":"Bearer token"}`, false},
		{"oversized", `{"MCP_BEARER_TOKEN":"` + strings.Repeat("x", 4097) + `"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validatePKULawCredentials(policy, []byte(test.credentials))
			if (err == nil) != test.valid {
				t.Fatalf("validation = %v, valid = %v", err, test.valid)
			}
		})
	}
}

// A distributed bundle must not look verified until this installation has
// recorded the exact bundle/Runtime evidence, and publication must fail closed.
type starterConnectorRepository struct {
	workspaceapplication.Repository
	connectorPackageRepository
	revision        domain.ConnectorRevision
	verified        bool
	verifyErr       error
	published       bool
	bundle, runtime string
}

func (r *starterConnectorRepository) GetConnectorRevision(context.Context, string) (domain.ConnectorRevision, error) {
	return r.revision, nil
}
func (r *starterConnectorRepository) HasConnectorBundleRuntimeConformance(_ context.Context, bundle, runtime string) (bool, error) {
	r.bundle, r.runtime = bundle, runtime
	return r.verified, r.verifyErr
}
func (r *starterConnectorRepository) PublishConnectorRevision(_ context.Context, owner, revision string, version int64) (domain.ConnectorPublication, error) {
	r.published = true
	return domain.ConnectorPublication{PackageSource: r.revision.PackageSource, ActiveRevisionID: revision, State: domain.ConnectorPublicationAvailable, Version: version + 1}, nil
}

func TestStarterCLIRequiresLocalExactConformance(t *testing.T) {
	for _, tc := range []struct {
		name          string
		verified      bool
		verifyErr     error
		wantPublished bool
	}{
		{name: "manifest without local evidence"},
		{name: "exact local evidence", verified: true, wantPublished: true},
		{name: "evidence query failure", verifyErr: errors.New("fixture database failure")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, runtime := strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
			policy, err := json.Marshal(connectorRevisionPolicy{BundleSHA256: bundle, CLI: &connectorpackage.CLIManifest{Runtime: connectorpackage.ManagedRuntime{Digest: runtime}, Capabilities: []connectorpackage.CLICapability{{}}}})
			if err != nil {
				t.Fatal(err)
			}
			r := &starterConnectorRepository{revision: domain.ConnectorRevision{ID: "revision", PackageSource: "fixture", Mode: domain.ConnectorModeCLI, RuntimePolicy: policy}, verified: tc.verified, verifyErr: tc.verifyErr}
			response := connectorRevisionResponse(r.revision)
			if response.ConformanceAvailable {
				t.Fatal("a complete manifest was reported as real Conformance")
			}
			err = applyConnectorConformance(context.Background(), r, r.revision, response)
			if (err != nil) != (tc.verifyErr != nil) || response.ConformanceAvailable != tc.verified {
				t.Fatalf("evidence response=%v err=%v", response.ConformanceAvailable, err)
			}
			app, err := workspaceapplication.New(r)
			if err != nil {
				t.Fatal(err)
			}
			svc := &Service{accounts: &accountapplication.Service{}, workspace: app}
			ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator", Administrator: true})
			_, err = svc.PublishConnectorRevision(ctx, &workspacev1.PublishConnectorRevisionRequest{RevisionId: "revision", ExpectedVersion: 1})
			if r.published != tc.wantPublished || (err == nil) != tc.wantPublished {
				t.Fatalf("published=%v err=%v", r.published, err)
			}
			if r.bundle != bundle || r.runtime != runtime {
				t.Fatal("Conformance queried a different bundle or Runtime")
			}
		})
	}
}
