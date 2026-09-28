package workspace

import (
	"context"
	"fmt"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) GetHomeOverview(ctx context.Context, _ *workspacev1.GetHomeOverviewRequest) (*workspacev1.HomeOverview, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	repository, ok := service.workspace.Repository().(workspaceapplication.HomeOverviewRepository)
	if !ok {
		return nil, publicError(fmt.Errorf("%w: Home overview is unavailable", workspacedomain.ErrInvalid))
	}
	item, err := repository.GetHomeOverview(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.HomeOverview{}
	for _, task := range item.RecentTasks {
		response.RecentTasks = append(response.RecentTasks, &workspacev1.HomeTask{Kind: task.Kind, Id: task.ID, ParentId: task.ParentID, Title: task.Title, State: task.State, UpdatedAt: timestamppb.New(task.UpdatedAt)})
	}
	for _, workflow := range item.CommonWorkflows {
		response.CommonWorkflows = append(response.CommonWorkflows, &workspacev1.HomeWorkflow{Id: workflow.ID, Name: workflow.Name, RunCount: workflow.RunCount, UpdatedAt: timestamppb.New(workflow.UpdatedAt)})
	}
	for _, action := range item.ActionItems {
		response.ActionItems = append(response.ActionItems, &workspacev1.HomeAction{Kind: action.Kind, Id: action.ID, ExecutionKind: action.ExecutionKind, ExecutionId: action.ExecutionID, ParentId: action.ParentID, Title: action.Title, State: action.State, CreatedAt: timestamppb.New(action.CreatedAt)})
	}
	return response, nil
}
