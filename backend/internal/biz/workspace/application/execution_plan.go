package application

import (
	"context"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type ExecutionPlanRepository interface {
	CreatePlannedMessagePair(context.Context, string, string, string, []domain.Attachment, string, string) (domain.Message, domain.Message, error)
	CreatePlannedRun(context.Context, string, string, string, *string, map[string]any, string) (domain.Run, error)
	ContinuePlannedRunConversation(context.Context, string, string, string, string, []domain.Attachment, string, string) (domain.Run, error)
	CompleteSessionPlanGeneration(context.Context, string, string, int64, []string, int64, bool) (domain.Message, error)
	CompleteRunPlanGeneration(context.Context, string, string, string, []string, int64, bool) (domain.Run, error)
	DecideSessionExecutionPlan(context.Context, string, string, int64, string, int64) (domain.Message, error)
	DecideRunExecutionPlan(context.Context, string, string, string, string, int64) (domain.Run, error)
}
