package runtimeapproval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/biz/transaction"
	approvalgorm "agent-platform/backend/internal/data/approval/gormrepo"

	"gorm.io/gorm"
)

const (
	pollInterval       = 100 * time.Millisecond
	pollErrorLimit     = 3
	idempotencyKeyLife = 24 * time.Hour
	abortTimeout       = 10 * time.Second
)

type Gate struct {
	database *gorm.DB
	writes   transaction.IdempotentTransactionManager
}

func New(database *gorm.DB, writes transaction.IdempotentTransactionManager) (*Gate, error) {
	if database == nil || writes == nil {
		return nil, fmt.Errorf("Runtime Approval database and Unit of Work are required")
	}
	return &Gate{database: database, writes: writes}, nil
}

var _ executionapplication.RuntimeApprovalGate = (*Gate)(nil)

type runIdentity struct {
	OrganizationID string `gorm:"column:organization_id"`
	TeamID         string `gorm:"column:team_id"`
	RequestedBy    string `gorm:"column:requested_by"`
	Version        int64  `gorm:"column:version"`
}

func (gate *Gate) AwaitDecision(ctx context.Context, request executionapplication.RuntimeApprovalRequest) error {
	if strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.AttemptID) == "" || request.Sequence <= 0 || strings.TrimSpace(request.Kind) == "" || len(request.Request) == 0 || !json.Valid(request.Request) {
		return fmt.Errorf("valid Runtime Approval Run, Attempt, sequence, kind, and request are required")
	}
	identity, err := gate.identity(ctx, request.RunID)
	if err != nil {
		return err
	}
	approvalID, err := gate.request(ctx, request, identity)
	if err != nil {
		return err
	}
	repository := approvalgorm.New(gate.database)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	consecutivePollErrors := 0
	for {
		approval, getErr := repository.Get(ctx, approvalID)
		if getErr == nil {
			consecutivePollErrors = 0
			switch approval.State {
			case approvaldomain.StateApproved:
				return nil
			case approvaldomain.StateRejected:
				return fmt.Errorf("%w: %s", executiondomain.ErrApprovalRejected, strings.TrimSpace(approval.DecisionReason))
			case approvaldomain.StatePending:
			default:
				return fmt.Errorf("unsupported Runtime Approval state %q", approval.State)
			}
		} else if errors.Is(getErr, approvaldomain.ErrNotFound) {
			return getErr
		} else {
			consecutivePollErrors++
			if consecutivePollErrors >= pollErrorLimit {
				pollErr := fmt.Errorf("poll Runtime Approval decision: %w", getErr)
				return errors.Join(pollErr, gate.rejectAbandoned(ctx, approvalID, identity, pollErr))
			}
		}
		select {
		case <-ctx.Done():
			return gate.rejectAbandoned(ctx, approvalID, identity, ctx.Err())
		case <-ticker.C:
		}
	}
}

func (gate *Gate) identity(ctx context.Context, runID string) (runIdentity, error) {
	var identity runIdentity
	err := gate.database.WithContext(ctx).Table("runs AS run").
		Select("task.organization_id, task.team_id, run.created_by AS requested_by, run.version").
		Joins("JOIN sessions AS session ON session.id = run.session_id").
		Joins("JOIN coding_tasks AS task ON task.id = session.coding_task_id").
		Where("run.id = ?", runID).Take(&identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return runIdentity{}, executiondomain.ErrRunNotFound
	}
	if err != nil {
		return runIdentity{}, fmt.Errorf("load Runtime Approval identity: %w", err)
	}
	return identity, nil
}

func (gate *Gate) request(ctx context.Context, request executionapplication.RuntimeApprovalRequest, identity runIdentity) (string, error) {
	digest := requestDigest(request)
	result, err := gate.writes.Execute(ctx, transaction.IdempotencyRequest{
		OrganizationID: identity.OrganizationID,
		TeamID:         identity.TeamID,
		ActorUserID:    identity.RequestedBy,
		Key:            fmt.Sprintf("runtime-approval:%s:%s:%d", request.RunID, request.AttemptID, request.Sequence),
		Operation:      "approval.request:" + request.RunID,
		RequestSHA256:  digest,
		ExpiresAt:      time.Now().UTC().Add(idempotencyKeyLife),
	}, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		approval, requestErr := services.Approvals.Request(ctx, request.RunID, approvaldomain.Kind(request.Kind), request.Request, identity.RequestedBy, identity.Version)
		body, marshalErr := json.Marshal(map[string]any{"id": approval.ID, "run_id": approval.RunID, "state": approval.State})
		if marshalErr != nil {
			return transaction.IdempotencyResult{}, marshalErr
		}
		return transaction.IdempotencyResult{Status: http.StatusCreated, Body: body}, requestErr
	})
	if err != nil {
		return "", err
	}
	var response struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(result.Body, &response) != nil || strings.TrimSpace(response.ID) == "" {
		return "", fmt.Errorf("Runtime Approval response is missing its ID")
	}
	return response.ID, nil
}

func (gate *Gate) rejectAbandoned(ctx context.Context, approvalID string, identity runIdentity, waitErr error) error {
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortTimeout)
	defer cancel()
	repository := approvalgorm.New(gate.database)
	approval, err := repository.Get(abortCtx, approvalID)
	if err != nil {
		return errors.Join(waitErr, fmt.Errorf("load abandoned Runtime Approval: %w", err))
	}
	if approval.State == approvaldomain.StateRejected {
		return fmt.Errorf("%w: Runtime Approval wait ended", executiondomain.ErrApprovalRejected)
	}
	if approval.State == approvaldomain.StateApproved {
		return nil
	}
	reason := "Runtime Approval wait ended before a decision"
	requestBody, _ := json.Marshal(map[string]any{"approved": false, "reason": reason, "version": approval.Version})
	digest := sha256.Sum256(requestBody)
	_, err = gate.writes.Execute(abortCtx, transaction.IdempotencyRequest{
		OrganizationID: identity.OrganizationID,
		TeamID:         identity.TeamID,
		SystemActor:    true,
		Key:            "runtime-approval-abort:" + approval.ID,
		Operation:      "approval.decide:" + approval.ID,
		RequestSHA256:  hex.EncodeToString(digest[:]),
		ExpiresAt:      time.Now().UTC().Add(idempotencyKeyLife),
	}, func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		updated, decideErr := services.Approvals.RejectBySystem(abortCtx, approval.ID, approval.Version, reason)
		body, marshalErr := json.Marshal(map[string]any{"id": updated.ID, "run_id": updated.RunID, "state": updated.State})
		if marshalErr != nil {
			return transaction.IdempotencyResult{}, marshalErr
		}
		return transaction.IdempotencyResult{Status: http.StatusOK, Body: body}, decideErr
	})
	if err != nil {
		return errors.Join(waitErr, fmt.Errorf("reject abandoned Runtime Approval: %w", err))
	}
	return fmt.Errorf("%w: %s", executiondomain.ErrApprovalRejected, reason)
}

func requestDigest(request executionapplication.RuntimeApprovalRequest) string {
	encoded, _ := json.Marshal(struct {
		RunID     string          `json:"run_id"`
		AttemptID string          `json:"attempt_id"`
		Sequence  int64           `json:"sequence"`
		Kind      string          `json:"kind"`
		Request   json.RawMessage `json:"request"`
	}{request.RunID, request.AttemptID, request.Sequence, request.Kind, request.Request})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}
