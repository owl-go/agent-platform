package agentruntime

import (
	"context"
	"fmt"
	"time"
)

type Capability string

const (
	CapabilityNativeResume    Capability = "native_resume"
	CapabilityStreaming       Capability = "streaming"
	CapabilityStructuredFinal Capability = "structured_final"
	CapabilitySubagents       Capability = "subagents"
	CapabilityUsage           Capability = "usage"
)

type Descriptor struct {
	Name         string
	Version      string
	Capabilities map[Capability]bool
}

func (d Descriptor) Supports(capability Capability) bool {
	return d.Capabilities[capability]
}

type ExecuteRequest struct {
	RunID          string
	WorkspacePath  string
	Instruction    string
	Model          string
	CheckpointRef  string
	EnvironmentRef string
}

func (r ExecuteRequest) Validate() error {
	required := []struct {
		name  string
		value string
	}{
		{name: "run ID", value: r.RunID},
		{name: "workspace path", value: r.WorkspacePath},
		{name: "model", value: r.Model},
		{name: "environment ref", value: r.EnvironmentRef},
	}
	for _, field := range required {
		if field.value == "" {
			return &Error{
				Code:    ErrorInvalidConfiguration,
				Message: fmt.Sprintf("%s is required", field.name),
			}
		}
	}
	return nil
}

type Result struct {
	FinalMessage  string
	ExitCode      int
	DiffArtifact  string
	CheckpointRef string
	Usage         Usage
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	CostMicros   int64
}

type Event struct {
	RunID      string
	Sequence   int64
	Kind       EventKind
	OccurredAt time.Time
	Payload    []byte
}

type EventKind string

const (
	EventRuntimeStarted    EventKind = "runtime.started"
	EventMessageDelta      EventKind = "message.delta"
	EventMessageCompleted  EventKind = "message.completed"
	EventCommandRequested  EventKind = "command.requested"
	EventCommandCompleted  EventKind = "command.completed"
	EventFileChanged       EventKind = "file.changed"
	EventApprovalRequested EventKind = "approval.requested"
	EventUsageUpdated      EventKind = "usage.updated"
	EventCheckpointSaved   EventKind = "checkpoint.saved"
	EventRuntimeCompleted  EventKind = "runtime.completed"
	EventRuntimeFailed     EventKind = "runtime.failed"
)

type EventSink interface {
	Publish(ctx context.Context, event Event) error
}

// Adapter hides each CLI's process, event, checkpoint, and output conventions.
type Adapter interface {
	Describe(ctx context.Context) (Descriptor, error)
	Execute(ctx context.Context, request ExecuteRequest, events EventSink) (Result, error)
}
