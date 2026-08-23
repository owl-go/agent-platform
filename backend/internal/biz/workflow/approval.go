package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
)

// Approval coordinates Approval and Execution owners in one transaction.
type Approval interface {
	Request(context.Context, approvaldomain.Approval, int64, time.Time) error
	Decide(context.Context, approvaldomain.Approval, int64, time.Time) error
}

type ApprovalService struct{ transactions TransactionManager }

func NewApproval(transactions TransactionManager) *ApprovalService {
	return &ApprovalService{transactions: transactions}
}

var _ Approval = (*ApprovalService)(nil)

func (workflow *ApprovalService) Request(ctx context.Context, approval approvaldomain.Approval, expectedRunVersion int64, now time.Time) error {
	return workflow.within(ctx, func(participants Participants) error {
		if err := participants.Execution.PauseForApproval(ctx, approval.RunID, expectedRunVersion, approval.ID, string(approval.Kind), approval.RequestedBy, now); err != nil {
			return mapExecutionApprovalError(err)
		}
		pending, err := participants.Approval.PendingExists(ctx, approval.RunID)
		if err != nil {
			return err
		}
		if pending {
			return approvaldomain.ErrPendingExists
		}
		return participants.Approval.Create(ctx, approval)
	})
}

func (workflow *ApprovalService) Decide(ctx context.Context, approval approvaldomain.Approval, expectedVersion int64, now time.Time) error {
	return workflow.within(ctx, func(participants Participants) error {
		if err := participants.Approval.Decide(ctx, approval, expectedVersion); err != nil {
			return err
		}
		err := participants.Execution.ApplyApprovalDecision(ctx, executiondomain.ApprovalDecision{
			ApprovalID: approval.ID, RunID: approval.RunID,
			Approved:    approval.State == approvaldomain.StateApproved,
			ActorUserID: approval.DecidedBy, ActorType: string(approval.DecisionActorType), Reason: approval.DecisionReason,
		}, now)
		return mapExecutionApprovalError(err)
	})
}

func (workflow *ApprovalService) within(ctx context.Context, operation func(Participants) error) error {
	if workflow == nil || workflow.transactions == nil {
		return fmt.Errorf("Approval transaction manager is required")
	}
	return workflow.transactions.Within(ctx, operation)
}

func mapExecutionApprovalError(err error) error {
	if errors.Is(err, executiondomain.ErrApprovalRunState) {
		return approvaldomain.ErrRunState
	}
	return err
}
