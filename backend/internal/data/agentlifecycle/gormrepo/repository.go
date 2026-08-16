package gormrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/agentlifecycle/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

var _ domain.Repository = (*Repository)(nil)

type jsonValue []byte

func (value jsonValue) Value() (driver.Value, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return string(value), nil
}
func (value *jsonValue) Scan(source any) error {
	switch typed := source.(type) {
	case nil:
		*value = nil
	case []byte:
		*value = append((*value)[:0], typed...)
	case string:
		*value = append((*value)[:0], typed...)
	default:
		return fmt.Errorf("scan Agent Lifecycle JSON from %T", source)
	}
	return nil
}
func (jsonValue) GormDataType() string                          { return "json" }
func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type agentRecord struct {
	ID, OrganizationID, TeamID, Name, Description, CreatedBy string
	CreatedAt, UpdatedAt                                     time.Time
	Version                                                  int64
}

func (agentRecord) TableName() string { return "agents" }

type draftRecord struct {
	ID, AgentID, CreatedBy string
	Revision               int64
	State                  domain.DraftState
	Configuration          jsonValue `gorm:"type:jsonb"`
	ReleaseRisk            domain.ReleaseRisk
	ValidationReport       jsonValue `gorm:"type:jsonb"`
	CreatedAt, UpdatedAt   time.Time
	Version                int64
}

func (draftRecord) TableName() string { return "agent_drafts" }

type approvalRecord struct {
	ID, DraftID, RequestedBy string
	DraftVersion             int64
	State                    domain.ApprovalState
	RequestedAt              time.Time
	DecidedBy                *string
	DecidedAt                *time.Time
	Reason                   string
	Version                  int64
}

func (approvalRecord) TableName() string { return "agent_release_approvals" }

type releaseRecord struct {
	ID, AgentID, SourceDraftID, RuntimeImageID, ConfiguredModelID, RepositoryBindingID string
	ReleaseNumber                                                                      int64
	ConfigurationSnapshot                                                              jsonValue `gorm:"type:jsonb"`
	ModelBudget                                                                        jsonValue `gorm:"type:jsonb"`
	ExecutionLimits                                                                    jsonValue `gorm:"type:jsonb"`
	Status                                                                             domain.ReleaseStatus
	ReleasedBy                                                                         string
	ReleasedAt                                                                         time.Time
	DeprecatedAt                                                                       *time.Time
	Version                                                                            int64
}

func (releaseRecord) TableName() string { return "agent_releases" }

func (repository *Repository) CreateAgent(ctx context.Context, agent domain.Agent) error {
	record := agentRecord{ID: agent.ID, OrganizationID: agent.OrganizationID, TeamID: agent.TeamID, Name: agent.Name, Description: agent.Description, CreatedBy: agent.CreatedBy, CreatedAt: agent.CreatedAt, UpdatedAt: agent.UpdatedAt, Version: agent.Version}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAgentNameExists
		}
		return fmt.Errorf("create Agent: %w", err)
	}
	return nil
}

func (repository *Repository) GetAgent(ctx context.Context, organizationID, teamID, id string) (domain.Agent, error) {
	var record agentRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ? AND id = ?", organizationID, teamID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Agent{}, domain.ErrAgentNotFound
		}
		return domain.Agent{}, fmt.Errorf("load Agent: %w", err)
	}
	return restoreAgent(record)
}

func (repository *Repository) ListAgents(ctx context.Context, organizationID, teamID string) ([]domain.Agent, error) {
	var records []agentRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ?", organizationID, teamID).Order("name, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Agents: %w", err)
	}
	result := make([]domain.Agent, 0, len(records))
	for _, record := range records {
		value, err := restoreAgent(record)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (repository *Repository) UpdateAgent(ctx context.Context, agent domain.Agent, expectedVersion int64) error {
	result := repository.db.WithContext(ctx).Model(&agentRecord{}).Where("organization_id = ? AND team_id = ? AND id = ? AND version = ?", agent.OrganizationID, agent.TeamID, agent.ID, expectedVersion).Updates(map[string]any{"name": agent.Name, "description": agent.Description, "updated_at": agent.UpdatedAt, "version": agent.Version})
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.ErrAgentNameExists
		}
		return fmt.Errorf("update Agent: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func (repository *Repository) CreateDraft(ctx context.Context, registration domain.DraftRegistration) (domain.Draft, error) {
	var created domain.Draft
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var agent agentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", registration.AgentID).Take(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrAgentNotFound
			}
			return err
		}
		var revision int64
		if err := tx.Model(&draftRecord{}).Where("agent_id = ?", registration.AgentID).Select("COALESCE(MAX(revision), 0)").Scan(&revision).Error; err != nil {
			return err
		}
		registration.Revision = revision + 1
		draft, err := domain.CreateDraft(registration)
		if err != nil {
			return err
		}
		record, err := draftToRecord(draft)
		if err != nil {
			return err
		}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create Agent Draft: %w", err)
		}
		created = draft
		return nil
	})
	return created, err
}

func (repository *Repository) GetDraft(ctx context.Context, agentID, id string) (domain.Draft, error) {
	var record draftRecord
	if err := repository.db.WithContext(ctx).Where("agent_id = ? AND id = ?", agentID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Draft{}, domain.ErrDraftNotFound
		}
		return domain.Draft{}, fmt.Errorf("load Agent Draft: %w", err)
	}
	return restoreDraft(record)
}

func (repository *Repository) ListDrafts(ctx context.Context, agentID string) ([]domain.Draft, error) {
	var records []draftRecord
	if err := repository.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("revision DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Agent Drafts: %w", err)
	}
	result := make([]domain.Draft, 0, len(records))
	for _, record := range records {
		value, err := restoreDraft(record)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (repository *Repository) UpdateDraft(ctx context.Context, draft domain.Draft, expectedVersion int64) error {
	configuration, err := json.Marshal(draft.Configuration)
	if err != nil {
		return err
	}
	var report any
	if draft.ValidationReport != nil {
		encoded, err := json.Marshal(draft.ValidationReport)
		if err != nil {
			return err
		}
		report = jsonValue(encoded)
	}
	result := repository.db.WithContext(ctx).Model(&draftRecord{}).Where("agent_id = ? AND id = ? AND version = ?", draft.AgentID, draft.ID, expectedVersion).Updates(map[string]any{"state": draft.State, "configuration": jsonValue(configuration), "release_risk": draft.ReleaseRisk, "validation_report": report, "updated_at": draft.UpdatedAt, "version": draft.Version})
	if result.Error != nil {
		return fmt.Errorf("update Agent Draft: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func (repository *Repository) CreateApproval(ctx context.Context, approval domain.ReleaseApproval) error {
	record := approvalRecord{ID: approval.ID, DraftID: approval.DraftID, DraftVersion: approval.DraftVersion, RequestedBy: approval.RequestedBy, State: approval.State, RequestedAt: approval.RequestedAt, Reason: approval.Reason, Version: approval.Version}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrApprovalExists
		}
		return fmt.Errorf("create Agent Release Approval: %w", err)
	}
	return nil
}

func (repository *Repository) GetApprovalByDraft(ctx context.Context, draftID string) (domain.ReleaseApproval, error) {
	var record approvalRecord
	if err := repository.db.WithContext(ctx).Where("draft_id = ?", draftID).Order("draft_version DESC").Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ReleaseApproval{}, domain.ErrApprovalNotFound
		}
		return domain.ReleaseApproval{}, fmt.Errorf("load Agent Release Approval: %w", err)
	}
	return restoreApproval(record)
}

func (repository *Repository) UpdateApproval(ctx context.Context, approval domain.ReleaseApproval, expectedVersion int64) error {
	var decidedBy any
	if approval.DecidedBy != "" {
		decidedBy = approval.DecidedBy
	}
	result := repository.db.WithContext(ctx).Model(&approvalRecord{}).Where("id = ? AND version = ?", approval.ID, expectedVersion).Updates(map[string]any{"state": approval.State, "decided_by": decidedBy, "decided_at": approval.DecidedAt, "reason": approval.Reason, "version": approval.Version})
	if result.Error != nil {
		return fmt.Errorf("update Agent Release Approval: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func (repository *Repository) CreateRelease(ctx context.Context, registration domain.ReleaseRegistration) (domain.Release, error) {
	var created domain.Release
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var draftRecord draftRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", registration.Draft.ID).Take(&draftRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrDraftNotFound
			}
			return err
		}
		draft, err := restoreDraft(draftRecord)
		if err != nil {
			return err
		}
		registration.Draft = draft
		var agent agentRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", draft.AgentID).Take(&agent).Error; err != nil {
			return fmt.Errorf("lock Agent for Release numbering: %w", err)
		}
		if draft.ReleaseRisk == domain.ReleaseRiskHigh {
			var record approvalRecord
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("draft_id = ?", draft.ID).Take(&record).Error; err != nil {
				return domain.ErrApprovalRequired
			}
			approval, err := restoreApproval(record)
			if err != nil {
				return err
			}
			registration.Approval = approval.ApprovedRiskApproval()
		}
		var number int64
		if err := tx.Model(&releaseRecord{}).Where("agent_id = ?", draft.AgentID).Select("COALESCE(MAX(release_number), 0)").Scan(&number).Error; err != nil {
			return err
		}
		registration.ReleaseNumber = number + 1
		release, err := domain.Publish(registration)
		if err != nil {
			return err
		}
		record, err := releaseToRecord(release)
		if err != nil {
			return err
		}
		if err := tx.Create(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrDraftAlreadyReleased
			}
			return fmt.Errorf("create Agent Release: %w", err)
		}
		created = release
		return nil
	})
	return created, err
}

func (repository *Repository) GetRelease(ctx context.Context, agentID, id string) (domain.Release, error) {
	var record releaseRecord
	if err := repository.db.WithContext(ctx).Where("agent_id = ? AND id = ?", agentID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Release{}, domain.ErrReleaseNotFound
		}
		return domain.Release{}, fmt.Errorf("load Agent Release: %w", err)
	}
	return restoreRelease(record)
}

func (repository *Repository) ListReleases(ctx context.Context, agentID string) ([]domain.Release, error) {
	var records []releaseRecord
	if err := repository.db.WithContext(ctx).Where("agent_id = ?", agentID).Order("release_number DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Agent Releases: %w", err)
	}
	result := make([]domain.Release, 0, len(records))
	for _, record := range records {
		value, err := restoreRelease(record)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (repository *Repository) UpdateReleaseStatus(ctx context.Context, release domain.Release, expectedVersion int64) error {
	result := repository.db.WithContext(ctx).Model(&releaseRecord{}).Where("id = ? AND version = ?", release.ID, expectedVersion).Updates(map[string]any{"status": release.Status, "deprecated_at": release.DeprecatedAt, "version": release.Version})
	if result.Error != nil {
		return fmt.Errorf("update Agent Release status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func restoreAgent(record agentRecord) (domain.Agent, error) {
	return domain.RestoreAgent(domain.AgentRegistration{ID: record.ID, OrganizationID: record.OrganizationID, TeamID: record.TeamID, Name: record.Name, Description: record.Description, CreatedBy: record.CreatedBy}, record.CreatedAt, record.UpdatedAt, record.Version)
}

func draftToRecord(draft domain.Draft) (draftRecord, error) {
	configuration, err := json.Marshal(draft.Configuration)
	if err != nil {
		return draftRecord{}, err
	}
	return draftRecord{ID: draft.ID, AgentID: draft.AgentID, Revision: draft.Revision, State: draft.State, Configuration: configuration, ReleaseRisk: draft.ReleaseRisk, CreatedBy: draft.CreatedBy, CreatedAt: draft.CreatedAt, UpdatedAt: draft.UpdatedAt, Version: draft.Version}, nil
}

func restoreDraft(record draftRecord) (domain.Draft, error) {
	var configuration domain.Configuration
	if err := json.Unmarshal(record.Configuration, &configuration); err != nil {
		return domain.Draft{}, err
	}
	var report *domain.ValidationReport
	if len(record.ValidationReport) > 0 {
		report = &domain.ValidationReport{}
		if err := json.Unmarshal(record.ValidationReport, report); err != nil {
			return domain.Draft{}, err
		}
	}
	return domain.RestoreDraft(domain.DraftRegistration{ID: record.ID, AgentID: record.AgentID, Revision: record.Revision, Configuration: configuration, ReleaseRisk: record.ReleaseRisk, CreatedBy: record.CreatedBy}, record.State, report, record.CreatedAt, record.UpdatedAt, record.Version)
}

func restoreApproval(record approvalRecord) (domain.ReleaseApproval, error) {
	approval := domain.ReleaseApproval{ID: record.ID, DraftID: record.DraftID, DraftVersion: record.DraftVersion, RequestedBy: record.RequestedBy, State: record.State, RequestedAt: record.RequestedAt, DecidedAt: record.DecidedAt, Reason: record.Reason, Version: record.Version}
	if record.DecidedBy != nil {
		approval.DecidedBy = *record.DecidedBy
	}
	if approval.ID == "" || approval.DraftID == "" || approval.DraftVersion <= 0 || approval.RequestedBy == "" || approval.RequestedAt.IsZero() || approval.Version <= 0 {
		return domain.ReleaseApproval{}, fmt.Errorf("invalid persisted Agent Release Approval")
	}
	return approval, nil
}

func releaseToRecord(release domain.Release) (releaseRecord, error) {
	snapshot, err := json.Marshal(release.Configuration)
	if err != nil {
		return releaseRecord{}, err
	}
	budget, err := json.Marshal(release.Configuration.ModelBudget)
	if err != nil {
		return releaseRecord{}, err
	}
	limits, err := json.Marshal(release.Configuration.ExecutionLimits)
	if err != nil {
		return releaseRecord{}, err
	}
	return releaseRecord{ID: release.ID, AgentID: release.AgentID, ReleaseNumber: release.ReleaseNumber, SourceDraftID: release.SourceDraftID, RuntimeImageID: release.RuntimeImageID, ConfiguredModelID: release.ConfiguredModelID, RepositoryBindingID: release.RepositoryBindingID, ConfigurationSnapshot: snapshot, ModelBudget: budget, ExecutionLimits: limits, Status: release.Status, ReleasedBy: release.ReleasedBy, ReleasedAt: release.ReleasedAt, DeprecatedAt: release.DeprecatedAt, Version: release.Version}, nil
}

func restoreRelease(record releaseRecord) (domain.Release, error) {
	var configuration domain.Configuration
	if err := json.Unmarshal(record.ConfigurationSnapshot, &configuration); err != nil {
		return domain.Release{}, err
	}
	return domain.RestoreRelease(domain.Release{ID: record.ID, AgentID: record.AgentID, ReleaseNumber: record.ReleaseNumber, SourceDraftID: record.SourceDraftID, RuntimeImageID: record.RuntimeImageID, ConfiguredModelID: record.ConfiguredModelID, RepositoryBindingID: record.RepositoryBindingID, Configuration: configuration, Status: record.Status, ReleasedBy: record.ReleasedBy, ReleasedAt: record.ReleasedAt, DeprecatedAt: record.DeprecatedAt, Version: record.Version})
}
