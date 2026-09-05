package claude

import (
	"slices"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsHeadlessSandboxInvocation(t *testing.T) {
	driver := Driver{}
	invocation, err := driver.Build(agentruntime.ExecuteRequest{
		Model: "claude-sonnet-configured",
	}, t.TempDir())
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	want := []string{
		"--print", "--bare", "--verbose", "--output-format", "stream-json", "--include-partial-messages",
		"--permission-mode", "bypassPermissions", "--dangerously-skip-permissions",
		"--strict-mcp-config", "--disable-slash-commands", "--no-chrome",
		"--model", "claude-sonnet-configured",
	}
	if !slices.Equal(invocation.Args, want) {
		t.Fatalf("args = %v, want %v", invocation.Args, want)
	}
	if invocation.Stdin == nil {
		t.Fatal("instruction must be passed on stdin")
	}
}

func TestDriverBuildsZhipuClaudeCodeEnvironment(t *testing.T) {
	driver := Driver{}
	invocation, err := driver.Build(agentruntime.ExecuteRequest{
		Model:         "glm-5.3",
		ModelProvider: "zhipu",
	}, t.TempDir())
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	want := []string{
		"API_TIMEOUT_MS=3000000",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
	}
	if !slices.Equal(invocation.Env, want) {
		t.Fatalf("environment = %v, want %v", invocation.Env, want)
	}
}

func TestParserReadsStreamingTextResultUsageAndSession(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	fixtures := []string{
		`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"working"}}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"result","subtype":"success","result":"done","session_id":"session-1","usage":{"input_tokens":12,"output_tokens":7}}`,
	}
	var kinds []agentruntime.EventKind
	for _, fixture := range fixtures {
		events, err := parser.Parse(processharness.StreamStdout, []byte(fixture))
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		for _, event := range events {
			kinds = append(kinds, event.Kind)
		}
	}
	if !slices.Equal(kinds, []agentruntime.EventKind{agentruntime.EventMessageDelta, agentruntime.EventCommandRequested}) {
		t.Fatalf("event kinds = %v", kinds)
	}
	result := parser.Result()
	if result.FinalMessage != "done" || result.CheckpointRef != "session-1" || result.Usage.InputTokens != 12 || result.Usage.OutputTokens != 7 {
		t.Fatalf("parsed result = %+v", result)
	}
}

func TestParserDoesNotReportOmittedUsage(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	if _, err := parser.Parse(processharness.StreamStdout, []byte(`{"type":"result","result":"done"}`)); err != nil {
		t.Fatal(err)
	}
	if parser.Result().Usage.Reported {
		t.Fatal("omitted usage was reported as measured zero")
	}
}

func TestParserReportsClaudeCodeErrorResult(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	fixture := `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"API Error: upstream request failed","session_id":"session-1"}`
	if _, err := parser.Parse(processharness.StreamStdout, []byte(fixture)); err != nil {
		t.Fatal(err)
	}
	result := parser.Result()
	if result.Error == nil || !strings.Contains(result.Error.Error(), "API Error: upstream request failed") {
		t.Fatalf("error = %v, want Claude Code diagnostic", result.Error)
	}
	if result.FinalMessage != "" {
		t.Fatalf("final message = %q, want empty error result", result.FinalMessage)
	}
}

func TestParserReportsStderrWhenClaudeCodeReturnsNoResult(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	if _, err := parser.Parse(processharness.StreamStderr, []byte("connection reset by peer")); err != nil {
		t.Fatal(err)
	}
	result := parser.Result()
	if result.Error == nil || !strings.Contains(result.Error.Error(), "connection reset by peer") {
		t.Fatalf("error = %v, want stderr diagnostic", result.Error)
	}
}
