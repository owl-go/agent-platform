package agentruntime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
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
	ModelEndpoint  string
	ModelProvider  string
	ModelProtocols []string
	CheckpointRef  string
	EnvironmentRef string
	MCPConfigPath  string
	Attachments    []Attachment
}

type Attachment struct {
	Path        string
	ContentType string
}

func (r ExecuteRequest) SupportsModelProtocol(protocol string) bool {
	for _, candidate := range r.ModelProtocols {
		if candidate == protocol {
			return true
		}
	}
	return false
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
	if len(r.Attachments) > 10 {
		return &Error{Code: ErrorInvalidConfiguration, Message: "at most 10 attachments are allowed"}
	}
	for _, attachment := range r.Attachments {
		clean := filepath.Clean(attachment.Path)
		const attachmentRoot = "/workspace/.agent-platform-attachments"
		if !filepath.IsAbs(clean) || clean == attachmentRoot || !strings.HasPrefix(clean, attachmentRoot+string(filepath.Separator)) || strings.TrimSpace(attachment.ContentType) == "" {
			return &Error{Code: ErrorInvalidConfiguration, Message: "attachment path and content type are invalid"}
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
	// ModelInvocationStarted reports only that the local Runtime process started.
	// It is not evidence of Provider usage and must not drive Credit settlement.
	ModelInvocationStarted bool
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	CostMicros   int64
	// Reported distinguishes an actual zero-token report from missing Usage.
	Reported bool
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
	EventReasoningSummary  EventKind = "reasoning.summary"
	EventCommandRequested  EventKind = "command.requested"
	EventCommandCompleted  EventKind = "command.completed"
	EventFileChanged       EventKind = "file.changed"
	EventApprovalRequested EventKind = "approval.requested"
	EventUsageUpdated      EventKind = "usage.updated"
	EventCheckpointSaved   EventKind = "checkpoint.saved"
	EventWorkflowDelivered EventKind = "workflow.delivered"
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
