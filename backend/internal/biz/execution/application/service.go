package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/biz/workflow"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type Service struct {
	repository domain.Repository
	completion workflow.Completion
	clock      Clock
}

func New(repository domain.Repository, completion ...workflow.Completion) *Service {
	return NewWithClock(repository, systemClock{}, completion...)
}

func NewWithClock(repository domain.Repository, clock Clock, completion ...workflow.Completion) *Service {
	coordinator := workflow.Completion(ownedCompletion{repository: repository})
	if len(completion) > 0 && completion[0] != nil {
		coordinator = completion[0]
	}
	return &Service{repository: repository, completion: coordinator, clock: clock}
}

type ownedCompletion struct{ repository domain.Repository }

func (completion ownedCompletion) Finish(ctx context.Context, token string, outcome domain.Outcome, now time.Time) error {
	_, err := completion.repository.FinishOwned(ctx, token, outcome, now)
	return err
}

func (completion ownedCompletion) Control(ctx context.Context, runID string, expectedVersion int64, action domain.ControlAction, actorUserID string, now time.Time) (domain.Details, error) {
	details, _, err := completion.repository.Control(ctx, runID, expectedVersion, action, actorUserID, now)
	return details, err
}

func (completion ownedCompletion) ReconcileExpired(ctx context.Context, maxAttempts int, now time.Time) (domain.ReconcileResult, error) {
	result, _, err := completion.repository.ReconcileExpired(ctx, maxAttempts, now)
	return result, err
}

func (service *Service) Get(ctx context.Context, runID string) (domain.Details, error) {
	if service.repository == nil {
		return domain.Details{}, fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(runID) == "" {
		return domain.Details{}, fmt.Errorf("Run ID is required")
	}
	return service.repository.Get(ctx, runID)
}

func (service *Service) Search(ctx context.Context, query domain.SearchQuery) ([]domain.Details, error) {
	repository, ok := service.repository.(domain.SearchRepository)
	if !ok || strings.TrimSpace(query.OrganizationID) == "" || strings.TrimSpace(query.TeamID) == "" {
		return nil, fmt.Errorf("searchable Run Repository and organization/Team scope are required")
	}
	if query.Limit <= 0 || query.Limit > 100 {
		return nil, fmt.Errorf("Run search limit must be between 1 and 100")
	}
	if query.State != "" {
		if _, err := domain.ParseState(string(query.State)); err != nil {
			return nil, err
		}
	}
	if query.CreatedFrom != nil && query.CreatedTo != nil && query.CreatedFrom.After(*query.CreatedTo) {
		return nil, fmt.Errorf("Run search start time must not follow end time")
	}
	return repository.Search(ctx, query)
}

func (service *Service) Claim(ctx context.Context, workerID string, leaseDuration time.Duration) (domain.Lease, bool, error) {
	if service.repository == nil {
		return domain.Lease{}, false, fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(workerID) == "" || leaseDuration <= 0 {
		return domain.Lease{}, false, fmt.Errorf("Worker ID and positive lease duration are required")
	}
	return service.repository.Claim(ctx, workerID, leaseDuration, service.clock.Now())
}

func (service *Service) Renew(ctx context.Context, token string, leaseDuration time.Duration) error {
	if service.repository == nil {
		return fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(token) == "" || leaseDuration <= 0 {
		return fmt.Errorf("lease token and positive duration are required")
	}
	return service.repository.Renew(ctx, token, leaseDuration, service.clock.Now())
}

func (service *Service) MarkRunning(ctx context.Context, token string) error {
	if service.repository == nil {
		return fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("lease token is required")
	}
	return service.repository.MarkRunning(ctx, token, service.clock.Now())
}

func (service *Service) Finish(ctx context.Context, token string, outcome domain.Outcome) error {
	if service.repository == nil {
		return fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("lease token is required")
	}
	outcome.Normalize()
	if err := outcome.Validate(); err != nil {
		return err
	}
	if service.completion == nil {
		return fmt.Errorf("Run Completion Workflow is required")
	}
	return service.completion.Finish(ctx, token, outcome, service.clock.Now())
}

func (service *Service) AppendEvent(ctx context.Context, token string, event domain.EventInput) error {
	if service.repository == nil {
		return fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(token) == "" || !validRuntimeEventType(event.Type) || len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return fmt.Errorf("valid lease token and Runtime Event are required")
	}
	return service.repository.AppendEvent(ctx, token, event, service.clock.Now())
}

func validRuntimeEventType(value string) bool {
	switch agentruntime.EventKind(value) {
	case agentruntime.EventRuntimeStarted,
		agentruntime.EventMessageDelta,
		agentruntime.EventMessageCompleted,
		agentruntime.EventCommandRequested,
		agentruntime.EventCommandCompleted,
		agentruntime.EventFileChanged,
		agentruntime.EventUsageUpdated,
		agentruntime.EventCheckpointSaved,
		agentruntime.EventWorkflowDelivered,
		agentruntime.EventRuntimeCompleted,
		agentruntime.EventRuntimeFailed:
		return true
	default:
		return false
	}
}

func (service *Service) ReconcileExpired(ctx context.Context, maxAttempts int) (domain.ReconcileResult, error) {
	if service.repository == nil {
		return domain.ReconcileResult{}, fmt.Errorf("Run Repository is required")
	}
	if maxAttempts <= 0 {
		return domain.ReconcileResult{}, fmt.Errorf("maximum Attempts must be positive")
	}
	return service.completion.ReconcileExpired(ctx, maxAttempts, service.clock.Now())
}

func (service *Service) ListEventsAfter(ctx context.Context, runID string, after int64, limit int) ([]domain.Event, error) {
	if service.repository == nil {
		return nil, fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(runID) == "" || after < 0 || limit <= 0 || limit > 500 {
		return nil, fmt.Errorf("invalid Run event query")
	}
	return service.repository.ListEventsAfter(ctx, runID, after, limit)
}

func (service *Service) Control(ctx context.Context, runID string, expectedVersion int64, action domain.ControlAction, actorUserID string) (domain.Details, error) {
	if service.repository == nil {
		return domain.Details{}, fmt.Errorf("Run Repository is required")
	}
	if strings.TrimSpace(runID) == "" || expectedVersion <= 0 || strings.TrimSpace(actorUserID) == "" {
		return domain.Details{}, fmt.Errorf("Run ID, expected Version, and actor are required")
	}
	switch action {
	case domain.ControlInterrupt, domain.ControlResume, domain.ControlCancel, domain.ControlKill:
	default:
		return domain.Details{}, fmt.Errorf("unknown Run control action %q", action)
	}
	return service.completion.Control(ctx, runID, expectedVersion, action, actorUserID, service.clock.Now())
}
