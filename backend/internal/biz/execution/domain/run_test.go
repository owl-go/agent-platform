package domain

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRunLifecycleOwnsTransitionsAndAttempts(t *testing.T) {
	run, err := RestoreRun("run-1", "session-1", string(Queued), 0, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	attempt, err := run.Claim(now)
	if err != nil || attempt != 1 || run.State != Provisioning || run.StartedAt == nil {
		t.Fatalf("Claim() = (%d, %v), Run = %+v", attempt, err, run)
	}
	if err := run.MarkRunning(); err != nil {
		t.Fatal(err)
	}
	outcome := Outcome{State: Completed, Usage: json.RawMessage(`{"input_tokens":12}`), Cost: "0.125"}
	if err := run.Finish(outcome, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if !run.Terminal() || run.EndedAt == nil || run.Version != 4 {
		t.Fatalf("unexpected completed Run: %+v", run)
	}
}

func TestRunInterruptResumeAndCancelLifecycle(t *testing.T) {
	now := time.Now().UTC()
	run, err := RestoreRun("run-1", "session-1", string(Running), 1, 2, &now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.RequestInterrupt(); err != nil || run.State != Interrupting {
		t.Fatalf("RequestInterrupt() state=%s error=%v", run.State, err)
	}
	if err := run.MarkInterrupted(); err != nil || run.State != Interrupted {
		t.Fatalf("MarkInterrupted() state=%s error=%v", run.State, err)
	}
	if err := run.Resume(); err != nil || run.State != Resuming {
		t.Fatalf("Resume() state=%s error=%v", run.State, err)
	}
	if err := run.Cancel(now.Add(time.Minute)); err != nil || run.State != Cancelled || run.EndedAt == nil {
		t.Fatalf("Cancel() Run=%+v error=%v", run, err)
	}
	if err := run.Resume(); !errors.Is(err, ErrControlRejected) {
		t.Fatalf("terminal Resume() error=%v, want ErrControlRejected", err)
	}
}

func TestRunRejectsInvalidTransitionsAndOutcomes(t *testing.T) {
	run, err := RestoreRun("run-1", "session-1", string(Queued), 0, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.MarkRunning(); err == nil {
		t.Fatal("queued Run transitioned directly to running")
	}
	for _, outcome := range []Outcome{
		{State: Running, Usage: json.RawMessage(`{}`), Cost: "0"},
		{State: Completed, Usage: json.RawMessage(`invalid`), Cost: "0"},
		{State: Completed, Usage: json.RawMessage(`{}`), Cost: "-1"},
		{State: Completed, Usage: json.RawMessage(`{}`), Cost: "NaN"},
	} {
		if err := outcome.Validate(); err == nil {
			t.Fatalf("invalid outcome accepted: %+v", outcome)
		}
	}
}

func TestRunWaitingForApprovalRejectsIndependentControls(t *testing.T) {
	now := time.Now().UTC()
	run, err := RestoreRun("run-1", "session-1", string(WaitingConfirmation), 1, 3, &now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.RequestInterrupt(); !errors.Is(err, ErrControlRejected) {
		t.Fatalf("RequestInterrupt() error=%v, want ErrControlRejected", err)
	}
	if err := run.Cancel(now.Add(time.Minute)); !errors.Is(err, ErrControlRejected) {
		t.Fatalf("Cancel() error=%v, want ErrControlRejected", err)
	}
}

func TestRunRecoveryOnlyAcceptsInfrastructureFailure(t *testing.T) {
	now := time.Now().UTC()
	run, err := RestoreRun("run-1", "session-1", string(RecoveryRequired), 2, 4, &now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := run.RecoverInfrastructure(false); !errors.Is(err, ErrControlRejected) {
		t.Fatalf("application failure recovery error=%v, want ErrControlRejected", err)
	}
	if err := run.RecoverInfrastructure(true); err != nil || run.State != Resuming || run.EndedAt != nil || run.Version != 5 {
		t.Fatalf("infrastructure recovery Run=%+v error=%v", run, err)
	}
}

func TestExpiredRunIsRescheduledThenRequiresManualRecoveryAtAttemptLimit(t *testing.T) {
	now := time.Now()
	first, _ := RestoreRun("run-1", "session-1", string(Running), 1, 2, &now, nil)
	decision, err := first.ReconcileExpired(2, now)
	if err != nil || decision.State != Resuming || decision.EventType != "run.retry_scheduled" {
		t.Fatalf("first decision = (%+v, %v)", decision, err)
	}
	second, _ := RestoreRun("run-2", "session-2", string(Running), 2, 3, &now, nil)
	decision, err = second.ReconcileExpired(2, now)
	if err != nil || decision.State != RecoveryRequired || second.Terminal() {
		t.Fatalf("second decision = (%+v, %v), Run = %+v", decision, err, second)
	}
}
