package workflow

import (
	"context"
	"fmt"
	"time"

	executiondomain "agent-platform/backend/internal/biz/execution/domain"
)

type Completion interface {
	Finish(context.Context, string, executiondomain.Outcome, time.Time) error
}

type CompletionService struct{ transactions TransactionManager }

func NewCompletion(transactions TransactionManager) *CompletionService {
	return &CompletionService{transactions: transactions}
}

var _ Completion = (*CompletionService)(nil)

func (workflow *CompletionService) Finish(ctx context.Context, token string, outcome executiondomain.Outcome, now time.Time) error {
	if workflow == nil || workflow.transactions == nil {
		return fmt.Errorf("Run Completion transaction manager is required")
	}
	return workflow.transactions.Within(ctx, func(participants Participants) error {
		projection, err := participants.Execution.FinishOwned(ctx, token, outcome, now)
		if err != nil || !projection.Completed {
			return err
		}
		return participants.Collaboration.ProjectCompletedRun(ctx, projection.RunID, projection.SessionID, now)
	})
}
