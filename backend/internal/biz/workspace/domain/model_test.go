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
		ProviderModelID:        "model-1",
		RuntimeEngine:          RuntimeCodex,
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
		{name: "missing Provider Model", mutate: func(input *ExpertInput) { input.ProviderModelID = " " }},
		{name: "invalid Runtime Engine", mutate: func(input *ExpertInput) { input.RuntimeEngine = "other" }},
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

func TestMigratedExpertWithoutExecutionConfigurationIsIncomplete(t *testing.T) {
	for _, expert := range []Expert{
		{Name: "No instruction", CapabilityIntroduction: "Old description", ProviderModelID: "model-1", RuntimeEngine: RuntimeCodex},
		{Name: "No model", CapabilityIntroduction: "Old description", ExecutionInstruction: "Help"},
		{Name: "No runtime", CapabilityIntroduction: "Old description", ExecutionInstruction: "Help", ProviderModelID: "model-1"},
	} {
		if expert.Available() {
			t.Fatalf("incomplete migrated Expert %q is available", expert.Name)
		}
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
	if responses[4].RuntimeEngine != RuntimePI || responses[4].Status != "unverified" {
		t.Fatalf("PI Agent compatibility = %#v", responses[4])
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

func TestExecutionSnapshotReturnsOrderedStages(t *testing.T) {
	snapshot := ExecutionSnapshot{SchemaVersion: 2, Stages: []ExecutionStageSnapshot{
		{Position: 1, RuntimeEngine: RuntimeClaude, ProviderModel: ProviderModelSnapshot{ID: "model-1"}},
		{Position: 2, RuntimeEngine: RuntimeCodex, ProviderModel: ProviderModelSnapshot{ID: "model-2"}},
	}}

	stages, err := snapshot.OrderedStages()
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 2 || stages[0].RuntimeEngine != RuntimeClaude || stages[1].ProviderModel.ID != "model-2" {
		t.Fatalf("ordered stages = %#v", stages)
	}
}

func TestExecutionSnapshotMapsLegacyTeamWithoutMutableExpertData(t *testing.T) {
	snapshot := ExecutionSnapshot{
		RuntimeEngine: RuntimeCodex,
		ProviderModel: ProviderModelSnapshot{ID: "legacy-model"},
		ExpertTeam: &ExpertTeamSnapshot{Members: []ExpertMemberSnapshot{
			{ExpertSnapshot: ExpertSnapshot{ID: "expert-1", Name: "Architect"}, Position: 1},
			{ExpertSnapshot: ExpertSnapshot{ID: "expert-2", Name: "Builder"}, Position: 2},
		}},
	}

	stages, err := snapshot.OrderedStages()
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 2 || stages[0].ProviderModel.ID != "legacy-model" || stages[1].RuntimeEngine != RuntimeCodex || stages[1].Expert == nil || stages[1].Expert.ID != "expert-2" {
		t.Fatalf("legacy stages = %#v", stages)
	}
}

func TestExecutionSnapshotRejectsAmbiguousStageSchemas(t *testing.T) {
	snapshot := ExecutionSnapshot{
		SchemaVersion: 2,
		Stages:        []ExecutionStageSnapshot{{Position: 1, RuntimeEngine: RuntimeCodex, ProviderModel: ProviderModelSnapshot{ID: "model-1"}}},
		RuntimeEngine: RuntimeClaude,
	}
	if _, err := snapshot.OrderedStages(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ambiguous snapshot error = %v", err)
	}
}

func TestModelProtocolForRuntimeSelectsTheProtocolActuallyUsed(t *testing.T) {
	tests := []struct {
		name      string
		runtime   RuntimeEngine
		protocols []string
		want      string
	}{
		{name: "Claude ignores earlier OpenAI protocol", runtime: RuntimeClaude, protocols: []string{"openai_responses", "anthropic_messages"}, want: "anthropic_messages"},
		{name: "Codex ignores earlier Anthropic protocol", runtime: RuntimeCodex, protocols: []string{"anthropic_messages", "openai_responses"}, want: "openai_responses"},
		{name: "PI follows its deterministic priority", runtime: RuntimePI, protocols: []string{"anthropic_messages", "openai_chat"}, want: "openai_chat"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ModelProtocolForRuntime(test.runtime, test.protocols)
			if err != nil || got != test.want {
				t.Fatalf("protocol = %q, err = %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestWorkflowRejectsLegacyExecutionOverrides(t *testing.T) {
	modelID := "model-1"
	runtime := RuntimeCodex
	input := WorkflowInput{Name: "Workflow", Goal: "Do the work", ProviderModelID: &modelID, RuntimeEngine: &runtime}
	if !errors.Is(input.Validate(), ErrInvalid) {
		t.Fatal("legacy Workflow execution overrides were accepted")
	}
}

func TestGitSourceAuthenticationAndConfigValidation(t *testing.T) {
	username := "developer"
	valid := []GitSource{
		{URL: "https://git.example.test/team/project.git", Branch: "main", Authentication: "none"},
		{URL: "https://git.example.test/team/project.git", Branch: "main", Authentication: "basic", Username: &username, CredentialConfigured: true},
		{URL: "git@git.example.test:team/project.git", Branch: "main", Authentication: "ssh", CredentialConfigured: true, Config: []GitConfigEntry{{Key: "user.name", Value: "Agent"}}},
	}
	for _, source := range valid {
		if err := ValidateGitSource(source); err != nil {
			t.Fatalf("valid Git source rejected: %v", err)
		}
	}
	for _, source := range []GitSource{
		{URL: "https://git.example.test/team/project.git", Branch: "main", Authentication: "basic"},
		{URL: "https://user:secret@git.example.test/team/project.git", Branch: "main", Authentication: "none"},
		{URL: "https://git.example.test/team/project.git", Branch: "main", Authentication: "none", Config: []GitConfigEntry{{Key: "credential.helper", Value: "store"}}},
		{URL: "https://git.example.test/team/project.git", Branch: "main", Authentication: "none", Config: []GitConfigEntry{{Key: "core.sshCommand", Value: "sh -c anything"}}},
	} {
		if err := ValidateGitSource(source); err == nil {
			t.Fatalf("unsafe Git source accepted: %#v", source)
		}
	}
}
