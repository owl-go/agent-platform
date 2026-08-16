package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	collaborationv1 "agent-platform/backend/api/collaboration/v1"
	typesv1 "agent-platform/backend/api/types/v1"
	collaborationapplication "agent-platform/backend/internal/biz/collaboration/application"
	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func collaborationQueryError(err error) error {
	if errors.Is(err, collaborationdomain.ErrTaskNotFound) || errors.Is(err, collaborationdomain.ErrSessionNotFound) || errors.Is(err, collaborationdomain.ErrMemoryCandidateNotFound) || errors.Is(err, collaborationdomain.ErrAgentMemoryNotFound) {
		return publicError(404, "collaboration_resource_not_found")
	}
	return publicError(500, "collaboration_query_failed")
}
func validCollaborationID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return publicError(400, "invalid_collaboration_id")
	}
	return nil
}

func (service *GeneratedServices) ListCodingTasks(ctx context.Context, request *collaborationv1.ListCodingTasksRequest) (*collaborationv1.ListCodingTasksResponse, error) {
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.Collaboration.ListTasks(ctx, actor.OrganizationID, actor.TeamID)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	items := make([]taskResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newTaskResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &collaborationv1.ListCodingTasksResponse{})
}

func (service *GeneratedServices) CreateCodingTask(ctx context.Context, request *collaborationv1.CreateCodingTaskRequest) (*collaborationv1.CreateCodingTaskResponse, error) {
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	var issue *collaborationdomain.IssueSnapshot
	if request.IssueSnapshot != nil {
		issue = &collaborationdomain.IssueSnapshot{Title: request.IssueSnapshot.Title, Body: request.IssueSnapshot.Body}
		if request.IssueSnapshot.Url != nil {
			issue.URL = *request.IssueSnapshot.Url
		}
	}
	result, err := service.executeWrite(ctx, actor, "coding-task.create", "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Collaboration.CreateTask(ctx, collaborationapplication.CreateTaskCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, AgentReleaseID: request.AgentReleaseId, CreatedBy: actor.UserID, Title: request.Title, RequestText: request.RequestText, IssueSnapshot: issue})
		return encodeWriteResult(http.StatusCreated, map[string]any{"task": newTaskResponse(value.Task), "session": newSessionResponse(value.Session), "run_id": value.RunID}, err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &collaborationv1.CreateCodingTaskResponse{})
}

func (service *GeneratedServices) GetCodingTask(ctx context.Context, request *collaborationv1.GetCodingTaskRequest) (*collaborationv1.CodingTask, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.Collaboration.GetTask(ctx, actor.OrganizationID, actor.TeamID, request.TaskId)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	return mappedResponse(ctx, http.StatusOK, newTaskResponse(value), &collaborationv1.CodingTask{})
}

func (service *GeneratedServices) UpdateCodingTask(ctx context.Context, request *collaborationv1.UpdateCodingTaskRequest) (*collaborationv1.CodingTask, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return collaborationWrite(service, ctx, actor, "coding-task.state:"+request.TaskId, strconv.FormatInt(version, 10), request, http.StatusOK, &collaborationv1.CodingTask{}, func(services transaction.TransactionServices) (any, error) {
		return services.Collaboration.ChangeTaskState(ctx, actor.OrganizationID, actor.TeamID, request.TaskId, version, collaborationdomain.TaskState(request.State))
	}, func(value any) any { return newTaskResponse(value.(collaborationdomain.Task)) })
}

func (service *GeneratedServices) ContinueCodingTask(ctx context.Context, request *collaborationv1.ContinueCodingTaskRequest) (*collaborationv1.ContinueCodingTaskResponse, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	result, err := service.executeWrite(ctx, actor, "coding-task.continue:"+request.TaskId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Collaboration.ContinueTask(ctx, collaborationapplication.ContinueTaskCommand{OrganizationID: actor.OrganizationID, TeamID: actor.TeamID, TaskID: request.TaskId, CreatedBy: actor.UserID, RequestText: request.RequestText, ExpectedTaskVersion: version, ExpectedSessionVersion: request.ExpectedSessionVersion})
		return encodeWriteResult(http.StatusCreated, map[string]any{"task": newTaskResponse(value.Task), "session": newSessionResponse(value.Session), "run_id": value.RunID}, err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &collaborationv1.ContinueCodingTaskResponse{})
}

func (service *GeneratedServices) GetCodingTaskSession(ctx context.Context, request *collaborationv1.GetCodingTaskSessionRequest) (*collaborationv1.Session, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	value, err := service.dependencies.Collaboration.GetSession(ctx, actor.OrganizationID, actor.TeamID, request.TaskId)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	return mappedResponse(ctx, http.StatusOK, newSessionResponse(value), &collaborationv1.Session{})
}

func (service *GeneratedServices) UpdateCodingTaskSession(ctx context.Context, request *collaborationv1.UpdateCodingTaskSessionRequest) (*collaborationv1.Session, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	memory := sessionMemory(request.Memory)
	return collaborationWrite(service, ctx, actor, "session-memory.update:"+request.TaskId, strconv.FormatInt(version, 10), request, http.StatusOK, &collaborationv1.Session{}, func(services transaction.TransactionServices) (any, error) {
		return services.Collaboration.UpdateSessionMemory(ctx, actor.OrganizationID, actor.TeamID, request.TaskId, memory, version)
	}, func(value any) any { return newSessionResponse(value.(collaborationdomain.Session)) })
}

func (service *GeneratedServices) ListSessionMessages(ctx context.Context, request *collaborationv1.ListSessionMessagesRequest) (*collaborationv1.ListSessionMessagesResponse, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	after, limit := int64(0), 100
	if request.After != nil {
		after = *request.After
	}
	if request.Limit != nil {
		limit = int(*request.Limit)
	}
	if after < 0 || limit <= 0 || limit > 200 {
		return nil, publicError(400, "invalid_message_cursor")
	}
	values, err := service.dependencies.Collaboration.ListMessages(ctx, actor.OrganizationID, actor.TeamID, request.TaskId, after, limit)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	items := make([]messageResponse, 0, len(values))
	for _, value := range values {
		items = append(items, messageResponse{ID: value.ID, RunID: value.RunID, Author: value.Author, AuthorUserID: value.AuthorUserID, Content: value.Content, CreatedAt: value.CreatedAt})
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &collaborationv1.ListSessionMessagesResponse{})
}

func (service *GeneratedServices) ListMemoryCandidates(ctx context.Context, request *collaborationv1.ListMemoryCandidatesRequest) (*collaborationv1.ListMemoryCandidatesResponse, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	values, err := service.dependencies.Collaboration.ListMemoryCandidates(ctx, actor.OrganizationID, actor.TeamID, request.TaskId)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	items := make([]memoryCandidateResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newMemoryCandidateResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &collaborationv1.ListMemoryCandidatesResponse{})
}

func (service *GeneratedServices) ProposeMemoryCandidate(ctx context.Context, request *collaborationv1.ProposeMemoryCandidateRequest) (*collaborationv1.MemoryCandidate, error) {
	if err := validCollaborationID(request.TaskId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return collaborationWrite(service, ctx, actor, "memory-candidate.create:"+request.TaskId, "", request, http.StatusCreated, &collaborationv1.MemoryCandidate{}, func(services transaction.TransactionServices) (any, error) {
		return services.Collaboration.ProposeMemory(ctx, actor.OrganizationID, actor.TeamID, request.TaskId, request.AgentId, request.Content)
	}, func(value any) any { return newMemoryCandidateResponse(value.(collaborationdomain.MemoryCandidate)) })
}

func (service *GeneratedServices) DecideMemoryCandidate(ctx context.Context, request *collaborationv1.DecideMemoryCandidateRequest) (*collaborationv1.DecideMemoryCandidateResponse, error) {
	if err := validCollaborationID(request.CandidateId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	result, err := service.executeWrite(ctx, actor, "memory-candidate.decide:"+request.CandidateId, "", request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		candidate, memory, err := services.Collaboration.DecideMemory(ctx, actor.OrganizationID, actor.TeamID, request.CandidateId, actor.UserID, request.Approve)
		response := map[string]any{"candidate": newMemoryCandidateResponse(candidate)}
		if memory != nil {
			response["memory"] = newAgentMemoryResponse(*memory)
		}
		return encodeWriteResult(http.StatusOK, response, err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &collaborationv1.DecideMemoryCandidateResponse{})
}

func (service *GeneratedServices) ListAgentMemories(ctx context.Context, request *collaborationv1.ListAgentMemoriesRequest) (*collaborationv1.ListAgentMemoriesResponse, error) {
	if err := validCollaborationID(request.AgentId); err != nil {
		return nil, err
	}
	actor, err := service.authorizeTeamRead(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	includeDeleted := request.IncludeDeleted != nil && *request.IncludeDeleted
	values, err := service.dependencies.Collaboration.ListAgentMemories(ctx, actor.OrganizationID, actor.TeamID, request.AgentId, includeDeleted)
	if err != nil {
		return nil, collaborationQueryError(err)
	}
	items := make([]agentMemoryResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newAgentMemoryResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &collaborationv1.ListAgentMemoriesResponse{})
}

func (service *GeneratedServices) UpdateAgentMemory(ctx context.Context, request *collaborationv1.UpdateAgentMemoryRequest) (*collaborationv1.AgentMemory, error) {
	if err := validCollaborationID(request.MemoryId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return collaborationWrite(service, ctx, actor, "agent-memory.update:"+request.MemoryId, strconv.FormatInt(version, 10), request, http.StatusOK, &collaborationv1.AgentMemory{}, func(services transaction.TransactionServices) (any, error) {
		return services.Collaboration.UpdateAgentMemory(ctx, actor.OrganizationID, actor.TeamID, request.MemoryId, request.Content, request.Enabled, version)
	}, func(value any) any { return newAgentMemoryResponse(value.(collaborationdomain.AgentMemory)) })
}

func (service *GeneratedServices) DeleteAgentMemory(ctx context.Context, request *collaborationv1.DeleteAgentMemoryRequest) (*collaborationv1.AgentMemory, error) {
	if err := validCollaborationID(request.MemoryId); err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.authorizeTaskUse(ctx, request.TeamId)
	if err != nil {
		return nil, err
	}
	return collaborationWrite(service, ctx, actor, "agent-memory.delete:"+request.MemoryId, strconv.FormatInt(version, 10), request, http.StatusOK, &collaborationv1.AgentMemory{}, func(services transaction.TransactionServices) (any, error) {
		return services.Collaboration.DeleteAgentMemory(ctx, actor.OrganizationID, actor.TeamID, request.MemoryId, version)
	}, func(value any) any { return newAgentMemoryResponse(value.(collaborationdomain.AgentMemory)) })
}

func sessionMemory(input *typesv1.SessionMemory) collaborationdomain.SessionMemory {
	if input == nil {
		return collaborationdomain.SessionMemory{}
	}
	result := collaborationdomain.SessionMemory{ConfirmedDecisions: append([]string(nil), input.ConfirmedDecisions...), Results: append([]string(nil), input.Results...), WorkspaceSnapshots: append([]string(nil), input.WorkspaceSnapshots...)}
	if input.Summary != nil {
		result.Summary = *input.Summary
	}
	return result
}

func collaborationWrite[T proto.Message](service *GeneratedServices, ctx context.Context, actor identitydomain.Actor, operation, version string, request proto.Message, status int, output T, call func(transaction.TransactionServices) (any, error), convert func(any) any) (T, error) {
	result, err := service.executeWrite(ctx, actor, operation, version, request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := call(services)
		return encodeWriteResult(status, convert(value), err)
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return mappedWriteResponse(ctx, result, output)
}
