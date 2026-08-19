package application

import (
	"context"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"
)

type BindingValidator interface {
	CheckReferences(context.Context, domain.RepositoryBinding) error
	Validate(context.Context, domain.RepositoryBinding) (map[string]string, error)
}

type BindingService struct {
	repository domain.BindingRepository
	validator  BindingValidator
	clock      Clock
	ids        IDGenerator
}

type RegisterBindingCommand struct {
	OrganizationID              string
	TeamID                      string
	SourceControlProviderID     string
	Name                        string
	RepositorySSHURL            string
	DefaultBranch               string
	SSHCredentialProfileID      string
	BuildCredentialProfileIDs   []string
	GitAuthorName               string
	GitAuthorEmail              string
	AllowedRuntimeImageIDs      []string
	DefaultRuntimeImageID       string
	RequiredRuntimeCapabilities []string
	DefaultModelID              string
	ModelBudget                 domain.ModelBudget
	Instructions                string
	QualityCommands             []domain.QualityCommand
	EgressPolicy                domain.EgressPolicy
}

type UpdateBindingCommand struct {
	ID              string
	ExpectedVersion int64
	RegisterBindingCommand
}

func NewBindingService(repository domain.BindingRepository, validator BindingValidator) *BindingService {
	return NewBindingServiceWithDependencies(repository, validator, systemClock{}, uuidGenerator{})
}

func NewBindingServiceWithDependencies(repository domain.BindingRepository, validator BindingValidator, clock Clock, ids IDGenerator) *BindingService {
	return &BindingService{repository: repository, validator: validator, clock: clock, ids: ids}
}

func (service *BindingService) Register(ctx context.Context, command RegisterBindingCommand) (domain.RepositoryBinding, error) {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return domain.RepositoryBinding{}, fmt.Errorf("Repository Binding dependencies are required")
	}
	binding, err := domain.RegisterBinding(domain.BindingRegistration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, TeamID: command.TeamID,
		SourceControlProviderID: command.SourceControlProviderID, Name: command.Name,
		RepositorySSHURL: command.RepositorySSHURL, DefaultBranch: command.DefaultBranch,
		SSHCredentialProfileID: command.SSHCredentialProfileID, BuildCredentialProfileIDs: command.BuildCredentialProfileIDs,
		GitAuthorName: command.GitAuthorName, GitAuthorEmail: command.GitAuthorEmail,
		AllowedRuntimeImageIDs: command.AllowedRuntimeImageIDs, DefaultRuntimeImageID: command.DefaultRuntimeImageID,
		RequiredRuntimeCapabilities: command.RequiredRuntimeCapabilities,
		DefaultModelID:              command.DefaultModelID, ModelBudget: command.ModelBudget, Instructions: command.Instructions,
		QualityCommands: command.QualityCommands, EgressPolicy: command.EgressPolicy, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.RepositoryBinding{}, err
	}
	if service.validator == nil {
		return domain.RepositoryBinding{}, fmt.Errorf("Repository Binding Validator is required")
	}
	if err := service.validator.CheckReferences(ctx, binding); err != nil {
		return domain.RepositoryBinding{}, err
	}
	if err := service.repository.CreateBinding(ctx, binding); err != nil {
		return domain.RepositoryBinding{}, err
	}
	return binding, nil
}

func (service *BindingService) Get(ctx context.Context, organizationID, teamID, id string) (domain.RepositoryBinding, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(teamID) == "" || strings.TrimSpace(id) == "" {
		return domain.RepositoryBinding{}, fmt.Errorf("Repository Binding Repository, Organization, Team, and ID are required")
	}
	return service.repository.GetBinding(ctx, organizationID, teamID, id)
}

func (service *BindingService) List(ctx context.Context, organizationID, teamID string) ([]domain.RepositoryBinding, error) {
	if service.repository == nil || strings.TrimSpace(organizationID) == "" || strings.TrimSpace(teamID) == "" {
		return nil, fmt.Errorf("Repository Binding Repository, Organization, and Team are required")
	}
	return service.repository.ListBindings(ctx, organizationID, teamID)
}

func (service *BindingService) Validate(ctx context.Context, organizationID, teamID, id string, expectedVersion int64) (domain.RepositoryBinding, error) {
	if service.repository == nil || service.validator == nil || service.clock == nil || expectedVersion <= 0 {
		return domain.RepositoryBinding{}, fmt.Errorf("Repository Binding validation dependencies and expected Version are required")
	}
	binding, err := service.Get(ctx, organizationID, teamID, id)
	if err != nil {
		return domain.RepositoryBinding{}, err
	}
	if binding.Version != expectedVersion {
		return domain.RepositoryBinding{}, domain.ErrBindingConcurrentUpdate
	}
	errorsByField, err := service.validator.Validate(ctx, binding)
	if err != nil {
		return domain.RepositoryBinding{}, fmt.Errorf("validate Repository Binding dependencies: %w", err)
	}
	report := domain.ValidationReport{Valid: len(errorsByField) == 0, Errors: errorsByField, CheckedAt: service.clock.Now()}
	if err := binding.RecordValidation(report); err != nil {
		return domain.RepositoryBinding{}, err
	}
	if err := service.repository.UpdateBindingValidation(ctx, binding, expectedVersion); err != nil {
		return domain.RepositoryBinding{}, err
	}
	return binding, nil
}

func (service *BindingService) Update(ctx context.Context, command UpdateBindingCommand) (domain.RepositoryBinding, error) {
	if command.ExpectedVersion <= 0 || service.validator == nil || service.clock == nil {
		return domain.RepositoryBinding{}, fmt.Errorf("Repository Binding Validator, Clock, and expected Version are required")
	}
	binding, err := service.Get(ctx, command.OrganizationID, command.TeamID, command.ID)
	if err != nil {
		return domain.RepositoryBinding{}, err
	}
	if binding.Version != command.ExpectedVersion {
		return domain.RepositoryBinding{}, domain.ErrBindingConcurrentUpdate
	}
	input := command.RegisterBindingCommand
	if err := binding.Reconfigure(domain.BindingRegistration{
		SourceControlProviderID: input.SourceControlProviderID, Name: input.Name,
		RepositorySSHURL: input.RepositorySSHURL, DefaultBranch: input.DefaultBranch,
		SSHCredentialProfileID: input.SSHCredentialProfileID, BuildCredentialProfileIDs: input.BuildCredentialProfileIDs,
		GitAuthorName: input.GitAuthorName, GitAuthorEmail: input.GitAuthorEmail,
		AllowedRuntimeImageIDs: input.AllowedRuntimeImageIDs, DefaultRuntimeImageID: input.DefaultRuntimeImageID,
		RequiredRuntimeCapabilities: input.RequiredRuntimeCapabilities,
		DefaultModelID:              input.DefaultModelID, ModelBudget: input.ModelBudget, Instructions: input.Instructions,
		QualityCommands: input.QualityCommands, EgressPolicy: input.EgressPolicy, Now: service.clock.Now(),
	}); err != nil {
		return domain.RepositoryBinding{}, err
	}
	if err := service.validator.CheckReferences(ctx, binding); err != nil {
		return domain.RepositoryBinding{}, err
	}
	if err := service.repository.UpdateBinding(ctx, binding, command.ExpectedVersion); err != nil {
		return domain.RepositoryBinding{}, err
	}
	return binding, nil
}
