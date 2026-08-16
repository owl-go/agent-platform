package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/domain"

	"github.com/google/uuid"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

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
	Runtime        domain.Runtime
	CLIVersion     string
	AdapterVersion string
	ImageDigest    string
	Capabilities   map[string]bool
}

type ChangeStatusCommand struct {
	ID              string
	ExpectedVersion int64
	Status          domain.Status
	BlockedReason   string
}

func New(repository domain.Repository) *Service {
	return NewWithDependencies(repository, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, clock: clock, ids: ids}
}

func (service *Service) Register(ctx context.Context, command RegisterCommand) (domain.RuntimeImage, error) {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Catalog dependencies are required")
	}
	image, err := domain.Register(domain.Registration{
		ID: service.ids.NewID(), Runtime: command.Runtime, CLIVersion: command.CLIVersion,
		AdapterVersion: command.AdapterVersion, ImageDigest: command.ImageDigest,
		Capabilities: command.Capabilities, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.RuntimeImage{}, err
	}
	if err := service.repository.Create(ctx, image); err != nil {
		return domain.RuntimeImage{}, err
	}
	return image, nil
}

func (service *Service) Get(ctx context.Context, id string) (domain.RuntimeImage, error) {
	if service.repository == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Image Repository is required")
	}
	if strings.TrimSpace(id) == "" {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Image ID is required")
	}
	return service.repository.Get(ctx, id)
}

func (service *Service) List(ctx context.Context) ([]domain.RuntimeImage, error) {
	if service.repository == nil {
		return nil, fmt.Errorf("Runtime Image Repository is required")
	}
	return service.repository.List(ctx)
}

func (service *Service) ChangeStatus(ctx context.Context, command ChangeStatusCommand) (domain.RuntimeImage, error) {
	if service.repository == nil || service.clock == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Catalog dependencies are required")
	}
	if strings.TrimSpace(command.ID) == "" || command.ExpectedVersion <= 0 {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Image ID and expected version are required")
	}
	image, err := service.repository.Get(ctx, command.ID)
	if err != nil {
		return domain.RuntimeImage{}, err
	}
	if image.Version != command.ExpectedVersion {
		return domain.RuntimeImage{}, domain.ErrConcurrentUpdate
	}
	originalVersion := image.Version
	if err := image.ChangeStatus(command.Status, command.BlockedReason, service.clock.Now()); err != nil {
		return domain.RuntimeImage{}, err
	}
	if image.Version == originalVersion {
		return image, nil
	}
	if err := service.repository.UpdateStatus(ctx, image, originalVersion); err != nil {
		return domain.RuntimeImage{}, err
	}
	return image, nil
}
