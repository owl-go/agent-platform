package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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
}

func TestInteractiveConnectorAuthorizationRejectsOtherDrivers(t *testing.T) {
	for _, driver := range []string{"connector_package", "none", "dingtalk"} {
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
	if err := validateInteractiveConnectorDriver(connectorRevisionPolicy{MCP: &connectorpackage.MCPManifest{}}); err == nil {
		t.Fatal("MCP revision entered the CLI authorization flow")
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
