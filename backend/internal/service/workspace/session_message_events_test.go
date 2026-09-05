package workspace

import (
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestSnapshotOfProjectsSafeProgress(t *testing.T) {
	snapshot := snapshotOf(domain.Message{
		State:         "generating",
		Content:       "partial",
		ProgressStage: "responding",
		ElapsedMS:     450,
		Activities:    []domain.ExecutionActivity{{Type: "command.requested", Detail: "git status --short"}},
	})

	if snapshot.State != "generating" || snapshot.Content != "partial" || snapshot.ProgressStage != "responding" || snapshot.ElapsedMS != 450 || len(snapshot.Activities) != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestTerminalMessageState(t *testing.T) {
	for _, test := range []struct {
		state string
		want  bool
	}{
		{state: "queued", want: false},
		{state: "generating", want: false},
		{state: "completed", want: true},
		{state: "failed", want: true},
		{state: "cancelled", want: true},
	} {
		if got := terminalMessageState(test.state); got != test.want {
			t.Fatalf("terminalMessageState(%q) = %v, want %v", test.state, got, test.want)
		}
	}
}
