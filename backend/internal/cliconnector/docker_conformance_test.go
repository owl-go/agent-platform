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
	tempRoot := t.TempDir()
	digest := "sha256:" + strings.Repeat("a", 64)
	image := "registry.example/codex@" + digest
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		uid, gid = 65532, 65532
	}
	var arguments []string
	suite, err := NewDockerConformance(DockerConformanceConfig{DockerCommand: "docker", Runtime: "runsc", TempRoot: tempRoot, RuntimeImages: map[string]string{digest: image}, UID: uid, GID: gid, Timeout: time.Minute}, func(_ context.Context, _ string, args ...string) error {
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
	mount := arguments[slices.Index(arguments, "--mount")+1]
	if !strings.Contains(mount, "src="+tempRoot+string(os.PathSeparator)) {
		t.Fatalf("conformance bundle is not under the host-visible temp root: %s", mount)
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

func TestDockerConformanceUsesDeclaredResourceLimits(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		uid, gid = 65532, 65532
	}
	var arguments []string
	suite, err := NewDockerConformance(DockerConformanceConfig{DockerCommand: "docker", Runtime: "runsc", TempRoot: t.TempDir(), RuntimeImages: map[string]string{digest: "registry.example/runtime@" + digest}, UID: uid, GID: gid, Timeout: time.Minute}, func(_ context.Context, _ string, args ...string) error { arguments = slices.Clone(args); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := suite.Test(context.Background(), testBundle(t, map[string]string{"node_modules/.bin/tool": "#!/bin/sh\n"}), digest, Definition{Executable: "tool", CPUMillis: 500, MemoryMiB: 256, ChildProcesses: 16}); err != nil {
		t.Fatal(err)
	}
	for flag, want := range map[string]string{"--cpus": "0.5", "--memory": "268435456", "--pids-limit": "16"} {
		index := slices.Index(arguments, flag)
		if index < 0 || arguments[index+1] != want {
			t.Errorf("%s = %v, want %s", flag, arguments, want)
		}
	}
}
