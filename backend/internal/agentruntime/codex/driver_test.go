package codex

import (
	"slices"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsNewAndResumeInvocations(t *testing.T) {
	driver := Driver{}
	request := agentruntime.ExecuteRequest{Model: "configured-model", Instruction: "fix tests"}
	created, err := driver.Build(request, t.TempDir())
	if err != nil {
		t.Fatalf("build new invocation: %v", err)
	}
	wantNew := []string{"exec", "--json", "--model", "configured-model", "--dangerously-bypass-approvals-and-sandbox", "--ignore-rules", "-"}
	if !slices.Equal(created.Args, wantNew) || created.Stdin == nil {
		t.Fatalf("new invocation = %+v", created)
	}

	request.CheckpointRef = "thread-1"
	resumed, err := driver.Build(request, t.TempDir())
	if err != nil {
		t.Fatalf("build resume invocation: %v", err)
	}
	wantResume := []string{"exec", "resume", "--json", "--model", "configured-model", "--dangerously-bypass-approvals-and-sandbox", "--ignore-rules", "thread-1", "-"}
	if !slices.Equal(resumed.Args, wantResume) {
		t.Fatalf("resume args = %v, want %v", resumed.Args, wantResume)
	}
}

func TestParserReadsThreadItemsAndUsage(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	fixtures := []string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"item.started","item":{"id":"item-1","type":"command_execution","command":"go test ./..."}}`,
		`{"type":"item.completed","item":{"id":"item-1","type":"command_execution","command":"go test ./...","exit_code":0}}`,
		`{"type":"item.completed","item":{"id":"item-2","type":"agent_message","text":"done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":14,"output_tokens":8}}`,
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
	wantKinds := []agentruntime.EventKind{agentruntime.EventCommandRequested, agentruntime.EventCommandCompleted}
	if !slices.Equal(kinds, wantKinds) {
		t.Fatalf("event kinds = %v, want %v", kinds, wantKinds)
	}
	result := parser.Result()
	if result.FinalMessage != "done" || result.CheckpointRef != "thread-1" || result.Usage.InputTokens != 14 || result.Usage.OutputTokens != 8 {
		t.Fatalf("parsed result = %+v", result)
	}
}
