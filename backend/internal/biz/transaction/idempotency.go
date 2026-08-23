package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	agentapplication "agent-platform/backend/internal/biz/agentlifecycle/application"
	approvalapplication "agent-platform/backend/internal/biz/approval/application"
	collaborationapplication "agent-platform/backend/internal/biz/collaboration/application"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	modelapplication "agent-platform/backend/internal/biz/modelcatalog/application"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
)

var (
	ErrIdempotencyConflict = errors.New("Idempotency Key was reused with a different request")
	hashPattern            = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type IdempotencyRequest struct {
	OrganizationID string
	TeamID         string
	ActorUserID    string
	SystemActor    bool
	Key            string
	Operation      string
	RequestSHA256  string
	ExpiresAt      time.Time
}

type IdempotencyResult struct {
	Status   int
	Body     json.RawMessage
	Replayed bool
}

// TransactionServices exist only for the duration of one database transaction.
// They must never be retained by a handler or placed in a Wire singleton.
type TransactionServices struct {
	RuntimeImages *runtimeapplication.Service
	Models        *modelapplication.Service
	SourceControl *sourceapplication.Service
	Bindings      *sourceapplication.BindingService
	Agents        *agentapplication.Service
	Approvals     *approvalapplication.Service
	Collaboration *collaborationapplication.Service
	Runs          *executionapplication.Service
}

type IdempotencyHandler func(TransactionServices) (IdempotencyResult, error)

type IdempotentTransactionManager interface {
	Execute(context.Context, IdempotencyRequest, IdempotencyHandler) (IdempotencyResult, error)
}

func (request IdempotencyRequest) Validate(now time.Time) error {
	if strings.TrimSpace(request.OrganizationID) == "" || strings.TrimSpace(request.Key) == "" || strings.TrimSpace(request.Operation) == "" {
		return fmt.Errorf("Organization ID, Idempotency Key, and operation are required")
	}
	if len(request.Key) > 200 || len(request.Operation) > 200 {
		return fmt.Errorf("Idempotency Key or operation is too long")
	}
	if request.SystemActor && strings.TrimSpace(request.ActorUserID) != "" {
		return fmt.Errorf("system Idempotency actor cannot also identify a user")
	}
	if !hashPattern.MatchString(request.RequestSHA256) {
		return fmt.Errorf("request SHA-256 is invalid")
	}
	if request.ExpiresAt.IsZero() || !request.ExpiresAt.After(now) {
		return fmt.Errorf("Idempotency Key expiry must be in the future")
	}
	return nil
}

func (result IdempotencyResult) Validate() error {
	if result.Status < 100 || result.Status > 599 {
		return fmt.Errorf("idempotent response status is invalid")
	}
	if len(result.Body) == 0 || !json.Valid(result.Body) {
		return fmt.Errorf("idempotent response body must be valid JSON")
	}
	return nil
}
