package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/agentlifecycle/domain"

	"github.com/google/uuid"
)

type Clock interface{ Now() time.Time }
type IDGenerator interface{ NewID() string }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

type DraftValidator interface {
	Validate(context.Context, domain.Agent, domain.Draft) (map[string]string, error)
}

type ReleaseDependencyResolver interface {
	PrepareRelease(context.Context, domain.Agent, domain.Draft) (domain.ReleaseDependencies, map[string]string, error)
}

type Service struct {
	repository          domain.Repository
	validator           DraftValidator
	releaseDependencies ReleaseDependencyResolver
	clock               Clock
	ids                 IDGenerator
}

func New(repository domain.Repository, validator DraftValidator, releaseDependencies ReleaseDependencyResolver) *Service {
	return NewWithDependencies(repository, validator, releaseDependencies, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, validator DraftValidator, releaseDependencies ReleaseDependencyResolver, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, validator: validator, releaseDependencies: releaseDependencies, clock: clock, ids: ids}
}

type CreateAgentCommand struct {
	OrganizationID string
	TeamID         string
	Name           string
	Description    string
	CreatedBy      string
}

func (service *Service) CreateAgent(ctx context.Context, command CreateAgentCommand) (domain.Agent, error) {
	if err := service.dependencies(); err != nil {
		return domain.Agent{}, err
	}
	agent, err := domain.RegisterAgent(domain.AgentRegistration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, TeamID: command.TeamID,
		Name: command.Name, Description: command.Description, CreatedBy: command.CreatedBy, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.Agent{}, err
	}
	if err := service.repository.CreateAgent(ctx, agent); err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

func (service *Service) GetAgent(ctx context.Context, organizationID, teamID, id string) (domain.Agent, error) {
	if service.repository == nil || organizationID == "" || teamID == "" || id == "" {
		return domain.Agent{}, fmt.Errorf("Agent Repository, Organization, Team, and ID are required")
	}
	return service.repository.GetAgent(ctx, organizationID, teamID, id)
}

func (service *Service) ListAgents(ctx context.Context, organizationID, teamID string) ([]domain.Agent, error) {
	if service.repository == nil || organizationID == "" || teamID == "" {
		return nil, fmt.Errorf("Agent Repository, Organization, and Team are required")
	}
	return service.repository.ListAgents(ctx, organizationID, teamID)
}

type UpdateAgentCommand struct {
	OrganizationID, TeamID, ID string
	Name, Description          string
	ExpectedVersion            int64
}

func (service *Service) UpdateAgent(ctx context.Context, command UpdateAgentCommand) (domain.Agent, error) {
	agent, err := service.GetAgent(ctx, command.OrganizationID, command.TeamID, command.ID)
	if err != nil {
		return domain.Agent{}, err
	}
	if command.ExpectedVersion <= 0 || agent.Version != command.ExpectedVersion {
		return domain.Agent{}, domain.ErrConcurrentUpdate
	}
	if err := agent.Rename(command.Name, command.Description, service.clock.Now()); err != nil {
		return domain.Agent{}, err
	}
	if err := service.repository.UpdateAgent(ctx, agent, command.ExpectedVersion); err != nil {
		return domain.Agent{}, err
	}
	return agent, nil
}

type CreateDraftCommand struct {
	OrganizationID, TeamID, AgentID, CreatedBy string
	Configuration                              domain.Configuration
	ReleaseRisk                                domain.ReleaseRisk
}

func (service *Service) CreateDraft(ctx context.Context, command CreateDraftCommand) (domain.Draft, error) {
	if err := service.dependencies(); err != nil {
		return domain.Draft{}, err
	}
	if _, err := service.GetAgent(ctx, command.OrganizationID, command.TeamID, command.AgentID); err != nil {
		return domain.Draft{}, err
	}
	return service.repository.CreateDraft(ctx, domain.DraftRegistration{
		ID: service.ids.NewID(), AgentID: command.AgentID, Configuration: command.Configuration,
		ReleaseRisk: command.ReleaseRisk, CreatedBy: command.CreatedBy, Now: service.clock.Now(),
	})
}

func (service *Service) GetDraft(ctx context.Context, organizationID, teamID, agentID, draftID string) (domain.Draft, error) {
	if _, err := service.GetAgent(ctx, organizationID, teamID, agentID); err != nil {
		return domain.Draft{}, err
	}
	return service.repository.GetDraft(ctx, agentID, draftID)
}

func (service *Service) ListDrafts(ctx context.Context, organizationID, teamID, agentID string) ([]domain.Draft, error) {
	if _, err := service.GetAgent(ctx, organizationID, teamID, agentID); err != nil {
		return nil, err
	}
	return service.repository.ListDrafts(ctx, agentID)
}

type EditDraftCommand struct {
	OrganizationID, TeamID, AgentID, DraftID string
	Configuration                            domain.Configuration
	ReleaseRisk                              domain.ReleaseRisk
	ExpectedVersion                          int64
}

func (service *Service) EditDraft(ctx context.Context, command EditDraftCommand) (domain.Draft, error) {
	draft, err := service.GetDraft(ctx, command.OrganizationID, command.TeamID, command.AgentID, command.DraftID)
	if err != nil {
		return domain.Draft{}, err
	}
	if command.ExpectedVersion <= 0 || draft.Version != command.ExpectedVersion {
		return domain.Draft{}, domain.ErrConcurrentUpdate
	}
	if err := draft.Edit(command.Configuration, command.ReleaseRisk, service.clock.Now()); err != nil {
		return domain.Draft{}, err
	}
	if err := service.repository.UpdateDraft(ctx, draft, command.ExpectedVersion); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func (service *Service) ValidateDraft(ctx context.Context, organizationID, teamID, agentID, draftID string, expectedVersion int64) (domain.Draft, error) {
	if service.validator == nil {
		return domain.Draft{}, fmt.Errorf("Draft Validator is required")
	}
	agent, err := service.GetAgent(ctx, organizationID, teamID, agentID)
	if err != nil {
		return domain.Draft{}, err
	}
	draft, err := service.repository.GetDraft(ctx, agentID, draftID)
	if err != nil {
		return domain.Draft{}, err
	}
	if expectedVersion <= 0 || draft.Version != expectedVersion {
		return domain.Draft{}, domain.ErrConcurrentUpdate
	}
	now := service.clock.Now()
	if err := draft.StartValidation(now); err != nil {
		return domain.Draft{}, err
	}
	errorsByField, err := service.validator.Validate(ctx, agent, draft)
	if err != nil {
		return domain.Draft{}, fmt.Errorf("validate Agent Draft dependencies: %w", err)
	}
	if err := draft.FinishValidation(domain.ValidationReport{Valid: len(errorsByField) == 0, Errors: errorsByField, CheckedAt: now}, now); err != nil {
		return domain.Draft{}, err
	}
	if err := service.repository.UpdateDraft(ctx, draft, expectedVersion); err != nil {
		return domain.Draft{}, err
	}
	return draft, nil
}

func (service *Service) RequestApproval(ctx context.Context, organizationID, teamID, agentID, draftID, requestedBy, riskReason string) (domain.ReleaseApproval, error) {
	draft, err := service.GetDraft(ctx, organizationID, teamID, agentID, draftID)
	if err != nil {
		return domain.ReleaseApproval{}, err
	}
	if draft.ReleaseRisk != domain.ReleaseRiskHigh || draft.State != domain.DraftStateReady {
		return domain.ReleaseApproval{}, domain.ErrApprovalRequired
	}
	approval, err := domain.RequestReleaseApproval(service.ids.NewID(), draft.ID, draft.Version, requestedBy, riskReason, service.clock.Now())
	if err != nil {
		return domain.ReleaseApproval{}, err
	}
	if err := service.repository.CreateApproval(ctx, approval); err != nil {
		return domain.ReleaseApproval{}, err
	}
	return approval, nil
}

func (service *Service) GetApproval(ctx context.Context, organizationID, teamID, agentID, draftID string) (domain.ReleaseApproval, error) {
	if _, err := service.GetDraft(ctx, organizationID, teamID, agentID, draftID); err != nil {
		return domain.ReleaseApproval{}, err
	}
	return service.repository.GetApprovalByDraft(ctx, draftID)
}

func (service *Service) DecideApproval(ctx context.Context, organizationID, teamID, agentID, draftID string, expectedVersion int64, approved bool, decidedBy, reason string) (domain.ReleaseApproval, error) {
	draft, err := service.GetDraft(ctx, organizationID, teamID, agentID, draftID)
	if err != nil {
		return domain.ReleaseApproval{}, err
	}
	approval, err := service.repository.GetApprovalByDraft(ctx, draftID)
	if err != nil {
		return domain.ReleaseApproval{}, err
	}
	if expectedVersion <= 0 || approval.Version != expectedVersion {
		return domain.ReleaseApproval{}, domain.ErrConcurrentUpdate
	}
	if approval.DraftVersion != draft.Version {
		return domain.ReleaseApproval{}, domain.ErrApprovalRequired
	}
	if err := approval.Decide(approved, decidedBy, reason, service.clock.Now()); err != nil {
		return domain.ReleaseApproval{}, err
	}
	if err := service.repository.UpdateApproval(ctx, approval, expectedVersion); err != nil {
		return domain.ReleaseApproval{}, err
	}
	return approval, nil
}

func (service *Service) Publish(ctx context.Context, organizationID, teamID, agentID, draftID, releasedBy string) (domain.Release, error) {
	if service.releaseDependencies == nil {
		return domain.Release{}, fmt.Errorf("Agent Release Dependency Resolver is required")
	}
	agent, err := service.GetAgent(ctx, organizationID, teamID, agentID)
	if err != nil {
		return domain.Release{}, err
	}
	draft, err := service.repository.GetDraft(ctx, agentID, draftID)
	if err != nil {
		return domain.Release{}, err
	}
	if err := draft.CanRelease(); err != nil {
		return domain.Release{}, err
	}
	dependencies, errorsByField, err := service.releaseDependencies.PrepareRelease(ctx, agent, draft)
	if err != nil {
		return domain.Release{}, fmt.Errorf("revalidate Agent Draft dependencies: %w", err)
	}
	if len(errorsByField) != 0 {
		return domain.Release{}, fmt.Errorf("%w: dependencies changed after validation", domain.ErrDraftNotReady)
	}
	var approval *domain.RiskApproval
	if draft.ReleaseRisk == domain.ReleaseRiskHigh {
		stored, err := service.repository.GetApprovalByDraft(ctx, draft.ID)
		if err != nil {
			if errors.Is(err, domain.ErrApprovalNotFound) {
				return domain.Release{}, domain.ErrApprovalRequired
			}
			return domain.Release{}, err
		}
		approval = stored.ApprovedRiskApproval()
	}
	return service.repository.CreateRelease(ctx, domain.ReleaseRegistration{
		ID: service.ids.NewID(), Draft: draft, ReleasedBy: releasedBy, Approval: approval, Dependencies: dependencies, Now: service.clock.Now(),
	})
}

func (service *Service) GetRelease(ctx context.Context, organizationID, teamID, agentID, releaseID string) (domain.Release, error) {
	if _, err := service.GetAgent(ctx, organizationID, teamID, agentID); err != nil {
		return domain.Release{}, err
	}
	return service.repository.GetRelease(ctx, agentID, releaseID)
}

func (service *Service) ListReleases(ctx context.Context, organizationID, teamID, agentID string) ([]domain.Release, error) {
	if _, err := service.GetAgent(ctx, organizationID, teamID, agentID); err != nil {
		return nil, err
	}
	return service.repository.ListReleases(ctx, agentID)
}

func (service *Service) DeprecateRelease(ctx context.Context, organizationID, teamID, agentID, releaseID string, expectedVersion int64) (domain.Release, error) {
	release, err := service.GetRelease(ctx, organizationID, teamID, agentID, releaseID)
	if err != nil {
		return domain.Release{}, err
	}
	if expectedVersion <= 0 || release.Version != expectedVersion {
		return domain.Release{}, domain.ErrConcurrentUpdate
	}
	expected := release.Version
	if err := release.Deprecate(service.clock.Now()); err != nil {
		return domain.Release{}, err
	}
	if err := service.repository.UpdateReleaseStatus(ctx, release, expected); err != nil {
		return domain.Release{}, err
	}
	return release, nil
}

func (service *Service) BlockRelease(ctx context.Context, organizationID, teamID, agentID, releaseID string, expectedVersion int64, reason string) (domain.Release, error) {
	release, err := service.GetRelease(ctx, organizationID, teamID, agentID, releaseID)
	if err != nil {
		return domain.Release{}, err
	}
	if expectedVersion <= 0 || release.Version != expectedVersion {
		return domain.Release{}, domain.ErrConcurrentUpdate
	}
	expected := release.Version
	if err := release.Block(reason); err != nil {
		return domain.Release{}, err
	}
	if err := service.repository.UpdateReleaseStatus(ctx, release, expected); err != nil {
		return domain.Release{}, err
	}
	return release, nil
}

func (service *Service) dependencies() error {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return fmt.Errorf("Agent Lifecycle Repository, Clock, and ID Generator are required")
	}
	return nil
}

func validID(value string) bool { return strings.TrimSpace(value) != "" }
