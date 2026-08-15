package claude

import (
	"slices"
	"testing"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/agentruntime/processharness"
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
		"--print", "--bare", "--output-format", "stream-json", "--include-partial-messages",
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
