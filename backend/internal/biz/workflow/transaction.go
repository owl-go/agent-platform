package workflow

import (
	"context"
	"time"

	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
)

type CollaborationCommands interface {
	CreateLaunchOwned(context.Context, collaborationdomain.LaunchRegistration) (collaborationdomain.Launch, collaborationdomain.QueuedRunPlan, error)
	ContinueOwned(context.Context, collaborationdomain.ContinueRegistration) (collaborationdomain.Launch, collaborationdomain.QueuedRunPlan, error)
	AppendLaunchMessage(context.Context, collaborationdomain.QueuedRunPlan) error
	ProjectCompletedRun(context.Context, string, string, time.Time) error
}

type ExecutionCommands interface {
	CreateQueuedRun(context.Context, executiondomain.QueuedRun) error
	FinishOwned(context.Context, string, executiondomain.Outcome, time.Time) (executiondomain.CompletionProjection, error)
	PauseForApproval(context.Context, string, int64, string, string, string, time.Time) error
	ApplyApprovalDecision(context.Context, executiondomain.ApprovalDecision, time.Time) error
}

type ApprovalCommands interface {
	PendingExists(context.Context, string) (bool, error)
	Create(context.Context, approvaldomain.Approval) error
	Decide(context.Context, approvaldomain.Approval, int64) error
}

type Participants struct {
	Collaboration CollaborationCommands
	Execution     ExecutionCommands
	Approval      ApprovalCommands
}

type TransactionManager interface {
	Within(context.Context, func(Participants) error) error
}
