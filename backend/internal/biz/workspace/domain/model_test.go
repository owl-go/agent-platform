package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateEnvironmentRejectsRuntimeControlVariables(t *testing.T) {
	for _, name := range []string{"PATH", "CODEX_HOME", "GIT_SSH_COMMAND", "LD_PRELOAD", "AGENT_PLATFORM_WORKFLOW_B64"} {
		if err := ValidateEnvironment([]EnvironmentVariable{{Name: name, Value: "unsafe"}}); err == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
	if err := ValidateEnvironment([]EnvironmentVariable{{Name: "CUSTOM_PROVIDER_TOKEN", Secret: true, Configured: true}}); err != nil {
		t.Fatalf("ordinary environment rejected: %v", err)
	}
}

func TestMCPHTTPAuthenticationIsNarrow(t *testing.T) {
	url := "https://mcp.example.test/mcp"
	valid := MCPServer{Name: "docs", Transport: "streamable_http", URL: &url, Environment: []EnvironmentVariable{{Name: "MCP_BEARER_TOKEN", Secret: true, Configured: true}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid HTTP MCP Server rejected: %v", err)
	}
	invalid := valid
	invalid.Environment[0].Name = "OTHER_SECRET"
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected non-standard HTTP authentication to be rejected")
	}
}

func TestModelProviderConnectionRequiresHTTPAndKnownProtocols(t *testing.T) {
	for _, endpoint := range []string{"https://api.openai.com/v1", "http://model-gateway.internal/v1"} {
		if err := ValidateModelProviderConnection("Primary", "openai", endpoint, []string{"openai_responses"}, "secret", true); err != nil {
			t.Fatalf("valid Provider Connection %q rejected: %v", endpoint, err)
		}
	}
	for _, test := range []struct {
		name      string
		endpoint  string
		protocols []string
	}{
		{name: "unsupported scheme", endpoint: "ftp://api.example.test/v1", protocols: []string{"openai_responses"}},
		{name: "userinfo", endpoint: "https://secret@api.example.test/v1", protocols: []string{"openai_responses"}},
		{name: "query", endpoint: "https://api.example.test/v1?token=secret", protocols: []string{"openai_responses"}},
		{name: "unknown protocol", endpoint: "https://api.example.test/v1", protocols: []string{"vendor_magic"}},
		{name: "duplicate protocol", endpoint: "https://api.example.test/v1", protocols: []string{"openai_chat", "openai_chat"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateModelProviderConnection("Connection", "custom_openai", test.endpoint, test.protocols, "secret", true); err == nil {
				t.Fatal("expected invalid Provider Connection to be rejected")
			}
		})
	}
}

func TestCompatibilityIsProtocolBased(t *testing.T) {
	responses := CompatibilityForProtocols([]string{"openai_responses"})
	if responses[0].RuntimeEngine != RuntimeClaude || responses[0].Status != "incompatible" {
		t.Fatalf("Claude compatibility = %#v", responses[0])
	}
	if responses[1].RuntimeEngine != RuntimeCodex || responses[1].Status != "unverified" {
		t.Fatalf("Codex compatibility = %#v", responses[1])
	}

	messages := CompatibilityForProtocols([]string{"anthropic_messages"})
	if messages[0].Status != "unverified" || messages[1].Status != "incompatible" {
		t.Fatalf("unexpected Anthropic compatibility: %#v", messages)
	}
}

func TestProviderCredentialDoesNotEnterExecutionSnapshotJSON(t *testing.T) {
	encoded, err := json.Marshal(ExecutionSnapshot{ProviderModel: ProviderModelSnapshot{
		ConnectionID: "connection-1", ConnectionVersion: 2, APIKeyCiphertext: []byte("encrypted-api-key"),
	}})
	if err != nil {
		t.Fatalf("encode Execution Snapshot: %v", err)
	}
	if strings.Contains(string(encoded), "encrypted-api-key") || strings.Contains(string(encoded), "api_key") {
		t.Fatalf("credential leaked into Execution Snapshot: %s", encoded)
	}
}
