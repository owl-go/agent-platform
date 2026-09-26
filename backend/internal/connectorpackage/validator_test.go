package connectorpackage_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"agent-platform/backend/internal/connectorpackage"
)

func TestParseValidMCPPackage(t *testing.T) {
	archive := packageZIP(t, map[string]string{
		"connector-meta.json":   `{"source":"example-service","version":"1.2.3","type":"mcp","name":"Example Service","description":"Query Example data","examples_zh":["查询数据"],"examples_en":["Query data"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`,
		"icon.svg":              `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"mcp.json":              `{"transport":"streamable_http","url":"https://mcp.example.com/tools","timeout_seconds":30,"egress_hosts":["mcp.example.com"]}`,
		"skills/query/SKILL.md": "---\nname: example-query\ndisplay_name: Example Query\ndescription: Query Example data\nversion: 1.0.0\nauthor: Example\n---\n\n# Example Query\nUse the connector tool to query data.\n",
	})

	pkg, err := connectorpackage.Parse(archive)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if pkg.Metadata.Source != "example-service" || pkg.Metadata.Version != "1.2.3" || pkg.Metadata.Type != connectorpackage.TypeMCP {
		t.Fatalf("unexpected metadata: %#v", pkg.Metadata)
	}
	if len(pkg.Skills) != 1 || pkg.Skills[0].Name != "example-query" {
		t.Fatalf("unexpected skills: %#v", pkg.Skills)
	}
	if len(pkg.SHA256) != 64 || pkg.NormalizedArchive == nil {
		t.Fatalf("package was not normalized: digest=%q size=%d", pkg.SHA256, len(pkg.NormalizedArchive))
	}
}

func TestParseValidCLIPackage(t *testing.T) {
	archive := packageZIP(t, map[string]string{
		"connector-meta.json":     `{"source":"example-cli","version":"2.0.0","type":"cli","name":"Example CLI","description":"Operate Example","examples_zh":["执行操作"],"examples_en":["Run operation"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`,
		"icon.svg":                `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"cli.json":                `{"runtime":{"kind":"node","version":"22.14.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"example","commands":{"init":{"argv":["setup","--non-interactive"]},"auth":{"argv":["auth","--json"]},"status":{"argv":["status","--json"]},"unAuth":{"argv":["logout","--json"]}},"status_match":{"json_path":"$.authenticated","equals":true},"auth_url_domains":["accounts.example.com"],"egress_hosts":["api.example.com"],"timeout_seconds":60}`,
		"skills/operate/SKILL.md": "---\nname: example-operate\ndisplay_name: Example Operate\ndescription: Operate Example\nversion: 2.0.0\nauthor: Example\n---\n\n# Operate\nFollow the documented command workflow.\n",
	})

	pkg, err := connectorpackage.Parse(archive)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if pkg.CLI == nil || pkg.CLI.Executable != "example" || pkg.MCP != nil {
		t.Fatalf("unexpected CLI package: %#v", pkg)
	}
	if pkg.CLI.AuthenticationDriver != "connector_package" {
		t.Fatalf("default authentication driver = %q", pkg.CLI.AuthenticationDriver)
	}
}

func TestParseOfficialFeishuAuthenticationDriver(t *testing.T) {
	archive := packageZIP(t, map[string]string{
		"connector-meta.json":      `{"source":"feishu","version":"1.0.93","type":"cli","name":"Feishu CLI","description":"Operate Feishu with reviewed commands","examples_zh":["查询飞书消息"],"examples_en":["Search Feishu messages"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`,
		"icon.svg":                 `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"cli.json":                 `{"runtime":{"kind":"node","version":"22.22.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"lark-cli","authentication_driver":"feishu","commands":{"init":{"argv":["app","status","--output","json"]},"auth":{"argv":["auth","login","--output","json"]},"status":{"argv":["auth","status","--output","json"]},"unAuth":{"argv":["auth","logout","--output","json"]}},"status_match":{"json_path":"$.authenticated","equals":true},"auth_url_domains":["open.feishu.cn"],"egress_hosts":["open.feishu.cn"],"timeout_seconds":60}`,
		"skills/messages/SKILL.md": "---\nname: feishu-messages\ndisplay_name: Feishu Messages\ndescription: Search and send reviewed Feishu messages\nversion: 1.0.0\nauthor: Agent Workspace\n---\n\nUse only the reviewed message capabilities.\n",
	})
	pkg, err := connectorpackage.Parse(archive)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.CLI == nil || pkg.CLI.AuthenticationDriver != "feishu" {
		t.Fatalf("official Feishu driver was not preserved: %#v", pkg.CLI)
	}
}

func TestParseValidCLIExecutableBundle(t *testing.T) {
	bundle := executableBundle(t, "bin/example", []byte("#!/bin/sh\necho ok\n"))
	archive := packageZIPBytes(t, map[string][]byte{
		"connector-meta.json":     []byte(`{"source":"bundled-cli","version":"2.0.0","type":"cli","name":"Bundled CLI","description":"Operate Example","examples_zh":["执行"],"examples_en":["Run"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`),
		"icon.svg":                []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"cli.json":                []byte(`{"runtime":{"kind":"node","version":"22.14.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"example","bundle_path":"bin/example","commands":{"init":{"argv":["setup"]},"auth":{"argv":["auth"]},"status":{"argv":["status"]},"unAuth":{"argv":["logout"]}},"status_match":{"json_path":"$.authenticated","equals":true},"egress_hosts":["api.example.com"],"timeout_seconds":60,"capabilities":[{"id":"identity","argv_prefix":["status"],"risk":"low","identities":["user"],"egress_hosts":["api.example.com"],"timeout_seconds":30}]}`),
		"cli-bundle.tgz":          bundle,
		"skills/operate/SKILL.md": []byte("---\nname: bundled-operate\ndisplay_name: Bundled Operate\ndescription: Operate Example\nversion: 2.0.0\nauthor: Example\n---\n\n# Operate\n"),
	})
	pkg, err := connectorpackage.Parse(archive)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	wantDigest := sha256.Sum256(bundle)
	if pkg.CLIBundleSHA256 != hex.EncodeToString(wantDigest[:]) || len(pkg.CLIBundle) != len(bundle) || pkg.CLI.BundlePath != "bin/example" {
		t.Fatalf("bundle projection = digest %q bytes %d path %q", pkg.CLIBundleSHA256, len(pkg.CLIBundle), pkg.CLI.BundlePath)
	}
}

func TestParseCLIBundleAllowsOnlyContainedSymlinks(t *testing.T) {
	for _, test := range []struct {
		name      string
		link      string
		wantError bool
	}{
		{name: "contained npm bin link", link: "../../bin/example"},
		{name: "escaping npm bin link", link: "../../../outside", wantError: true},
		{name: "absolute npm bin link", link: "/outside", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var bundle bytes.Buffer
			compressed := gzip.NewWriter(&bundle)
			archive := tar.NewWriter(compressed)
			if err := archive.WriteHeader(&tar.Header{Name: "./", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
				t.Fatal(err)
			}
			body := []byte("#!/bin/sh\nexit 0\n")
			if err := archive.WriteHeader(&tar.Header{Name: "bin/example", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
				t.Fatal(err)
			}
			if _, err := archive.Write(body); err != nil {
				t.Fatal(err)
			}
			if err := archive.WriteHeader(&tar.Header{Name: "node_modules/.bin/example", Linkname: test.link, Typeflag: tar.TypeSymlink}); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := compressed.Close(); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				"connector-meta.json": []byte(`{"source":"example-cli","version":"1.0.0","type":"cli","name":"Example","description":"Example CLI","examples_zh":["执行"],"examples_en":["Run"],"minPlatformVersion":"1.0.0","auth_mode":"none"}`),
				"icon.svg":            []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
				"cli.json":            []byte(`{"runtime":{"kind":"node","version":"22.22.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"example","bundle_path":"node_modules/.bin/example","commands":{"init":{"argv":["setup"]},"auth":{"argv":["auth"]},"status":{"argv":["status"]},"unAuth":{"argv":["logout"]}},"status_match":{"json_path":"$.ok","equals":true},"egress_hosts":["api.example.com"],"timeout_seconds":60,"capabilities":[{"id":"run","argv_prefix":["run"],"risk":"low","identities":["user"],"egress_hosts":["api.example.com"],"timeout_seconds":30}]}`),
				"cli-bundle.tgz":      bundle.Bytes(),
				"skills/run/SKILL.md": []byte("---\nname: example-run\ndisplay_name: Run\ndescription: Run command\nversion: 1.0.0\nauthor: Example\n---\n\n# Run\nUse the reviewed command.\n"),
			}
			_, err := connectorpackage.Parse(packageZIPBytes(t, files))
			if test.wantError && (err == nil || !strings.Contains(err.Error(), "unsafe symlink")) {
				t.Fatalf("Parse() error = %v, want unsafe symlink", err)
			}
			if !test.wantError && err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
		})
	}
}

func TestParseRejectsInvalidPackageBoundaries(t *testing.T) {
	validMeta := `{"source":"example-service","version":"1.0.0","type":"mcp","name":"Example","description":"Example connector","examples_zh":["查询"],"examples_en":["Query"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`
	validSkill := "---\nname: example\ndisplay_name: Example\ndescription: Example skill\nversion: 1.0.0\nauthor: Example\n---\n\n# Example\nInstructions.\n"
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{name: "missing skill", files: map[string]string{"connector-meta.json": validMeta, "icon.svg": "<svg></svg>", "mcp.json": `{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30}`}, want: "at least one skills/*/SKILL.md"},
		{name: "mixed modes", files: map[string]string{"connector-meta.json": validMeta, "icon.svg": "<svg></svg>", "mcp.json": `{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30}`, "cli.json": `{}`, "skills/a/SKILL.md": validSkill}, want: "exactly one"},
		{name: "hard coded secret", files: map[string]string{"connector-meta.json": validMeta, "icon.svg": "<svg></svg>", "mcp.json": `{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30,"headers":{"Authorization":"Bearer secret-value"}}`, "skills/a/SKILL.md": validSkill}, want: "literal secret"},
		{name: "shell command", files: map[string]string{"connector-meta.json": strings.Replace(validMeta, `"type":"mcp"`, `"type":"cli"`, 1), "icon.svg": "<svg></svg>", "cli.json": `{"runtime":{"kind":"node","version":"22.14.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"example","commands":{"init":{"command":"example setup"}},"egress_hosts":["example.com"],"timeout_seconds":30}`, "skills/a/SKILL.md": validSkill}, want: "unknown field"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := connectorpackage.Parse(packageZIP(t, test.files))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestParseRejectsUnsafeZIPEntries(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("../connector-meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := connectorpackage.Parse(buffer.Bytes()); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("Parse() error = %v, want unsafe path", err)
	}
	var nested bytes.Buffer
	nestedWriter := zip.NewWriter(&nested)
	entry, err = nestedWriter.Create("connector/../connector-meta.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := nestedWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := connectorpackage.Parse(nested.Bytes()); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("nested traversal error = %v, want unsafe path", err)
	}
}

func TestParseRejectsTrailingJSONAndCredentialBearingURL(t *testing.T) {
	meta := `{"source":"example-service","version":"1.0.0","type":"mcp","name":"Example","description":"Example connector","examples_zh":["查询"],"examples_en":["Query"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`
	skill := "---\nname: example\ndisplay_name: Example\ndescription: Example skill\nversion: 1.0.0\nauthor: Example\n---\n\n# Example\nInstructions.\n"
	files := map[string]string{"connector-meta.json": meta, "icon.svg": "<svg></svg>", "mcp.json": `{"transport":"streamable_http","url":"https://example.com/tools?access_token=secret-value","egress_hosts":["example.com"],"timeout_seconds":30}`, "skills/a/SKILL.md": skill}
	if _, err := connectorpackage.Parse(packageZIP(t, files)); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("credential-bearing URL error = %v", err)
	}
	files["mcp.json"] = `{"transport":"streamable_http","url":"https://example.com/tools","egress_hosts":["example.com"],"timeout_seconds":30} {}`
	if _, err := connectorpackage.Parse(packageZIP(t, files)); err == nil || !strings.Contains(err.Error(), "multiple JSON") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestParseRejectsSkillResourceTraversalAndMissingFiles(t *testing.T) {
	meta := `{"source":"example-service","version":"1.0.0","type":"mcp","name":"Example","description":"Example connector","examples_zh":["查询"],"examples_en":["Query"],"minPlatformVersion":"1.0.0","auth_mode":"oauth"}`
	base := map[string]string{"connector-meta.json": meta, "icon.svg": "<svg></svg>", "mcp.json": `{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30}`}
	for _, test := range []struct{ name, link, want string }{
		{name: "traversal", link: "../../secret.txt", want: "outside its directory"},
		{name: "missing", link: "references/missing.md", want: "missing resource"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := make(map[string]string, len(base)+1)
			for name, value := range base {
				files[name] = value
			}
			files["skills/a/SKILL.md"] = "---\nname: example\ndisplay_name: Example\ndescription: Example skill\nversion: 1.0.0\nauthor: Example\n---\n\nSee [reference](" + test.link + ").\n"
			if _, err := connectorpackage.Parse(packageZIP(t, files)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseRejectsUnsupportedTransportAndResourceLimits(t *testing.T) {
	meta := `{"source":"example-service","version":"1.0.0","type":"mcp","name":"Example","description":"Example connector","examples_zh":["查询"],"examples_en":["Query"],"minPlatformVersion":"1.0.0","auth_mode":"none"}`
	skill := "---\nname: example\ndisplay_name: Example\ndescription: Example skill\nversion: 1.0.0\nauthor: Example\n---\n\n# Example\nInstructions.\n"
	for _, manifest := range []string{
		`{"transport":"sse","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30}`,
		`{"transport":"streamable_http","url":"https://example.com","egress_hosts":["example.com"],"timeout_seconds":30,"resource_limits":{"cpu_millis":-1}}`,
	} {
		files := map[string]string{"connector-meta.json": meta, "icon.svg": "<svg></svg>", "mcp.json": manifest, "skills/a/SKILL.md": skill}
		if _, err := connectorpackage.Parse(packageZIP(t, files)); err == nil {
			t.Fatalf("manifest %s was accepted", manifest)
		}
	}
}

func packageZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	contents := make(map[string][]byte, len(files))
	for name, body := range files {
		contents[name] = []byte(body)
	}
	return packageZIPBytes(t, contents)
}

func packageZIPBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, body := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func executableBundle(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
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
	return buffer.Bytes()
}
