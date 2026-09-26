package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/backend/internal/connectorpackage"
)

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
