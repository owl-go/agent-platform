package productanalytics

import (
	"context"
	"time"
)

type SessionTerminalObservation struct {
	OwnerID       string
	SessionID     string
	MessageID     int64
	State         string
	Duration      time.Duration
	Specialist    string
	ArtifactCount int
	FailureStage  string
	SafeErrorCode string
	Recoverable   bool
}

type WorkflowCreatedObservation struct {
	OwnerID      string
	WorkflowID   string
	Source       string
	HasSchedule  bool
	HasConnector bool
}

type WorkflowTerminalObservation struct {
	OwnerID    string
	WorkflowID string
	RunID      string
	State      string
}

type Observer interface {
	LoginCompleted(context.Context, string, string)
	DefaultExecutionReady(context.Context, string, bool, string, string)
	FirstTaskStarted(context.Context, string, string, string, bool)
	SessionTerminal(context.Context, SessionTerminalObservation)
	WorkflowSaveStarted(context.Context, string, string, int)
	WorkflowCreated(context.Context, WorkflowCreatedObservation)
	WorkflowTerminal(context.Context, WorkflowTerminalObservation)
	ExecutionStreamReconnected(context.Context, string, string, string, string)
}

type Nop struct{}

func (Nop) LoginCompleted(context.Context, string, string)                      {}
func (Nop) DefaultExecutionReady(context.Context, string, bool, string, string) {}
func (Nop) FirstTaskStarted(context.Context, string, string, string, bool)      {}
func (Nop) SessionTerminal(context.Context, SessionTerminalObservation)         {}
func (Nop) WorkflowSaveStarted(context.Context, string, string, int)            {}
func (Nop) WorkflowCreated(context.Context, WorkflowCreatedObservation)         {}
func (Nop) WorkflowTerminal(context.Context, WorkflowTerminalObservation)       {}
func (Nop) ExecutionStreamReconnected(context.Context, string, string, string, string) {
}
