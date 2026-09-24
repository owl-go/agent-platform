package connectorpackage

import (
	"archive/tar"
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
