package pi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsIsolatedJSONInvocation(t *testing.T) {
	scratch := t.TempDir()
	invocation, err := (Driver{}).Build(agentruntime.ExecuteRequest{
		Instruction:    "fix tests",
		Model:          "configured-model",
		ModelEndpoint:  "http://models.example.test/v1",
		ModelProtocols: []string{"openai_chat"},
	}, scratch)
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	want := []string{
		"--mode", "json", "--print", "--provider", providerName, "--model", "configured-model",
		"--session-dir", filepath.Join(scratch, "sessions"), "--no-extensions", "--no-skills",
		"--no-prompt-templates", "--no-context-files", "--", "fix tests",
	}
	if !slices.Equal(invocation.Args, want) || !slices.Equal(invocation.Env, []string{"PI_CODING_AGENT_DIR=" + scratch}) || invocation.Stdin != nil {
		t.Fatalf("invocation = %+v", invocation)
	}
	contents, err := os.ReadFile(filepath.Join(scratch, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Providers map[string]providerConfiguration `json:"providers"`
	}
	if err := json.Unmarshal(contents, &config); err != nil {
		t.Fatal(err)
	}
	provider := config.Providers[providerName]
	if provider.BaseURL != "http://models.example.test/v1" || provider.API != "openai-completions" || provider.APIKey != "$OPENAI_API_KEY" || len(provider.Models) != 1 || provider.Models[0].ID != "configured-model" {
		t.Fatalf("provider config = %+v", provider)
	}
}

func TestDriverSelectsSupportedProtocols(t *testing.T) {
	tests := map[string]string{
		"openai_responses":   "openai-responses",
		"openai_chat":        "openai-completions",
		"anthropic_messages": "anthropic-messages",
		"gemini":             "google-generative-ai",
	}
	for platform, want := range tests {
		t.Run(platform, func(t *testing.T) {
			got, err := modelProtocol([]string{platform})
			if err != nil || got != want {
				t.Fatalf("protocol = %q, err = %v", got, err)
			}
		})
	}
}

func TestDriverRejectsUnsafeConfiguration(t *testing.T) {
	for _, request := range []agentruntime.ExecuteRequest{
		{Model: "model", ModelEndpoint: "https://user:secret@example.test/v1"},
		{Model: "model", ModelEndpoint: "https://example.test/v1?token=secret"},
		{Model: "model", ModelProtocols: []string{"unsupported"}},
		{Model: "model", CheckpointRef: "session-1"},
		{Model: "model", MCPConfigPath: "/tmp/mcp.json"},
	} {
		if _, err := (Driver{}).Build(request, t.TempDir()); err == nil {
			t.Fatalf("expected request to be rejected: %+v", request)
		}
	}
}

func TestParserStreamsTextToolsFinalAndUsage(t *testing.T) {
	parser := (Driver{}).NewParser(t.TempDir())
	fixtures := []string{
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"do"}}`,
		`{"type":"tool_execution_start","toolCallId":"tool-1","toolName":"bash","args":{"command":"go test ./..."}}`,
		`{"type":"tool_execution_end","toolCallId":"tool-1","toolName":"bash","result":{"content":"ok"},"isError":false}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"done"}],"usage":{"input":12,"output":7,"cost":{"total":0.0123}},"stopReason":"stop"}}`,
	}
	var kinds []agentruntime.EventKind
	for _, fixture := range fixtures {
		events, err := parser.Parse(processharness.StreamStdout, []byte(fixture))
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			kinds = append(kinds, event.Kind)
		}
	}
	if !slices.Equal(kinds, []agentruntime.EventKind{agentruntime.EventMessageDelta, agentruntime.EventCommandRequested, agentruntime.EventCommandCompleted}) {
		t.Fatalf("event kinds = %v", kinds)
	}
	result := parser.Result()
	if result.FinalMessage != "done" || result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 7 || result.Usage.CostMicros != 12300 {
		t.Fatalf("result = %+v", result)
	}
}

func TestParserDoesNotReportOmittedUsage(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	if _, err := parser.Parse(processharness.StreamStdout, []byte(`{"type":"message_end","message":{"role":"assistant","content":[]}}`)); err != nil {
		t.Fatal(err)
	}
	if parser.Result().Usage.Reported {
		t.Fatal("omitted usage was reported as measured zero")
	}
}

func TestParserClassifiesAuthenticationFailure(t *testing.T) {
	parser := (Driver{}).NewParser(t.TempDir())
	_, err := parser.Parse(processharness.StreamStdout, []byte(`{"type":"message_end","message":{"role":"assistant","stopReason":"error","errorMessage":"HTTP 401 invalid API key"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if agentruntime.ErrorCodeOf(parser.Result().Error) != agentruntime.ErrorAuthenticationFailed {
		t.Fatalf("result = %+v", parser.Result())
	}
}

func TestParserClassifiesAuthenticationFailureFromStderr(t *testing.T) {
	parser := (Driver{}).NewParser(t.TempDir())
	if _, err := parser.Parse(processharness.StreamStderr, []byte("request failed: 401 unauthorized")); err != nil {
		t.Fatal(err)
	}
	if agentruntime.ErrorCodeOf(parser.Result().Error) != agentruntime.ErrorAuthenticationFailed {
		t.Fatalf("result = %+v", parser.Result())
	}
}
