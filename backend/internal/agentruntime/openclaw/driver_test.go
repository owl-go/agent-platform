package openclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsNativeLocalAgentInvocation(t *testing.T) {
	scratch := t.TempDir()
	invocation, err := Driver{}.Build(agentruntime.ExecuteRequest{
		RunID:          "run-1",
		Instruction:    "fix tests",
		Model:          "openai/configured-model",
		ModelEndpoint:  "https://models.example.test/v1",
		ModelProtocols: []string{"openai_responses"},
	}, scratch)
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	promptPath := filepath.Join(scratch, "instruction.txt")
	want := []string{
		"agent", "--local", "--agent", "main", "--session-key", "run-1",
		"--message-file", promptPath, "--model", "agent-workspace/openai/configured-model",
		"--timeout", "0", "--json",
	}
	if !slices.Equal(invocation.Args, want) {
		t.Fatalf("args = %v, want %v", invocation.Args, want)
	}
	configPath := filepath.Join(scratch, "openclaw.json")
	if !slices.Equal(invocation.Env, []string{"OPENCLAW_CONFIG_PATH=" + configPath}) {
		t.Fatalf("env = %v", invocation.Env)
	}
	contents, err := os.ReadFile(promptPath)
	if err != nil || string(contents) != "fix tests" {
		t.Fatalf("prompt file = %q, err = %v", contents, err)
	}
	configContents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(configContents, &config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	serialized := string(configContents)
	for _, want := range []string{`"agent-workspace"`, `"https://models.example.test/v1"`, `"openai-responses"`, `"${OPENAI_API_KEY}"`, `"id":"openclaw"`} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("config = %s, missing %s", serialized, want)
		}
	}
	if strings.Contains(serialized, "configured-secret") {
		t.Fatalf("config must reference an environment variable: %s", serialized)
	}
}

func TestDriverQualifiesPlatformModelWithProvider(t *testing.T) {
	for _, test := range []struct {
		provider string
		model    string
		want     string
	}{
		{provider: "openai", model: "gpt-5.6-sol", want: "agent-workspace/gpt-5.6-sol"},
		{provider: "anthropic", model: "claude-fable-5", want: "agent-workspace/claude-fable-5"},
	} {
		t.Run(test.provider, func(t *testing.T) {
			invocation, err := Driver{}.Build(agentruntime.ExecuteRequest{
				RunID: "run-1", Instruction: "hello", Model: test.model, ModelProvider: test.provider,
			}, t.TempDir())
			if err != nil {
				t.Fatalf("build invocation: %v", err)
			}
			if !slices.Contains(invocation.Args, test.want) {
				t.Fatalf("args = %v, want qualified model %q", invocation.Args, test.want)
			}
		})
	}
}

func TestDriverRejectsNestedCLIBackendsAndUnverifiedResume(t *testing.T) {
	for _, request := range []agentruntime.ExecuteRequest{
		{RunID: "run-1", Model: "claude-cli/sonnet"},
		{RunID: "run-1", Model: "codex-cli/gpt", CheckpointRef: "session-1"},
		{RunID: "run-1", Model: "openai/model", CheckpointRef: "session-1"},
		{RunID: "run-1", Model: ""},
		{RunID: "run-1", Model: "model", ModelEndpoint: "https://user:secret@example.test/v1"},
		{RunID: "run-1", Model: "model", ModelEndpoint: "https://example.test/v1?token=secret"},
		{RunID: "run-1", Model: "model", ModelProtocols: []string{"unsupported"}},
	} {
		if _, err := (Driver{}).Build(request, t.TempDir()); err == nil {
			t.Fatalf("expected request to be rejected: %+v", request)
		}
	}
}

func TestEncodeRuntimeConfigMapsProtocolsInSnapshotOrder(t *testing.T) {
	for _, test := range []struct {
		protocol string
		wantAPI  string
		wantKey  string
	}{
		{protocol: "openai_responses", wantAPI: "openai-responses", wantKey: "${OPENAI_API_KEY}"},
		{protocol: "openai_chat", wantAPI: "openai-completions", wantKey: "${OPENAI_API_KEY}"},
		{protocol: "anthropic_messages", wantAPI: "anthropic-messages", wantKey: "${ANTHROPIC_API_KEY}"},
		{protocol: "gemini", wantAPI: "google-generative-ai", wantKey: "${OPENAI_API_KEY}"},
	} {
		t.Run(test.protocol, func(t *testing.T) {
			encoded, err := EncodeRuntimeConfig(agentruntime.ExecuteRequest{
				Model: "CaseSensitive/Model", ModelProtocols: []string{test.protocol, "openai_responses"},
			}, map[string]MCPServer{"docs": {Enabled: true, URL: "https://mcp.example.test/mcp", Transport: "streamable-http"}})
			if err != nil {
				t.Fatal(err)
			}
			value := string(encoded)
			for _, want := range []string{`"api":"` + test.wantAPI + `"`, `"apiKey":"` + test.wantKey + `"`, `"agent-workspace/CaseSensitive/Model"`, `"mcp"`} {
				if !strings.Contains(value, want) {
					t.Fatalf("config = %s, missing %s", value, want)
				}
			}
		})
	}
}

func TestParserReadsPrettyJSONFinalAndSession(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	lines := []string{
		`{`,
		`  "payloads": [{"text": "done"}],`,
		`  "meta": {"agentMeta": {"sessionId": "session-1"}}`,
		`}`,
	}
	for _, line := range lines {
		if _, err := parser.Parse(processharness.StreamStdout, []byte(line)); err != nil {
			t.Fatalf("parse output: %v", err)
		}
	}
	result := parser.Result()
	if result.Error != nil || result.FinalMessage != "done" || result.CheckpointRef != "session-1" {
		t.Fatalf("parsed result = %+v", result)
	}
}

func TestParserClassifiesAuthenticationFailureFromStderr(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	for _, line := range []string{
		"[provider-transport-fetch] response status=401",
		"FailoverError: Authentication failed (provider returned HTTP 401)",
	} {
		if _, err := parser.Parse(processharness.StreamStderr, []byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	result := parser.Result()
	if agentruntime.ErrorCodeOf(result.Error) != agentruntime.ErrorAuthenticationFailed {
		t.Fatalf("authentication result = %+v", result)
	}
}
