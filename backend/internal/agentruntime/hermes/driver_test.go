package hermes

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsSafeOneShotInvocation(t *testing.T) {
	scratch := t.TempDir()
	invocation, err := Driver{}.Build(agentruntime.ExecuteRequest{
		Instruction: "fix tests",
		Model:       "provider/configured-model",
	}, scratch)
	if err != nil {
		t.Fatalf("build invocation: %v", err)
	}
	want := []string{
		"--oneshot", "fix tests",
		"--model", "provider/configured-model",
		"--toolsets", "all",
		"--ignore-rules",
		"--usage-file", filepath.Join(scratch, "usage.json"),
	}
	if !slices.Equal(invocation.Args, want) {
		t.Fatalf("args = %v, want %v", invocation.Args, want)
	}
}

func TestDriverRejectsUnverifiedNativeResume(t *testing.T) {
	_, err := Driver{}.Build(agentruntime.ExecuteRequest{Model: "model", CheckpointRef: "session-1"}, t.TempDir())
	if err == nil {
		t.Fatal("expected native resume to remain disabled")
	}
}

func TestParserReadsMultilineFinalAndUsageFile(t *testing.T) {
	scratch := t.TempDir()
	usage := `{"completed":true,"failed":false,"input_tokens":10,"output_tokens":4,"estimated_cost_usd":0.0123,"session_id":"session-1"}`
	if err := os.WriteFile(filepath.Join(scratch, "usage.json"), []byte(usage), 0o600); err != nil {
		t.Fatalf("write usage fixture: %v", err)
	}
	parser := Driver{}.NewParser(scratch)
	for _, line := range []string{"first line", "second line"} {
		if _, err := parser.Parse(processharness.StreamStdout, []byte(line)); err != nil {
			t.Fatalf("parse output: %v", err)
		}
	}
	result := parser.Result()
	if result.FinalMessage != "first line\nsecond line" || result.CheckpointRef != "session-1" || result.Usage.InputTokens != 10 || result.Usage.OutputTokens != 4 || result.Usage.CostMicros != 12300 {
		t.Fatalf("parsed result = %+v", result)
	}
}

func TestParserRejectsHermesFailedUsageReport(t *testing.T) {
	scratch := t.TempDir()
	usage := `{"completed":false,"failed":true,"input_tokens":null,"output_tokens":null,"estimated_cost_usd":null,"session_id":null}`
	if err := os.WriteFile(filepath.Join(scratch, "usage.json"), []byte(usage), 0o600); err != nil {
		t.Fatal(err)
	}
	parser := Driver{}.NewParser(scratch)
	if _, err := parser.Parse(processharness.StreamStdout, []byte("HTTP 401: Missing Authentication header")); err != nil {
		t.Fatal(err)
	}
	result := parser.Result()
	if result.Error == nil || agentruntime.ErrorCodeOf(result.Error) != agentruntime.ErrorAuthenticationFailed || result.FinalMessage != "HTTP 401: Missing Authentication header" {
		t.Fatalf("parsed result = %+v", result)
	}
}
