package runtimeexecutor

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
)

func TestNativeMCPFilesProjectTestedSnapshotIntoAllRuntimes(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := box.Encrypt([]byte(`{"MCP_BEARER_TOKEN":"secret-canary"}`), "mcp-server:owner-1")
	if err != nil {
		t.Fatal(err)
	}
	httpConfig, _ := json.Marshal(map[string]any{
		"url":         "https://mcp.example.test/mcp",
		"environment": []domain.EnvironmentVariable{{Name: "MCP_BEARER_TOKEN", Secret: true, Configured: true}},
	})
	stdioConfig, _ := json.Marshal(map[string]any{
		"runner": "npx", "package": "@example/server", "package_version": "1.2.3",
		"arguments": []string{"--stdio"}, "environment": []domain.EnvironmentVariable{{Name: "REGION", Value: "test"}},
	})
	executor := &Executor{box: box}
	files, variables, redactions, err := executor.nativeMCPFiles(application.ExecutionJob{
		OwnerID: "owner-1",
		Snapshot: domain.ExecutionSnapshot{
			RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ModelID: "openai/test"},
			MCPServers: []domain.MCPServerSnapshot{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "remote", Transport: "streamable_http", Configuration: httpConfig, SecretCiphertext: secret},
				{ID: "22222222-2222-2222-2222-222222222222", Name: "local", Transport: "stdio", Configuration: stdioConfig},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"extensions/claude-mcp.json", "extensions/codex-config.toml", "runtime-home/.hermes/config.yaml", "extensions/openclaw.json"} {
		if len(files[name]) == 0 {
			t.Fatalf("missing %s", name)
		}
	}
	if len(variables) != 1 || len(redactions) != 1 || string(redactions[0]) != "secret-canary" {
		t.Fatalf("credential projection = variables:%v redactions:%q", variables, redactions)
	}
	if strings.Contains(string(files["extensions/codex-config.toml"]), "secret-canary") {
		t.Fatal("Codex config must reference the generated bearer-token environment variable")
	}
	if !strings.Contains(string(files["extensions/codex-config.toml"]), `@example/server@1.2.3`) {
		t.Fatalf("Codex stdio config = %s", files["extensions/codex-config.toml"])
	}
}
