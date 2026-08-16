package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"

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
	clock      Clock
	ids        IDGenerator
}

type RegisterCommand struct {
	OrganizationID string
	Name           string
	Kind           domain.Kind
	BaseURL        string
}

type ChangeStatusCommand struct {
	OrganizationID  string
	ID              string
	ExpectedVersion int64
	Enabled         bool
}

func New(repository domain.Repository) *Service {
	return NewWithDependencies(repository, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, clock: clock, ids: ids}
}

func (service *Service) Register(ctx context.Context, command RegisterCommand) (domain.Provider, error) {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return domain.Provider{}, fmt.Errorf("Source Control Catalog dependencies are required")
	}
	provider, err := domain.Register(domain.Registration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, Name: command.Name,
		Kind: command.Kind, BaseURL: command.BaseURL, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.Provider{}, err
	}
	if err := service.repository.Create(ctx, provider); err != nil {
		return domain.Provider{}, err
	}
	return provider, nil
}

func (service *Service) Get(ctx context.Context, organizationID, id string) (domain.Provider, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(id) == "" {
		return domain.Provider{}, fmt.Errorf("Source Control Provider Repository, Organization ID, and ID are required")
	}
	return service.repository.Get(ctx, organizationID, id)
}

func (service *Service) List(ctx context.Context, organizationID string) ([]domain.Provider, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("Source Control Provider Repository and Organization ID are required")
	}
	return service.repository.List(ctx, organizationID)
}

func (service *Service) ChangeStatus(ctx context.Context, command ChangeStatusCommand) (domain.Provider, error) {
	if service.repository == nil || service.clock == nil || command.ExpectedVersion <= 0 {
		return domain.Provider{}, fmt.Errorf("Source Control Provider dependencies and expected version are required")
	}
	provider, err := service.repository.Get(ctx, command.OrganizationID, command.ID)
	if err != nil {
		return domain.Provider{}, err
	}
	if provider.Version != command.ExpectedVersion {
		return domain.Provider{}, domain.ErrConcurrentUpdate
	}
	originalVersion := provider.Version
	if err := provider.SetEnabled(command.Enabled, service.clock.Now()); err != nil {
		return domain.Provider{}, err
	}
	if provider.Version != originalVersion {
		if err := service.repository.UpdateStatus(ctx, provider, originalVersion); err != nil {
			return domain.Provider{}, err
		}
	}
	return provider, nil
}
