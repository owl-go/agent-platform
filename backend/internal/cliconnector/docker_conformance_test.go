package cliconnector

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDockerConformanceRunsPinnedRuntimeWithoutNetwork(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "registry.example/codex@" + digest
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		uid, gid = 65532, 65532
	}
	var arguments []string
	suite, err := NewDockerConformance(DockerConformanceConfig{DockerCommand: "docker", Runtime: "runsc", RuntimeImages: map[string]string{digest: image}, UID: uid, GID: gid, Timeout: time.Minute}, func(_ context.Context, _ string, args ...string) error {
		arguments = slices.Clone(args)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.Test(context.Background(), testBundle(t, map[string]string{"node_modules/.bin/tool": "#!/usr/bin/env node\n"}), digest, Definition{Executable: "tool"}); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"runsc", "--read-only", "ALL", "no-new-privileges", "none", "/opt/agent-connector/node_modules/.bin/tool", image, "--help"} {
		if !slices.Contains(arguments, required) {
			t.Fatalf("missing %q in %v", required, arguments)
		}
	}
}

func TestExtractBundleRejectsTraversal(t *testing.T) {
	err := extractBundle(testBundle(t, map[string]string{"../escape": "bad"}), t.TempDir())
	if err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestExtractBundleRejectsEscapingSymlink(t *testing.T) {
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: "node_modules/.bin/tool", Linkname: "../../../escape", Typeflag: tar.TypeSymlink}); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractBundle(output.Bytes(), t.TempDir()); err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}
}

func testBundle(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for name, contents := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
