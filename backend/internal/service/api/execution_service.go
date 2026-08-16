package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	executionv1 "agent-platform/backend/api/execution/v1"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func (service *GeneratedServices) ListRuns(ctx context.Context, request *executionv1.ListRunsRequest) (*executionv1.ListRunsResponse, error) {
	actor, err := service.dependencies.RunSearchAccess.AuthorizeTeamRead(ctx, "", request.TeamId)
	if err != nil {
		return nil, mapAuthorizationError(err, "run_search_denied")
	}
	query := executiondomain.SearchQuery{OrganizationID: actor.OrganizationID, TeamID: request.TeamId, Limit: 50}
	if request.AgentId != nil {
		query.AgentID = *request.AgentId
	}
	if request.RepositoryBindingId != nil {
		query.RepositoryBindingID = *request.RepositoryBindingId
	}
	if request.TaskId != nil {
		query.TaskID = *request.TaskId
	}
	for _, id := range []string{query.AgentID, query.RepositoryBindingID, query.TaskID} {
		if id != "" {
			if _, err := uuid.Parse(id); err != nil {
				return nil, publicError(400, "invalid_run_search")
			}
		}
	}
	if request.State != nil {
		state, err := executiondomain.ParseState(*request.State)
		if err != nil {
			return nil, publicError(400, "invalid_run_search")
		}
		query.State = state
	}
	if request.Runtime != nil {
		query.Runtime = *request.Runtime
		if query.Runtime != "claude" && query.Runtime != "codex" && query.Runtime != "hermes" && query.Runtime != "openclaw" {
			return nil, publicError(400, "invalid_run_search")
		}
	}
	if request.CreatedFrom != nil {
		value := request.CreatedFrom.AsTime().UTC()
		query.CreatedFrom = &value
	}
	if request.CreatedTo != nil {
		value := request.CreatedTo.AsTime().UTC()
		query.CreatedTo = &value
	}
	if request.Limit != nil {
		query.Limit = int(*request.Limit)
	}
	if query.Limit <= 0 || query.Limit > 100 || query.CreatedFrom != nil && query.CreatedTo != nil && query.CreatedFrom.After(*query.CreatedTo) {
		return nil, publicError(400, "invalid_run_search")
	}
	values, err := service.dependencies.RunSearch.Search(ctx, query)
	if err != nil {
		return nil, publicError(500, "run_search_failed")
	}
	items := make([]runResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newRunResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &executionv1.ListRunsResponse{})
}

func (service *GeneratedServices) GetRun(ctx context.Context, request *executionv1.GetRunRequest) (*executionv1.Run, error) {
	if _, err := uuid.Parse(request.RunId); err != nil {
		return nil, publicError(400, "invalid_run_id")
	}
	if err := service.dependencies.Access.AuthorizeRunRead(ctx, "", request.RunId); err != nil {
		return nil, mapAuthorizationError(err, "run_access_denied")
	}
	value, err := service.dependencies.Runs.Get(ctx, request.RunId)
	if errors.Is(err, executiondomain.ErrRunNotFound) {
		return nil, publicError(404, "run_not_found")
	}
	if err != nil {
		return nil, publicError(500, "run_query_failed")
	}
	return mappedResponse(ctx, http.StatusOK, newRunResponse(value), &executionv1.Run{})
}

func (service *GeneratedServices) InterruptRun(ctx context.Context, request *executionv1.InterruptRunRequest) (*executionv1.Run, error) {
	return service.controlRun(ctx, request, request.RunId, executiondomain.ControlInterrupt)
}
func (service *GeneratedServices) ResumeRun(ctx context.Context, request *executionv1.ResumeRunRequest) (*executionv1.Run, error) {
	return service.controlRun(ctx, request, request.RunId, executiondomain.ControlResume)
}
func (service *GeneratedServices) CancelRun(ctx context.Context, request *executionv1.CancelRunRequest) (*executionv1.Run, error) {
	return service.controlRun(ctx, request, request.RunId, executiondomain.ControlCancel)
}
func (service *GeneratedServices) KillRun(ctx context.Context, request *executionv1.KillRunRequest) (*executionv1.Run, error) {
	return service.controlRun(ctx, request, request.RunId, executiondomain.ControlKill)
}

func (service *GeneratedServices) controlRun(ctx context.Context, request proto.Message, runID string, action executiondomain.ControlAction) (*executionv1.Run, error) {
	if _, err := uuid.Parse(runID); err != nil {
		return nil, publicError(400, "invalid_resource_id")
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.RunControlAccess.AuthorizeRunControl(ctx, "", runID, string(action))
	if err != nil {
		return nil, mapAuthorizationError(err, "run_control_denied")
	}
	result, err := service.executeWrite(ctx, actor, "run."+string(action)+":"+runID, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Runs.Control(ctx, runID, version, action, actor.UserID)
		return encodeWriteResult(http.StatusOK, newRunResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &executionv1.Run{})
}
