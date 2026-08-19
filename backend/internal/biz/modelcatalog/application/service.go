package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/modelcatalog/domain"

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

type RegisterCredentialCommand struct {
	OrganizationID string
	TeamID         *string
	Name           string
	Kind           domain.CredentialKind
	SecretRef      string
}

type RegisterModelCommand struct {
	OrganizationID      string
	Name                string
	ModelID             string
	Endpoint            string
	CredentialProfileID string
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

func (service *Service) RegisterCredential(ctx context.Context, command RegisterCredentialCommand) (domain.CredentialProfile, error) {
	if err := service.requireDependencies(); err != nil {
		return domain.CredentialProfile{}, err
	}
	profile, err := domain.RegisterCredential(domain.CredentialRegistration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, TeamID: command.TeamID,
		Name: command.Name, Kind: command.Kind, SecretRef: command.SecretRef, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.CredentialProfile{}, err
	}
	if err := service.repository.CreateCredential(ctx, profile); err != nil {
		return domain.CredentialProfile{}, err
	}
	return profile, nil
}

func (service *Service) RegisterModel(ctx context.Context, command RegisterModelCommand) (domain.ConfiguredModel, error) {
	if err := service.requireDependencies(); err != nil {
		return domain.ConfiguredModel{}, err
	}
	credential, err := service.repository.GetCredential(ctx, command.OrganizationID, command.CredentialProfileID)
	if err != nil {
		return domain.ConfiguredModel{}, err
	}
	model, err := domain.RegisterModel(domain.ModelRegistration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, Name: command.Name,
		ModelID: command.ModelID, Endpoint: command.Endpoint, Credential: credential, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.ConfiguredModel{}, err
	}
	if err := service.repository.CreateModel(ctx, model); err != nil {
		return domain.ConfiguredModel{}, err
	}
	return model, nil
}

func (service *Service) GetCredential(ctx context.Context, organizationID, id string) (domain.CredentialProfile, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(id) == "" {
		return domain.CredentialProfile{}, fmt.Errorf("Credential Profile Repository, Organization ID, and ID are required")
	}
	return service.repository.GetCredential(ctx, organizationID, id)
}

func (service *Service) ListCredentials(ctx context.Context, organizationID string) ([]domain.CredentialProfile, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("Credential Profile Repository and Organization ID are required")
	}
	return service.repository.ListCredentials(ctx, organizationID)
}

func (service *Service) GetModel(ctx context.Context, organizationID, id string) (domain.ConfiguredModel, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(id) == "" {
		return domain.ConfiguredModel{}, fmt.Errorf("Configured Model Repository, Organization ID, and ID are required")
	}
	return service.repository.GetModel(ctx, organizationID, id)
}

func (service *Service) ListModels(ctx context.Context, organizationID string) ([]domain.ConfiguredModel, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("Configured Model Repository and Organization ID are required")
	}
	return service.repository.ListModels(ctx, organizationID)
}

func (service *Service) ChangeCredentialStatus(ctx context.Context, command ChangeStatusCommand) (domain.CredentialProfile, error) {
	if service.repository == nil || service.clock == nil || command.ExpectedVersion <= 0 {
		return domain.CredentialProfile{}, fmt.Errorf("Credential Profile dependencies and expected version are required")
	}
	profile, err := service.repository.GetCredential(ctx, command.OrganizationID, command.ID)
	if err != nil {
		return domain.CredentialProfile{}, err
	}
	if profile.Version != command.ExpectedVersion {
		return domain.CredentialProfile{}, domain.ErrConcurrentUpdate
	}
	originalVersion := profile.Version
	if err := profile.SetEnabled(command.Enabled, service.clock.Now()); err != nil {
		return domain.CredentialProfile{}, err
	}
	if profile.Version != originalVersion {
		if err := service.repository.UpdateCredentialStatus(ctx, profile, originalVersion); err != nil {
			return domain.CredentialProfile{}, err
		}
	}
	return profile, nil
}

func (service *Service) ChangeModelStatus(ctx context.Context, command ChangeStatusCommand) (domain.ConfiguredModel, error) {
	if service.repository == nil || service.clock == nil || command.ExpectedVersion <= 0 {
		return domain.ConfiguredModel{}, fmt.Errorf("Configured Model dependencies and expected version are required")
	}
	model, err := service.repository.GetModel(ctx, command.OrganizationID, command.ID)
	if err != nil {
		return domain.ConfiguredModel{}, err
	}
	if model.Version != command.ExpectedVersion {
		return domain.ConfiguredModel{}, domain.ErrConcurrentUpdate
	}
	originalVersion := model.Version
	credential := domain.CredentialProfile{}
	if command.Enabled && !model.Enabled {
		credential, err = service.repository.GetCredential(ctx, command.OrganizationID, model.CredentialProfileID)
		if err != nil {
			return domain.ConfiguredModel{}, err
		}
	}
	if err := model.SetEnabled(command.Enabled, credential, service.clock.Now()); err != nil {
		return domain.ConfiguredModel{}, err
	}
	if model.Version != originalVersion {
		if err := service.repository.UpdateModelStatus(ctx, model, originalVersion); err != nil {
			return domain.ConfiguredModel{}, err
		}
	}
	return model, nil
}

func (service *Service) requireDependencies() error {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return fmt.Errorf("Model Catalog dependencies are required")
	}
	return nil
}
