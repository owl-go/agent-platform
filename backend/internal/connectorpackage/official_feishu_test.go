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
	archive, err := BuildOfficialFeishuArchive(OfficialFeishuInput{Version: "1.0.93", Bundle: bundle, BundlePath: "bin/lark-cli", RuntimeVersion: "22.22.0", RuntimeDigest: digest, Capabilities: []CLICapability{{ID: "task_create", ArgvPrefix: []string{"task", "+create"}, Risk: "high", Identities: []string{"user"}, Scopes: []string{"task:task:write"}, EgressHosts: []string{"open.feishu.cn"}, TimeoutSeconds: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Parse(archive)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Metadata.Source != "feishu" || pkg.Metadata.Version != "1.0.93" || pkg.CLI == nil || pkg.CLI.AuthenticationDriver != "feishu" || pkg.CLI.Runtime.Digest != digest || len(pkg.CLIBundleSHA256) != 64 || len(pkg.CLI.Capabilities) != 1 || pkg.CLI.Capabilities[0].ID != "task_create" {
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
		if !strings.Contains(contents.String(), "im +chat-search --query") || !strings.Contains(contents.String(), "im +messages-send --chat-id") || !strings.Contains(contents.String(), "task +create --summary") || !strings.Contains(contents.String(), "contact +search-user --query") {
			t.Fatalf("official Feishu Skill lacks a reviewed flow: %q", contents.String())
		}
		foundSkill = true
	}
	if !foundSkill {
		t.Fatal("official Feishu Skill is missing")
	}
	for _, name := range []string{"skills/feishu/references/lark-task/SKILL.md", "skills/feishu/references/lark-calendar/SKILL.md", "skills/feishu/references/lark-base/SKILL.md", "skills/feishu/references/lark-shared/SKILL.md", "skills/feishu/references/LICENSE"} {
		found := false
		for _, entry := range reader.File {
			if entry.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("official Feishu reference %q is missing", name)
		}
	}
}

func TestBuildOfficialFeishuArchiveKeepsCLIPinWhenPackageVersionChanges(t *testing.T) {
	bundle := testExecutableBundle(t, "bin/lark-cli")
	archive, err := BuildOfficialFeishuArchive(OfficialFeishuInput{
		Version: "1.0.93", PackageVersion: "1.0.94", Bundle: bundle,
		BundlePath: "bin/lark-cli", RuntimeVersion: "22.22.0",
		RuntimeDigest: "sha256:" + strings.Repeat("a", 64),
		Capabilities:  []CLICapability{{ID: "task_create", ArgvPrefix: []string{"task", "+create"}, Risk: "high", Identities: []string{"user"}, Scopes: []string{"task:task:write"}, EgressHosts: []string{"open.feishu.cn"}, TimeoutSeconds: 60}},
	})
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Parse(archive)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Metadata.Version != "1.0.94" {
		t.Fatalf("package version = %q", pkg.Metadata.Version)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
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
		if !strings.Contains(contents.String(), "@larksuite/cli@1.0.93") {
			t.Fatal("package revision changed the pinned CLI Skill references")
		}
		return
	}
	t.Fatal("official Feishu Skill is missing")
}

func TestOfficialFeishuSkillReferencesCoverPinnedCLIDomains(t *testing.T) {
	resources, err := OfficialFeishuSkillResources("1.0.93")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 546 {
		t.Fatalf("Feishu Skill resource count = %d", len(resources))
	}
	domains := 0
	for name, body := range resources {
		if strings.HasPrefix(name, "references/lark-") && strings.HasSuffix(name, "/SKILL.md") && strings.Count(name, "/") == 2 {
			domains++
			if len(body) == 0 || !strings.Contains(string(resources["SKILL.md"]), "("+name+")") {
				t.Fatalf("Feishu Skill does not route to %q", name)
			}
		}
	}
	if domains != 28 {
		t.Fatalf("official CLI domain count = %d", domains)
	}
	if _, err := OfficialFeishuSkillResources("1.0.94"); err == nil {
		t.Fatal("unreviewed CLI version reused stale Skill references")
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
