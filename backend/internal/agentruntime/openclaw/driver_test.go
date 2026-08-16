package openclaw

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsNativeLocalAgentInvocation(t *testing.T) {
	scratch := t.TempDir()
	invocation, err := Driver{}.Build(agentruntime.ExecuteRequest{
		RunID:       "run-1",
		Instruction: "fix tests",
		Model:       "openai/configured-model",
	}, scratch)
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	promptPath := filepath.Join(scratch, "instruction.txt")
	want := []string{
		"agent", "--local", "--agent", "main", "--session-key", "run-1",
		"--message-file", promptPath, "--model", "openai/configured-model",
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
	wantConfig := `{"plugins":{"allow":["openai"],"bundledDiscovery":"allowlist","slots":{"memory":"none"},"entries":{"openai":{"enabled":true}}}}` + "\n"
	if string(configContents) != wantConfig {
		t.Fatalf("config = %s, want %s", configContents, wantConfig)
	}
}

func TestDriverRejectsNestedCLIBackendsAndUnverifiedResume(t *testing.T) {
	for _, request := range []agentruntime.ExecuteRequest{
		{RunID: "run-1", Model: "claude-cli/sonnet"},
		{RunID: "run-1", Model: "codex-cli/gpt", CheckpointRef: "session-1"},
		{RunID: "run-1", Model: "openai/model", CheckpointRef: "session-1"},
		{RunID: "run-1", Model: "missing-provider-prefix"},
		{RunID: "run-1", Model: "../model"},
	} {
		if _, err := (Driver{}).Build(request, t.TempDir()); err == nil {
			t.Fatalf("expected request to be rejected: %+v", request)
		}
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
