package workflow

import (
	"context"
	"fmt"

	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
)

type Launch struct{ transactions TransactionManager }

func NewLaunch(transactions TransactionManager) *Launch { return &Launch{transactions: transactions} }

var _ collaborationdomain.LaunchCoordinator = (*Launch)(nil)

func (workflow *Launch) CreateLaunch(ctx context.Context, registration collaborationdomain.LaunchRegistration) (collaborationdomain.Launch, error) {
	var result collaborationdomain.Launch
	err := workflow.within(ctx, func(participants Participants) error {
		launch, plan, err := participants.Collaboration.CreateLaunchOwned(ctx, registration)
		if err != nil {
			return err
		}
		if err := participants.Execution.CreateQueuedRun(ctx, queuedRun(plan)); err != nil {
			return err
		}
		if err := participants.Collaboration.AppendLaunchMessage(ctx, plan); err != nil {
			return err
		}
		result = launch
		return nil
	})
	return result, err
}

func (workflow *Launch) Continue(ctx context.Context, registration collaborationdomain.ContinueRegistration) (collaborationdomain.Launch, error) {
	var result collaborationdomain.Launch
	err := workflow.within(ctx, func(participants Participants) error {
		launch, plan, err := participants.Collaboration.ContinueOwned(ctx, registration)
		if err != nil {
			return err
		}
		if err := participants.Execution.CreateQueuedRun(ctx, queuedRun(plan)); err != nil {
			return err
		}
		if err := participants.Collaboration.AppendLaunchMessage(ctx, plan); err != nil {
			return err
		}
		result = launch
		return nil
	})
	return result, err
}

func (workflow *Launch) within(ctx context.Context, operation func(Participants) error) error {
	if workflow == nil || workflow.transactions == nil {
		return fmt.Errorf("Coding Task Launch transaction manager is required")
	}
	return workflow.transactions.Within(ctx, operation)
}

func queuedRun(plan collaborationdomain.QueuedRunPlan) executiondomain.QueuedRun {
	return executiondomain.QueuedRun{
		ID: plan.ID, SessionID: plan.SessionID, CodingTaskID: plan.CodingTaskID,
		AgentReleaseID: plan.AgentReleaseID, RuntimeImageID: plan.RuntimeImageID,
		RequestText: plan.RequestText, ModelBinding: plan.ModelBinding,
		CredentialBindings: plan.CredentialBindings, ModelBudget: plan.ModelBudget,
		ExecutionLimits: plan.ExecutionLimits, CreatedBy: plan.CreatedBy, CreatedAt: plan.CreatedAt,
	}
}
