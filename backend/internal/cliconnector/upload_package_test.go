package cliconnector

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZIPPackageBuilderCreatesReviewedRuntimeBundle(t *testing.T) {
	archive := zipPackageFixture(t, map[string]string{
		"package.json": `{"name":"example-cli","version":"1.2.3","bin":{"example":"bin/cli.js"},"agentWorkspace":{"executable":"example","authenticationDriver":"none","supportedArchitectures":["linux-amd64"],"capabilities":[{"id":"read","argvPrefix":["read"],"risk":"low","identities":["user"],"egressHosts":["api.example.test"],"timeoutSeconds":60}]}}`,
		"bin/cli.js":   "#!/usr/bin/env node\n",
	})
	artifact, err := (ZIPPackageBuilder{}).Build(context.Background(), archive)
	if err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	definition, err := resolvePackageDefinition(Definition{ID: "connector-1", Name: "Example", Icon: "terminal", Description: "Reads examples", InstallationType: "upload", SourceObjectKey: "cli-connectors/sources/connector-1/v1/" + digest + ".zip", SourceSHA256: digest, State: StateBuilding}, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if definition.Package != "example-cli" || definition.Version != "1.2.3" || definition.Executable != "example" || len(definition.Capabilities) != 1 {
		t.Fatalf("definition = %#v", definition)
	}
	root := t.TempDir()
	if err := extractBundle(artifact.BundleBytes, root); err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(filepath.Join(root, "node_modules", ".bin", "example"))
	if err != nil || link != filepath.FromSlash("../example-cli/bin/cli.js") {
		t.Fatalf("link = %q, err = %v", link, err)
	}
}

func TestZIPPackageBuilderRejectsSymlinks(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	manifest, _ := writer.Create("package.json")
	_, _ = manifest.Write([]byte(`{"name":"example-cli","version":"1.2.3","bin":{"example":"bin/cli.js"}}`))
	header := &zip.FileHeader{Name: "bin/cli.js"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, _ := writer.CreateHeader(header)
	_, _ = entry.Write([]byte("../../escape"))
	_ = writer.Close()
	if _, err := (ZIPPackageBuilder{}).Build(context.Background(), buffer.Bytes()); err == nil {
		t.Fatal("expected uploaded symlink to be rejected")
	}
}

func zipPackageFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
