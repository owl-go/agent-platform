package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrRunNotFound      = errors.New("Run not found")
	ErrApprovalRunState = errors.New("Run state does not allow the Approval operation")
)

type ControlAction string

const (
	ControlInterrupt ControlAction = "interrupt"
	ControlResume    ControlAction = "resume"
	ControlCancel    ControlAction = "cancel"
	ControlKill      ControlAction = "kill"
)

type Lease struct {
	RunID              string
	SessionID          string
	AttemptID          string
	AttemptNumber      int
	Token              string
	RequestText        string
	ModelBinding       json.RawMessage
	CredentialBindings json.RawMessage
	ModelBudget        json.RawMessage
	ExecutionLimits    json.RawMessage
	RuntimeName        string
	RuntimeCLIVersion  string
	AdapterVersion     string
	ImageDigest        string
	Capabilities       json.RawMessage
	WorkspaceVolume    string
	RepositorySSHURL   string
	TargetBranch       string
	ReviewBranch       string
	GitAuthorName      string
	GitAuthorEmail     string
	QualityCommands    json.RawMessage
	ExpiresAt          time.Time
}

type ReconcileResult struct {
	Rescheduled int
	Failed      int
}

type SearchQuery struct {
	OrganizationID      string
	TeamID              string
	AgentID             string
	RepositoryBindingID string
	TaskID              string
	State               State
	Runtime             string
	CreatedFrom         *time.Time
	CreatedTo           *time.Time
	Limit               int
}

type Event struct {
	Sequence  int64
	Type      string
	Payload   json.RawMessage
	CreatedAt time.Time
}

type EventInput struct {
	Type    string
	Payload json.RawMessage
}

type ApprovalDecision struct {
	ApprovalID  string
	RunID       string
	Approved    bool
	ActorUserID string
	ActorType   string
	Reason      string
}

type ApprovalCommands interface {
	PauseForApproval(context.Context, string, int64, string, string, string, time.Time) error
	ApplyApprovalDecision(context.Context, ApprovalDecision, time.Time) error
}

type CompletionProjection struct {
	RunID     string
	SessionID string
	Completed bool
}

type QueuedRun struct {
	ID                 string
	SessionID          string
	CodingTaskID       string
	AgentReleaseID     string
	RuntimeImageID     string
	RequestText        string
	ModelBinding       json.RawMessage
	CredentialBindings json.RawMessage
	ModelBudget        json.RawMessage
	ExecutionLimits    json.RawMessage
	CreatedBy          string
	CreatedAt          time.Time
}

type LaunchCommands interface {
	CreateQueuedRun(context.Context, QueuedRun) error
}

// Details is the read model exposed by the Execution application service.
// Credential bindings and lease tokens are deliberately excluded.
type Details struct {
	ID              string
	SessionID       string
	AgentReleaseID  string
	RuntimeImageID  string
	RequestText     string
	State           State
	ModelBinding    json.RawMessage
	ModelBudget     json.RawMessage
	ExecutionLimits json.RawMessage
	Usage           json.RawMessage
	Cost            string
	TerminalError   json.RawMessage
	AttemptCount    int
	CreatedBy       string
	CreatedAt       time.Time
	StartedAt       *time.Time
	EndedAt         *time.Time
	UpdatedAt       time.Time
	Version         int64
	Attempts        []Attempt
}

type Attempt struct {
	ID                    string
	Number                int
	WorkerID              string
	State                 AttemptState
	InfrastructureFailure bool
	Error                 json.RawMessage
	StartedAt             time.Time
	EndedAt               *time.Time
}

type AttemptState string

const (
	AttemptProvisioning AttemptState = "provisioning"
	AttemptRunning      AttemptState = "running"
	AttemptCompleted    AttemptState = "completed"
	AttemptFailed       AttemptState = "failed"
	AttemptCancelled    AttemptState = "cancelled"
	AttemptLost         AttemptState = "lost"
)

func ParseAttemptState(value string) (AttemptState, error) {
	state := AttemptState(value)
	switch state {
	case AttemptProvisioning, AttemptRunning, AttemptCompleted, AttemptFailed, AttemptCancelled, AttemptLost:
		return state, nil
	default:
		return "", fmt.Errorf("unknown Attempt state %q", value)
	}
}

type Repository interface {
	Get(context.Context, string) (Details, error)
	Claim(context.Context, string, time.Duration, time.Time) (Lease, bool, error)
	Renew(context.Context, string, time.Duration, time.Time) error
	MarkRunning(context.Context, string, time.Time) error
	FinishOwned(context.Context, string, Outcome, time.Time) (CompletionProjection, error)
	AppendEvent(context.Context, string, EventInput, time.Time) error
	ReconcileExpired(context.Context, int, time.Time) (ReconcileResult, error)
	ListEventsAfter(context.Context, string, int64, int) ([]Event, error)
	Control(context.Context, string, int64, ControlAction, string, time.Time) (Details, error)
}

type SearchRepository interface {
	Search(context.Context, SearchQuery) ([]Details, error)
}
