package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	approvalv1 "agent-platform/backend/api/approval/v1"
	artifactv1 "agent-platform/backend/api/artifact/v1"
	auditv1 "agent-platform/backend/api/audit/v1"
	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	artifactdomain "agent-platform/backend/internal/biz/artifact/domain"
	auditdomain "agent-platform/backend/internal/biz/audit/domain"
	transaction "agent-platform/backend/internal/biz/transaction"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

func (service *GeneratedServices) ListRunApprovals(ctx context.Context, request *approvalv1.ListRunApprovalsRequest) (*approvalv1.ListRunApprovalsResponse, error) {
	if _, err := uuid.Parse(request.RunId); err != nil {
		return nil, publicError(400, "invalid_run_id")
	}
	if err := service.dependencies.Access.AuthorizeRunRead(ctx, "", request.RunId); err != nil {
		return nil, mapAuthorizationError(err, "run_access_denied")
	}
	values, err := service.dependencies.Approvals.ListByRun(ctx, request.RunId)
	if err != nil {
		return nil, publicError(500, "approval_query_failed")
	}
	items := make([]runApprovalResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newRunApprovalResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &approvalv1.ListRunApprovalsResponse{})
}

func (service *GeneratedServices) GetRunApproval(ctx context.Context, request *approvalv1.GetRunApprovalRequest) (*approvalv1.RunApproval, error) {
	value, err := service.scopedApproval(ctx, request.ApprovalId)
	if err != nil {
		return nil, err
	}
	return mappedResponse(ctx, http.StatusOK, newRunApprovalResponse(value), &approvalv1.RunApproval{})
}

func (service *GeneratedServices) RequestRunApproval(ctx context.Context, request *approvalv1.RequestRunApprovalRequest) (*approvalv1.RunApproval, error) {
	if _, err := uuid.Parse(request.RunId); err != nil {
		return nil, publicError(400, "invalid_resource_id")
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.RunControlAccess.AuthorizeRunControl(ctx, "", request.RunId, "approval_request")
	if err != nil {
		return nil, mapAuthorizationError(err, "run_control_denied")
	}
	if request.Request == nil {
		return nil, publicError(400, "invalid_request_body")
	}
	openRequest, err := json.Marshal(request.Request.AsMap())
	if err != nil {
		return nil, publicError(400, "invalid_request_body")
	}
	result, err := service.executeWrite(ctx, actor, "approval.request:"+request.RunId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		value, err := services.Approvals.Request(ctx, request.RunId, approvaldomain.Kind(request.Kind), openRequest, actor.UserID, version)
		return encodeWriteResult(http.StatusCreated, newRunApprovalResponse(value), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &approvalv1.RunApproval{})
}

func (service *GeneratedServices) DecideRunApproval(ctx context.Context, request *approvalv1.DecideRunApprovalRequest) (*approvalv1.RunApproval, error) {
	value, err := service.scopedApproval(ctx, request.ApprovalId)
	if err != nil {
		return nil, err
	}
	version, err := expectedVersion(ctx)
	if err != nil {
		return nil, err
	}
	actor, err := service.dependencies.RunControlAccess.AuthorizeRunControl(ctx, "", value.RunID, "approval_decide")
	if err != nil {
		return nil, mapAuthorizationError(err, "run_control_denied")
	}
	result, err := service.executeWrite(ctx, actor, "approval.decide:"+request.ApprovalId, strconv.FormatInt(version, 10), request, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		updated, err := services.Approvals.Decide(ctx, request.ApprovalId, version, request.Approved, actor.UserID, request.Reason)
		return encodeWriteResult(http.StatusOK, newRunApprovalResponse(updated), err)
	})
	if err != nil {
		return nil, err
	}
	return mappedWriteResponse(ctx, result, &approvalv1.RunApproval{})
}

func (service *GeneratedServices) scopedApproval(ctx context.Context, id string) (approvaldomain.Approval, error) {
	if _, err := uuid.Parse(id); err != nil {
		return approvaldomain.Approval{}, publicError(400, "invalid_approval_id")
	}
	scope, err := service.dependencies.ResourceAccess.ResolveReadScope(ctx, "")
	if err != nil {
		mapped := mapAuthorizationError(err, "approval_not_found")
		if public := kratoserrors.FromError(mapped); public.Code == 403 {
			return approvaldomain.Approval{}, publicError(404, "approval_not_found")
		}
		return approvaldomain.Approval{}, mapped
	}
	value, err := service.dependencies.Approvals.GetInScope(ctx, id, scope)
	if errors.Is(err, approvaldomain.ErrNotFound) {
		return approvaldomain.Approval{}, publicError(404, "approval_not_found")
	}
	if err != nil {
		return approvaldomain.Approval{}, publicError(500, "approval_query_failed")
	}
	return value, nil
}

func (service *GeneratedServices) ListRunArtifacts(ctx context.Context, request *artifactv1.ListRunArtifactsRequest) (*artifactv1.ListRunArtifactsResponse, error) {
	if _, err := uuid.Parse(request.RunId); err != nil {
		return nil, publicError(400, "invalid_run_id")
	}
	if err := service.dependencies.Access.AuthorizeRunRead(ctx, "", request.RunId); err != nil {
		return nil, mapAuthorizationError(err, "run_access_denied")
	}
	values, err := service.dependencies.Artifacts.ListByRun(ctx, request.RunId)
	if err != nil {
		return nil, publicError(500, "artifact_query_failed")
	}
	items := make([]artifactResponse, 0, len(values))
	for _, value := range values {
		items = append(items, newArtifactResponse(value))
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &artifactv1.ListRunArtifactsResponse{})
}

func (service *GeneratedServices) GetArtifactDownload(ctx context.Context, request *artifactv1.GetArtifactDownloadRequest) (*artifactv1.GetArtifactDownloadResponse, error) {
	if _, err := uuid.Parse(request.ArtifactId); err != nil {
		return nil, publicError(400, "invalid_artifact_id")
	}
	scope, err := service.dependencies.ResourceAccess.ResolveReadScope(ctx, "")
	if err != nil {
		return nil, publicError(404, "artifact_not_found")
	}
	artifact, err := service.dependencies.Artifacts.GetInScope(ctx, request.ArtifactId, scope)
	if errors.Is(err, artifactdomain.ErrNotFound) {
		return nil, publicError(404, "artifact_not_found")
	}
	if err != nil {
		return nil, publicError(500, "artifact_query_failed")
	}
	signed, err := service.dependencies.Artifacts.PresignDownload(ctx, artifact, 5*time.Minute)
	if err != nil {
		return nil, publicError(503, "artifact_download_unavailable")
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"url": signed.URL, "expires_at": signed.ExpiresAt}, &artifactv1.GetArtifactDownloadResponse{})
}

func (service *GeneratedServices) ListAuditEvents(ctx context.Context, request *auditv1.ListAuditEventsRequest) (*auditv1.ListAuditEventsResponse, error) {
	if _, err := uuid.Parse(request.TeamId); err != nil {
		return nil, publicError(400, "valid_team_id_required")
	}
	actor, err := service.dependencies.AuditAccess.AuthorizeTeamRead(ctx, "", request.TeamId)
	if err != nil {
		return nil, mapAuthorizationError(err, "audit_access_denied")
	}
	query := auditdomain.Query{OrganizationID: actor.OrganizationID, TeamID: request.TeamId, Limit: 100}
	if request.Action != nil {
		query.Action = *request.Action
	}
	if request.ResourceType != nil {
		query.ResourceType = *request.ResourceType
	}
	if request.ResourceId != nil {
		query.ResourceID = *request.ResourceId
	}
	if request.ActorUserId != nil {
		query.ActorUserID = *request.ActorUserId
		if _, err := uuid.Parse(query.ActorUserID); err != nil {
			return nil, publicError(400, "invalid_audit_query")
		}
	}
	if request.Outcome != nil {
		query.Outcome = *request.Outcome
		if query.Outcome != "succeeded" && query.Outcome != "failed" {
			return nil, publicError(400, "invalid_audit_query")
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
	if query.Limit <= 0 || query.Limit > 200 || query.CreatedFrom != nil && query.CreatedTo != nil && query.CreatedFrom.After(*query.CreatedTo) {
		return nil, publicError(400, "invalid_audit_query")
	}
	values, err := service.dependencies.Audit.Search(ctx, query)
	if err != nil {
		return nil, publicError(500, "audit_query_failed")
	}
	items := make([]auditEventResponse, 0, len(values))
	for _, value := range values {
		items = append(items, auditEventResponse{ID: value.ID, TeamID: value.TeamID, ActorUserID: value.ActorUserID, Outcome: value.Outcome, Action: value.Action, ResourceType: value.ResourceType, ResourceID: value.ResourceID, Details: value.Details, CreatedAt: value.CreatedAt})
	}
	return mappedResponse(ctx, http.StatusOK, map[string]any{"items": items}, &auditv1.ListAuditEventsResponse{})
}
