package runtimeexecutor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	secret, err := box.Encrypt([]byte(`{"MCP_BEARER_TOKEN":"secret-canary"}`), "mcp-server:platform-owner")
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
	files, variables, redactions, err := executor.nativeMCPFiles(context.Background(), application.ExecutionJob{
		OwnerID: "user-owner",
		Snapshot: domain.ExecutionSnapshot{
			RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{
				ModelID: "claude-fable-5", Endpoint: "https://models.example.test", ProviderType: "anthropic", Protocols: []string{"anthropic_messages"},
			},
			MCPServers: []domain.MCPServerSnapshot{
				{ID: "11111111-1111-1111-1111-111111111111", Name: "remote", Transport: "streamable_http", Configuration: httpConfig, SecretCiphertext: secret, SecretOwnerID: "platform-owner"},
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
	openClawConfig := string(files["extensions/openclaw.json"])
	for _, want := range []string{`"agent-workspace/claude-fable-5"`, `"https://models.example.test"`, `"anthropic-messages"`, `"${ANTHROPIC_API_KEY}"`, `"mcp"`} {
		if !strings.Contains(openClawConfig, want) {
			t.Fatalf("OpenClaw config = %s, missing %s", openClawConfig, want)
		}
	}
}

func TestNativeMCPFilesRevalidatesLifecycleBeforeReadingSnapshot(t *testing.T) {
	ctx := context.WithValue(context.Background(), mcpLifecycleContextKey{}, "request-context")
	lifecycleErr := errors.New("MCP Server is unavailable")
	lifecycle := &recordingMCPLifecycle{err: lifecycleErr}
	executor := &Executor{mcpLifecycle: lifecycle}

	_, _, _, err := executor.nativeMCPFiles(ctx, application.ExecutionJob{
		OwnerID: "user-owner",
		Snapshot: domain.ExecutionSnapshot{MCPServers: []domain.MCPServerSnapshot{{
			ID: "server-id", Name: "removed", Configuration: json.RawMessage(`not-json`),
		}}},
	})
	if !errors.Is(err, lifecycleErr) {
		t.Fatalf("nativeMCPFiles error = %v, want lifecycle error", err)
	}
	if lifecycle.ownerID != "user-owner" || lifecycle.serverID != "server-id" || lifecycle.contextValue != "request-context" {
		t.Fatalf("lifecycle call = %#v", lifecycle)
	}
}

func TestNativeMCPFilesDecryptsConnectorAuthorizationAAD(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := box.Encrypt([]byte(`{"MCP_BEARER_TOKEN":"package-secret"}`), "connector-authorization:user-owner")
	if err != nil {
		t.Fatal(err)
	}
	configuration, _ := json.Marshal(map[string]any{
		"url":         "https://mcp.example.test/mcp",
		"environment": []domain.EnvironmentVariable{{Name: "MCP_BEARER_TOKEN", Secret: true, Configured: true}},
	})
	executor := &Executor{box: box}
	_, variables, redactions, err := executor.nativeMCPFiles(context.Background(), application.ExecutionJob{
		ID:      "run-connector",
		OwnerID: "user-owner",
		Snapshot: domain.ExecutionSnapshot{
			ProviderModel: domain.ProviderModelSnapshot{ModelID: "model", Endpoint: "https://models.example.test", ProviderType: "anthropic", Protocols: []string{"anthropic_messages"}},
			MCPServers: []domain.MCPServerSnapshot{{
				ID: "installation-id", Name: "package", Transport: "streamable_http", Configuration: configuration,
				SecretCiphertext: secret, SecretOwnerID: "user-owner", SecretAAD: "connector-authorization:user-owner",
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if variables == nil || len(redactions) != 1 || string(redactions[0]) != "package-secret" {
		t.Fatalf("connector authorization projection = variables:%v redactions:%q", variables, redactions)
	}
}

type mcpLifecycleContextKey struct{}

type recordingMCPLifecycle struct {
	err          error
	ownerID      string
	serverID     string
	contextValue string
}

func (lifecycle *recordingMCPLifecycle) ValidateMCPInvocation(ctx context.Context, ownerID, serverID string) error {
	lifecycle.ownerID = ownerID
	lifecycle.serverID = serverID
	lifecycle.contextValue, _ = ctx.Value(mcpLifecycleContextKey{}).(string)
	return lifecycle.err
}

func TestLinearOAuthMaterializesBearerForEveryRuntimeWithoutRefreshToken(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	aad := "connector-authorization:owner:installation:"
	secret, err := box.Encrypt([]byte(`{"MCP_BEARER_TOKEN":"linear-access-canary","client_id":"registered-client","access_expires_at":"2026-10-01T12:00:00Z"}`), aad)
	if err != nil {
		t.Fatal(err)
	}
	configuration, _ := json.Marshal(map[string]any{"url": "https://mcp.linear.app/mcp", "egress_hosts": []string{"mcp.linear.app"}})
	executor := &Executor{box: box}
	files, variables, redactions, err := executor.nativeMCPFiles(context.Background(), application.ExecutionJob{OwnerID: "owner", Snapshot: domain.ExecutionSnapshot{
		ProviderModel: domain.ProviderModelSnapshot{ModelID: "model", Endpoint: "https://models.example.test", ProviderType: "anthropic", Protocols: []string{"anthropic_messages"}},
		MCPServers:    []domain.MCPServerSnapshot{{ID: "installation", Name: "linear", Transport: "streamable_http", Configuration: configuration, SecretCiphertext: secret, SecretOwnerID: "owner", SecretAAD: aad}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if variables[mcpTokenVariable("installation")] != "linear-access-canary" {
		t.Fatal("Codex bearer token was not materialized")
	}
	for _, name := range []string{"extensions/claude-mcp.json", "runtime-home/.hermes/config.yaml", "extensions/openclaw.json"} {
		if !bytes.Contains(files[name], []byte("Bearer linear-access-canary")) {
			t.Fatalf("%s lost bearer authentication", name)
		}
	}
	if len(redactions) != 3 {
		t.Fatalf("secret redaction set: %d", len(redactions))
	}
	for _, body := range files {
		if bytes.Contains(body, []byte("refresh_token")) {
			t.Fatal("refresh token entered Runtime files")
		}
	}
}
