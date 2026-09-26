package connectorpackage

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestBuildOfficialFeishuArchiveCarriesExactEvidence(t *testing.T) {
	bundle := testExecutableBundle(t, "bin/lark-cli")
	digest := "sha256:" + strings.Repeat("a", 64)
	archive, err := BuildOfficialFeishuArchive(OfficialFeishuInput{Version: "1.0.93", Bundle: bundle, BundlePath: "bin/lark-cli", RuntimeVersion: "22.22.0", RuntimeDigest: digest, Capabilities: []CLICapability{{ID: "messages_search", ArgvPrefix: []string{"im", "message", "search"}, Risk: "low", Identities: []string{"user"}, Scopes: []string{"im:message:readonly"}, EgressHosts: []string{"open.feishu.cn"}, TimeoutSeconds: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Parse(archive)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Metadata.Source != "feishu" || pkg.Metadata.Version != "1.0.93" || pkg.CLI == nil || pkg.CLI.AuthenticationDriver != "feishu" || pkg.CLI.Runtime.Digest != digest || len(pkg.CLIBundleSHA256) != 64 {
		t.Fatalf("official package = %#v", pkg)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	foundSkill := false
	for _, entry := range reader.File {
		if entry.Name != "skills/feishu/SKILL.md" {
			continue
		}
		body, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		var contents bytes.Buffer
		if _, err := contents.ReadFrom(body); err != nil {
			t.Fatal(err)
		}
		if err := body.Close(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(contents.String(), "im +chat-search --query") || !strings.Contains(contents.String(), "im +messages-send --chat-id") {
			t.Fatalf("official Feishu Skill lacks the message flow: %q", contents.String())
		}
		foundSkill = true
	}
	if !foundSkill {
		t.Fatal("official Feishu Skill is missing")
	}
}

func TestBuildOfficialFeishuArchiveRejectsMissingEvidence(t *testing.T) {
	if _, err := BuildOfficialFeishuArchive(OfficialFeishuInput{Version: "1.0.93"}); err == nil {
		t.Fatal("missing build and Conformance evidence was accepted")
	}
}

func testExecutableBundle(t *testing.T, name string) []byte {
	t.Helper()
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	tarWriter := tar.NewWriter(gzipWriter)
	body := []byte("#!/usr/bin/env node\n")
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
