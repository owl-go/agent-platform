package workflow

import (
	"context"
	"fmt"
	"time"

	executiondomain "agent-platform/backend/internal/biz/execution/domain"
)

type Completion interface {
	Finish(context.Context, string, executiondomain.Outcome, time.Time) error
	Control(context.Context, string, int64, executiondomain.ControlAction, string, time.Time) (executiondomain.Details, error)
	ReconcileExpired(context.Context, int, time.Time) (executiondomain.ReconcileResult, error)
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
		if err != nil || projection.State == "" {
			return err
		}
		return participants.Collaboration.ProjectFinishedRun(ctx, projection.RunID, projection.SessionID, projection.State, now)
	})
}

func (workflow *CompletionService) Control(ctx context.Context, runID string, expectedVersion int64, action executiondomain.ControlAction, actorUserID string, now time.Time) (executiondomain.Details, error) {
	var details executiondomain.Details
	err := workflow.transactions.Within(ctx, func(participants Participants) error {
		var projection executiondomain.CompletionProjection
		var err error
		details, projection, err = participants.Execution.Control(ctx, runID, expectedVersion, action, actorUserID, now)
		if err != nil || projection.State == "" {
			return err
		}
		return participants.Collaboration.ProjectFinishedRun(ctx, projection.RunID, projection.SessionID, projection.State, now)
	})
	return details, err
}

func (workflow *CompletionService) ReconcileExpired(ctx context.Context, maxAttempts int, now time.Time) (executiondomain.ReconcileResult, error) {
	var result executiondomain.ReconcileResult
	err := workflow.transactions.Within(ctx, func(participants Participants) error {
		var projections []executiondomain.CompletionProjection
		var err error
		result, projections, err = participants.Execution.ReconcileExpired(ctx, maxAttempts, now)
		if err != nil {
			return err
		}
		for _, projection := range projections {
			if err := participants.Collaboration.ProjectFinishedRun(ctx, projection.RunID, projection.SessionID, projection.State, now); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
