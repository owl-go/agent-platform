package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/approval/domain"
	"agent-platform/backend/internal/biz/authz"
	"agent-platform/backend/internal/biz/workflow"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() string }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

type Service struct {
	repository domain.Repository
	workflow   workflow.Approval
	clock      Clock
	ids        IDGenerator
}

func New(repository domain.Repository, approvalWorkflow workflow.Approval) *Service {
	return NewWithDependencies(repository, approvalWorkflow, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, approvalWorkflow workflow.Approval, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, workflow: approvalWorkflow, clock: clock, ids: ids}
}

func (service *Service) Get(ctx context.Context, id string) (domain.Approval, error) {
	if service.repository == nil || strings.TrimSpace(id) == "" {
		return domain.Approval{}, fmt.Errorf("Run Approval Repository and ID are required")
	}
	return service.repository.Get(ctx, id)
}

func (service *Service) GetInScope(ctx context.Context, id string, scope authz.ReadScope) (domain.Approval, error) {
	if service.repository == nil || strings.TrimSpace(id) == "" || !scope.Valid() {
		return domain.Approval{}, fmt.Errorf("Run Approval Repository, ID, and authorized read scope are required")
	}
	return service.repository.GetInScope(ctx, id, scope)
}

func (service *Service) ListByRun(ctx context.Context, runID string) ([]domain.Approval, error) {
	if service.repository == nil || strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("Run Approval Repository and Run ID are required")
	}
	return service.repository.ListByRun(ctx, runID)
}

func (service *Service) Request(ctx context.Context, runID string, kind domain.Kind, request json.RawMessage, requestedBy string, expectedRunVersion int64) (domain.Approval, error) {
	if service.repository == nil || service.workflow == nil || service.clock == nil || service.ids == nil || expectedRunVersion <= 0 {
		return domain.Approval{}, fmt.Errorf("Run Approval dependencies and expected Run Version are required")
	}
	now := service.clock.Now()
	approval, err := domain.Request(service.ids.NewID(), runID, kind, request, requestedBy, now)
	if err != nil {
		return domain.Approval{}, err
	}
	if err := service.workflow.Request(ctx, approval, expectedRunVersion, now); err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
}

func (service *Service) Decide(ctx context.Context, id string, expectedVersion int64, approved bool, actor, reason string) (domain.Approval, error) {
	if service.repository == nil || service.workflow == nil || service.clock == nil || expectedVersion <= 0 {
		return domain.Approval{}, fmt.Errorf("Run Approval dependencies and expected Version are required")
	}
	approval, err := service.repository.Get(ctx, id)
	if err != nil {
		return domain.Approval{}, err
	}
	if approval.Version != expectedVersion {
		return domain.Approval{}, domain.ErrConcurrentUpdate
	}
	now := service.clock.Now()
	if err := approval.Decide(approved, actor, reason, now); err != nil {
		return domain.Approval{}, err
	}
	if err := service.workflow.Decide(ctx, approval, expectedVersion, now); err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
}

func (service *Service) RejectBySystem(ctx context.Context, id string, expectedVersion int64, reason string) (domain.Approval, error) {
	if service.repository == nil || service.workflow == nil || service.clock == nil || expectedVersion <= 0 {
		return domain.Approval{}, fmt.Errorf("Run Approval dependencies and expected Version are required")
	}
	approval, err := service.repository.Get(ctx, id)
	if err != nil {
		return domain.Approval{}, err
	}
	if approval.Version != expectedVersion {
		return domain.Approval{}, domain.ErrConcurrentUpdate
	}
	now := service.clock.Now()
	if err := approval.RejectBySystem(reason, now); err != nil {
		return domain.Approval{}, err
	}
	if err := service.workflow.Decide(ctx, approval, expectedVersion, now); err != nil {
		return domain.Approval{}, err
	}
	return approval, nil
}
