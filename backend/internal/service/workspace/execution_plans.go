package workspace

import (
	"context"
	"fmt"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) executionPlanRepository() (workspaceapplication.ExecutionPlanRepository, error) {
	repository, ok := service.workspace.Repository().(workspaceapplication.ExecutionPlanRepository)
	if !ok {
		return nil, fmt.Errorf("Execution Plan repository is unavailable")
	}
	return repository, nil
}

func (service *Service) DecideSessionExecutionPlan(ctx context.Context, request *workspacev1.DecideSessionExecutionPlanRequest) (*workspacev1.SessionMessage, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.executionPlanRepository()
	if err != nil {
		return nil, publicError(err)
	}
	message, err := repository.DecideSessionExecutionPlan(ctx, owner, request.SessionId, request.MessageId, request.Decision, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return messageResponse(message), nil
}

func (service *Service) DecideRunExecutionPlan(ctx context.Context, request *workspacev1.DecideRunExecutionPlanRequest) (*workspacev1.Run, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, err := service.executionPlanRepository()
	if err != nil {
		return nil, publicError(err)
	}
	run, err := repository.DecideRunExecutionPlan(ctx, owner, request.WorkflowId, request.RunId, request.Decision, request.ExpectedVersion)
	if err != nil {
		return nil, publicError(err)
	}
	return runResponse(run), nil
}

func executionPlanResponse(item *workspacedomain.ExecutionPlan) *workspacev1.ExecutionPlan {
	if item == nil {
		return nil
	}
	response := &workspacev1.ExecutionPlan{Id: item.ID, State: item.State, Objective: item.Objective, SideEffects: item.SideEffects, Reasons: item.Reasons, EstimatedModelCalls: int32(item.EstimatedModelCalls), EstimatedCreditHundredths: item.EstimatedCreditHundredths, GenerationCreditHundredths: item.GenerationCreditHundredths, Generator: item.Generator, CreatedAt: timestamppb.New(item.CreatedAt), Version: item.Version}
	if item.DecidedAt != nil {
		response.DecidedAt = timestamppb.New(*item.DecidedAt)
	}
	for _, step := range item.Steps {
		response.Steps = append(response.Steps, &workspacev1.ExecutionPlanStep{Id: step.ID, Kind: step.Kind, Label: step.Label, Position: int32(step.Position), State: step.State})
	}
	for _, resource := range item.Resources {
		response.Resources = append(response.Resources, &workspacev1.ExecutionPlanResource{Kind: resource.Kind, Id: resource.ID, Name: resource.Name})
	}
	return response
}
