package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

type State string

const (
	Queued              State = "queued"
	Provisioning        State = "provisioning"
	Running             State = "running"
	WaitingConfirmation State = "waiting_confirmation"
	Interrupting        State = "interrupting"
	Interrupted         State = "interrupted"
	Resuming            State = "resuming"
	Completed           State = "completed"
	Failed              State = "failed"
	Cancelled           State = "cancelled"
)

var (
	ErrLeaseLost               = errors.New("Run lease is missing, expired, or owned by another Worker")
	ErrInterruptionRequested   = errors.New("Run interruption was requested")
	ErrConcurrentModification  = errors.New("Run was modified concurrently")
	ErrControlRejected         = errors.New("Run control action is not valid in the current state")
	ErrApprovalRejected        = errors.New("Run Approval rejected the requested operation")
	ErrApprovalRequesterDenied = errors.New("Run Approval requester is no longer authorized")
	moneyPattern               = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	transitions                = map[State]map[State]struct{}{
		Queued:              stateSet(Provisioning, Cancelled),
		Provisioning:        stateSet(Running, Interrupting, Resuming, Failed, Cancelled),
		Running:             stateSet(WaitingConfirmation, Interrupting, Resuming, Completed, Failed, Cancelled),
		WaitingConfirmation: stateSet(Running, Interrupting, Cancelled, Failed),
		Interrupting:        stateSet(Interrupted, Resuming, Failed, Cancelled),
		Interrupted:         stateSet(Resuming, Cancelled),
		Resuming:            stateSet(Provisioning, Running, Failed, Cancelled),
		Completed:           {},
		Failed:              {},
		Cancelled:           {},
	}
)

// Run is the aggregate root for execution state, Attempts, leases, and emitted events.
type Run struct {
	ID           string
	SessionID    string
	State        State
	AttemptCount int
	Version      int64
	StartedAt    *time.Time
	EndedAt      *time.Time
}

type Outcome struct {
	State State
	Error json.RawMessage
	Usage json.RawMessage
	Cost  string
}

type ReconcileDecision struct {
	State     State
	EventType string
}

func RestoreRun(id, sessionID, stateValue string, attemptCount int, version int64, startedAt, endedAt *time.Time) (Run, error) {
	state, err := ParseState(stateValue)
	if err != nil {
		return Run{}, err
	}
	if id == "" || sessionID == "" || attemptCount < 0 || version <= 0 {
		return Run{}, fmt.Errorf("invalid persisted Run")
	}
	return Run{ID: id, SessionID: sessionID, State: state, AttemptCount: attemptCount, Version: version, StartedAt: startedAt, EndedAt: endedAt}, nil
}

func ParseState(value string) (State, error) {
	state := State(value)
	if _, ok := transitions[state]; !ok {
		return "", fmt.Errorf("unknown Run state %q", value)
	}
	return state, nil
}

func (run *Run) Claim(now time.Time) (int, error) {
	if run.State != Queued && run.State != Resuming {
		return 0, fmt.Errorf("Run in state %s cannot be claimed", run.State)
	}
	if err := run.transition(Provisioning); err != nil {
		return 0, err
	}
	run.AttemptCount++
	if run.StartedAt == nil {
		started := now.UTC()
		run.StartedAt = &started
	}
	return run.AttemptCount, nil
}

func (run *Run) MarkRunning() error {
	return run.transition(Running)
}

func (run *Run) RequestInterrupt() error {
	if run.State != Provisioning && run.State != Running {
		return fmt.Errorf("%w: Run in state %s cannot be interrupted", ErrControlRejected, run.State)
	}
	return run.transition(Interrupting)
}

func (run *Run) MarkInterrupted() error {
	if run.State != Interrupting {
		return fmt.Errorf("%w: Run in state %s cannot acknowledge interruption", ErrControlRejected, run.State)
	}
	return run.transition(Interrupted)
}

func (run *Run) Resume() error {
	if run.State != Interrupted {
		return fmt.Errorf("%w: Run in state %s cannot resume", ErrControlRejected, run.State)
	}
	run.EndedAt = nil
	return run.transition(Resuming)
}

func (run *Run) Cancel(now time.Time) error {
	if run.Terminal() || run.State == WaitingConfirmation {
		return fmt.Errorf("%w: terminal Run in state %s cannot be cancelled", ErrControlRejected, run.State)
	}
	if err := run.transition(Cancelled); err != nil {
		return fmt.Errorf("%w: %v", ErrControlRejected, err)
	}
	ended := now.UTC()
	run.EndedAt = &ended
	return nil
}

func (run *Run) Finish(outcome Outcome, now time.Time) error {
	if err := outcome.Validate(); err != nil {
		return err
	}
	if err := run.transition(outcome.State); err != nil {
		return err
	}
	ended := now.UTC()
	run.EndedAt = &ended
	return nil
}

func (run *Run) ReconcileExpired(maxAttempts int, now time.Time) (ReconcileDecision, error) {
	if maxAttempts <= 0 {
		return ReconcileDecision{}, fmt.Errorf("maximum Attempts must be positive")
	}
	if run.Terminal() {
		return ReconcileDecision{}, fmt.Errorf("terminal Run %s cannot have an active lease", run.ID)
	}
	decision := ReconcileDecision{State: Resuming, EventType: "run.retry_scheduled"}
	if run.AttemptCount >= maxAttempts {
		decision = ReconcileDecision{State: Failed, EventType: "run.failed"}
	}
	if err := run.transition(decision.State); err != nil {
		return ReconcileDecision{}, err
	}
	if decision.State == Failed {
		ended := now.UTC()
		run.EndedAt = &ended
	} else {
		run.EndedAt = nil
	}
	return decision, nil
}

func (run Run) Terminal() bool {
	return run.State == Completed || run.State == Failed || run.State == Cancelled
}

func (outcome *Outcome) Normalize() {
	if len(outcome.Usage) == 0 {
		outcome.Usage = json.RawMessage(`{}`)
	}
	if outcome.Cost == "" {
		outcome.Cost = "0"
	}
}

func (outcome Outcome) Validate() error {
	if outcome.State != Completed && outcome.State != Failed && outcome.State != Cancelled {
		return fmt.Errorf("Run outcome must be completed, failed, or cancelled")
	}
	if len(outcome.Usage) == 0 || !json.Valid(outcome.Usage) {
		return fmt.Errorf("Run usage must be valid JSON")
	}
	if len(outcome.Error) > 0 && !json.Valid(outcome.Error) {
		return fmt.Errorf("Run terminal error must be valid JSON")
	}
	if !moneyPattern.MatchString(outcome.Cost) {
		return fmt.Errorf("invalid Run model cost %q", outcome.Cost)
	}
	return nil
}

func (run *Run) transition(next State) error {
	allowed, ok := transitions[run.State]
	if !ok {
		return fmt.Errorf("unknown Run state %q", run.State)
	}
	if _, ok := transitions[next]; !ok {
		return fmt.Errorf("unknown Run state %q", next)
	}
	if _, ok := allowed[next]; !ok {
		return fmt.Errorf("invalid Run state transition %s -> %s", run.State, next)
	}
	run.State = next
	run.Version++
	return nil
}

func stateSet(states ...State) map[State]struct{} {
	values := make(map[State]struct{}, len(states))
	for _, state := range states {
		values[state] = struct{}{}
	}
	return values
}
