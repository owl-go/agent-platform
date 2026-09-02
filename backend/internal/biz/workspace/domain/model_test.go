package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestExpertInputRequiresIntroductionInstructionAndValidTags(t *testing.T) {
	valid := ExpertInput{
		Name:                   "Architecture Expert",
		CapabilityIntroduction: "Designs maintainable systems.",
		ExecutionInstruction:   "Review the task and propose a concrete architecture.",
		ExpertiseTags:          []string{"Architecture", "Go"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid Expert input rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ExpertInput)
	}{
		{name: "missing capability introduction", mutate: func(input *ExpertInput) { input.CapabilityIntroduction = " " }},
		{name: "missing execution instruction", mutate: func(input *ExpertInput) { input.ExecutionInstruction = " " }},
		{name: "duplicate tags ignoring case", mutate: func(input *ExpertInput) { input.ExpertiseTags = []string{"Go", "go"} }},
		{name: "too many tags", mutate: func(input *ExpertInput) { input.ExpertiseTags = make([]string, 11) }},
		{name: "tag too long", mutate: func(input *ExpertInput) { input.ExpertiseTags = []string{strings.Repeat("x", 21)} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			if err := input.Validate(); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Validate() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestMigratedExpertWithoutInstructionIsIncomplete(t *testing.T) {
	expert := Expert{Name: "Legacy Expert", CapabilityIntroduction: "Old description"}
	if expert.Available() {
		t.Fatal("migrated Expert without an Execution Instruction is available")
	}
}

func TestExpertTeamInputRequiresDistinctOrderedMembers(t *testing.T) {
	valid := ExpertTeamInput{Name: "Delivery Team", CapabilityIntroduction: "Ships changes", ExpertiseTags: []string{"delivery"}, ExpertIDs: []string{"expert-a", "expert-b"}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid Expert Team rejected: %v", err)
	}
	for _, members := range [][]string{{"expert-a"}, {"expert-a", "expert-a"}, make([]string, 11)} {
		input := valid
		input.ExpertIDs = members
		if err := input.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("members %#v error = %v, want ErrInvalid", members, err)
		}
	}
}

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

func TestProviderModelValidationHasNoModelType(t *testing.T) {
	if err := ValidateProviderModel("gpt-5.6-sol", "GPT 5.6 Sol"); err != nil {
		t.Fatalf("valid Provider Model rejected: %v", err)
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
