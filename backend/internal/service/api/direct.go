package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	agentdomain "agent-platform/backend/internal/biz/agentlifecycle/domain"
	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"
	transaction "agent-platform/backend/internal/biz/transaction"
	"agent-platform/backend/internal/transportmeta"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	internalResponseBodyHeader   = "X-Agent-Platform-Internal-Response-Body"
	internalResponseStatusHeader = "X-Agent-Platform-Internal-Response-Status"
	idempotencyKeyLife           = 24 * time.Hour
)

func publicError(status int, code string) error {
	return kratoserrors.New(status, code, code)
}

func requestHeader(ctx context.Context, name string) string {
	transporter, ok := transport.FromServerContext(ctx)
	if !ok || transporter.RequestHeader() == nil {
		return ""
	}
	return transporter.RequestHeader().Get(name)
}

func setMappedHTTPResponse(ctx context.Context, status int, body []byte, replayed bool) {
	transporter, ok := transport.FromServerContext(ctx)
	if !ok || transporter.Kind() != transport.KindHTTP || transporter.ReplyHeader() == nil {
		return
	}
	transporter.ReplyHeader().Set(internalResponseStatusHeader, strconv.Itoa(status))
	transporter.ReplyHeader().Set(internalResponseBodyHeader, string(body))
	if replayed {
		transporter.ReplyHeader().Set("Idempotency-Replayed", "true")
	}
}

func mappedResponse[T proto.Message](ctx context.Context, status int, legacy any, output T) (T, error) {
	body, err := json.Marshal(legacy)
	if err != nil {
		var zero T
		return zero, publicError(500, "response_mapping_failed")
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(body, output); err != nil {
		var zero T
		return zero, publicError(500, "response_mapping_failed")
	}
	setMappedHTTPResponse(ctx, status, body, false)
	return output, nil
}

func mappedWriteResponse[T proto.Message](ctx context.Context, result transaction.IdempotencyResult, output T) (T, error) {
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(result.Body, output); err != nil {
		var zero T
		return zero, publicError(500, "response_mapping_failed")
	}
	setMappedHTTPResponse(ctx, result.Status, result.Body, result.Replayed)
	return output, nil
}

func expectedVersion(ctx context.Context) (int64, error) {
	value := strings.Trim(strings.TrimSpace(requestHeader(ctx, "If-Match")), `"`)
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version <= 0 {
		return 0, publicError(428, "valid_if_match_required")
	}
	return version, nil
}

func (service *GeneratedServices) executeWrite(
	ctx context.Context,
	actor identitydomain.Actor,
	operation string,
	version string,
	request proto.Message,
	handler transaction.IdempotencyHandler,
) (transaction.IdempotencyResult, error) {
	key := strings.TrimSpace(requestHeader(ctx, "Idempotency-Key"))
	if key == "" {
		return transaction.IdempotencyResult{}, publicError(400, "idempotency_key_required")
	}
	body, ok := transportmeta.RawBodyFromContext(ctx)
	if !ok {
		var err error
		body, err = protojson.MarshalOptions{UseProtoNames: true}.Marshal(request)
		if err != nil {
			return transaction.IdempotencyResult{}, publicError(400, "invalid_request_body")
		}
	}
	hashInput := append(append([]byte(nil), body...), []byte("\nversion:"+version)...)
	digest := sha256.Sum256(hashInput)
	result, err := service.dependencies.CatalogWrites.Execute(ctx, transaction.IdempotencyRequest{
		OrganizationID: actor.OrganizationID,
		TeamID:         actor.TeamID,
		ActorUserID:    actor.UserID,
		Key:            key,
		Operation:      operation,
		RequestSHA256:  hex.EncodeToString(digest[:]),
		ExpiresAt:      time.Now().UTC().Add(idempotencyKeyLife),
	}, handler)
	if err != nil {
		return transaction.IdempotencyResult{}, mapWriteError(err)
	}
	return result, nil
}

func mapAuthorizationError(err error, forbiddenCode string) error {
	switch {
	case errors.Is(err, identitydomain.ErrUnauthenticated):
		return publicError(401, "invalid_authentication")
	case errors.Is(err, identitydomain.ErrForbidden):
		return publicError(403, forbiddenCode)
	default:
		return publicError(503, "authorization_failed")
	}
}

func (service *GeneratedServices) authorizeTeamRead(ctx context.Context, teamID string) (identitydomain.Actor, error) {
	if _, err := uuid.Parse(teamID); err != nil {
		return identitydomain.Actor{}, publicError(400, "invalid_team_id")
	}
	actor, err := service.dependencies.AgentAccess.AuthorizeTeamRead(ctx, "", teamID)
	if err != nil {
		return identitydomain.Actor{}, mapAuthorizationError(err, "team_read_access_denied")
	}
	return actor, nil
}

func (service *GeneratedServices) authorizeAgentBuild(ctx context.Context, teamID string) (identitydomain.Actor, error) {
	if _, err := uuid.Parse(teamID); err != nil {
		return identitydomain.Actor{}, publicError(400, "invalid_team_id")
	}
	actor, err := service.dependencies.AgentAccess.AuthorizeAgentBuild(ctx, "", teamID)
	if err != nil {
		return identitydomain.Actor{}, mapAuthorizationError(err, "agent_build_access_denied")
	}
	return actor, nil
}

func (service *GeneratedServices) authorizeTaskUse(ctx context.Context, teamID string) (identitydomain.Actor, error) {
	if _, err := uuid.Parse(teamID); err != nil {
		return identitydomain.Actor{}, publicError(400, "invalid_team_id")
	}
	actor, err := service.dependencies.CollaborationAccess.AuthorizeTaskUse(ctx, "", teamID)
	if err != nil {
		return identitydomain.Actor{}, mapAuthorizationError(err, "collaboration_access_denied")
	}
	return actor, nil
}

func mapWriteError(err error) error {
	switch {
	case errors.Is(err, transaction.ErrIdempotencyConflict):
		return publicError(409, "idempotency_key_conflict")
	case errors.Is(err, runtimedomain.ErrConcurrentUpdate), errors.Is(err, modeldomain.ErrConcurrentUpdate),
		errors.Is(err, sourcedomain.ErrConcurrentUpdate), errors.Is(err, sourcedomain.ErrBindingConcurrentUpdate),
		errors.Is(err, agentdomain.ErrConcurrentUpdate), errors.Is(err, collaborationdomain.ErrConcurrentUpdate),
		errors.Is(err, executiondomain.ErrConcurrentModification), errors.Is(err, approvaldomain.ErrConcurrentUpdate):
		return publicError(412, "version_conflict")
	case errors.Is(err, runtimedomain.ErrRuntimeImageNotFound), errors.Is(err, modeldomain.ErrCredentialProfileNotFound),
		errors.Is(err, modeldomain.ErrConfiguredModelNotFound), errors.Is(err, sourcedomain.ErrProviderNotFound),
		errors.Is(err, sourcedomain.ErrBindingNotFound):
		return publicError(404, "catalog_resource_not_found")
	case errors.Is(err, agentdomain.ErrAgentNotFound), errors.Is(err, agentdomain.ErrDraftNotFound),
		errors.Is(err, agentdomain.ErrReleaseNotFound), errors.Is(err, agentdomain.ErrApprovalNotFound):
		return publicError(404, "agent_lifecycle_resource_not_found")
	case errors.Is(err, collaborationdomain.ErrTaskNotFound), errors.Is(err, collaborationdomain.ErrSessionNotFound),
		errors.Is(err, collaborationdomain.ErrMemoryCandidateNotFound), errors.Is(err, collaborationdomain.ErrAgentMemoryNotFound):
		return publicError(404, "collaboration_resource_not_found")
	case errors.Is(err, executiondomain.ErrRunNotFound):
		return publicError(404, "run_not_found")
	case errors.Is(err, approvaldomain.ErrNotFound):
		return publicError(404, "approval_not_found")
	case errors.Is(err, runtimedomain.ErrImageDigestExists), errors.Is(err, modeldomain.ErrCatalogNameExists),
		errors.Is(err, sourcedomain.ErrNameExists), errors.Is(err, sourcedomain.ErrBindingNameExists):
		return publicError(409, "catalog_resource_conflict")
	case errors.Is(err, agentdomain.ErrAgentNameExists), errors.Is(err, agentdomain.ErrDraftAlreadyReleased),
		errors.Is(err, agentdomain.ErrApprovalExists), errors.Is(err, agentdomain.ErrApprovalRequired):
		return publicError(409, "agent_lifecycle_conflict")
	case errors.Is(err, runtimedomain.ErrInvalidRuntimeImage), errors.Is(err, modeldomain.ErrInvalidCatalogInput),
		errors.Is(err, sourcedomain.ErrInvalidProvider), errors.Is(err, sourcedomain.ErrInvalidBinding):
		return publicError(422, "invalid_catalog_resource")
	case errors.Is(err, runtimeapplication.ErrInvalidEvidence):
		return publicError(422, "invalid_conformance_evidence")
	case errors.Is(err, runtimeapplication.ErrEvidenceUnavailable):
		return publicError(503, "conformance_evidence_unavailable")
	case errors.Is(err, agentdomain.ErrInvalidAgent), errors.Is(err, agentdomain.ErrDraftNotReady):
		return publicError(422, "invalid_agent_lifecycle_resource")
	case errors.Is(err, collaborationdomain.ErrReleaseUnavailable), errors.Is(err, collaborationdomain.ErrTaskStateConflict),
		errors.Is(err, collaborationdomain.ErrRunLimitReached):
		return publicError(409, "collaboration_conflict")
	case errors.Is(err, collaborationdomain.ErrRuntimeUnavailable):
		return publicError(409, "coding_task_runtime_unavailable")
	case errors.Is(err, collaborationdomain.ErrModelUnavailable):
		return publicError(409, "coding_task_model_unavailable")
	case errors.Is(err, collaborationdomain.ErrBindingUnavailable):
		return publicError(409, "coding_task_binding_unavailable")
	case errors.Is(err, executiondomain.ErrControlRejected):
		return publicError(409, "run_control_conflict")
	case errors.Is(err, approvaldomain.ErrPendingExists), errors.Is(err, approvaldomain.ErrRunState):
		return publicError(409, "approval_conflict")
	default:
		return publicError(500, "catalog_write_failed")
	}
}
