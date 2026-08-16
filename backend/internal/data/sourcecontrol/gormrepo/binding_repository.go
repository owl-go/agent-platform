package gormrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type bindingJSON []byte

func (value bindingJSON) Value() (driver.Value, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return string(value), nil
}
func (value *bindingJSON) Scan(source any) error {
	switch typed := source.(type) {
	case []byte:
		*value = append((*value)[:0], typed...)
	case string:
		*value = append((*value)[:0], typed...)
	case nil:
		*value = nil
	default:
		return fmt.Errorf("scan Repository Binding JSON from %T", source)
	}
	return nil
}
func (bindingJSON) GormDataType() string                          { return "json" }
func (bindingJSON) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type bindingRecord struct {
	ID                        string      `gorm:"column:id;primaryKey"`
	OrganizationID            string      `gorm:"column:organization_id"`
	TeamID                    string      `gorm:"column:team_id"`
	SourceControlProviderID   string      `gorm:"column:source_control_provider_id"`
	Name                      string      `gorm:"column:name"`
	RepositorySSHURL          string      `gorm:"column:repository_ssh_url"`
	DefaultBranch             string      `gorm:"column:default_branch"`
	SSHCredentialProfileID    string      `gorm:"column:ssh_credential_profile_id"`
	BuildCredentialProfileIDs bindingJSON `gorm:"column:build_credential_profile_ids;type:jsonb"`
	GitAuthorName             string      `gorm:"column:git_author_name"`
	GitAuthorEmail            string      `gorm:"column:git_author_email"`
	AllowedRuntimeImageIDs    bindingJSON `gorm:"column:allowed_runtime_image_ids;type:jsonb"`
	DefaultRuntimeImageID     string      `gorm:"column:default_runtime_image_id"`
	DefaultModelID            string      `gorm:"column:default_model_id"`
	ModelBudget               bindingJSON `gorm:"column:model_budget;type:jsonb"`
	Instructions              string      `gorm:"column:instructions"`
	QualityCommands           bindingJSON `gorm:"column:quality_commands;type:jsonb"`
	EgressPolicy              bindingJSON `gorm:"column:egress_policy;type:jsonb"`
	ValidationReport          bindingJSON `gorm:"column:validation_report;type:jsonb"`
	ValidatedAt               *time.Time  `gorm:"column:validated_at"`
	CreatedAt                 time.Time   `gorm:"column:created_at"`
	UpdatedAt                 time.Time   `gorm:"column:updated_at"`
	Version                   int64       `gorm:"column:version"`
}

func (bindingRecord) TableName() string { return "repository_bindings" }

func (repository *Repository) CreateBinding(ctx context.Context, binding domain.RepositoryBinding) error {
	record, err := bindingToRecord(binding)
	if err != nil {
		return err
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrBindingNameExists
		}
		return fmt.Errorf("create Repository Binding: %w", err)
	}
	return nil
}

func (repository *Repository) GetBinding(ctx context.Context, organizationID, teamID, id string) (domain.RepositoryBinding, error) {
	var record bindingRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ? AND id = ?", organizationID, teamID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RepositoryBinding{}, domain.ErrBindingNotFound
		}
		return domain.RepositoryBinding{}, fmt.Errorf("load Repository Binding: %w", err)
	}
	return restoreBindingRecord(record)
}

func (repository *Repository) ListBindings(ctx context.Context, organizationID, teamID string) ([]domain.RepositoryBinding, error) {
	var records []bindingRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ?", organizationID, teamID).Order("name, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Repository Bindings: %w", err)
	}
	bindings := make([]domain.RepositoryBinding, 0, len(records))
	for _, record := range records {
		binding, err := restoreBindingRecord(record)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func (repository *Repository) UpdateBindingValidation(ctx context.Context, binding domain.RepositoryBinding, expectedVersion int64) error {
	report, err := json.Marshal(binding.ValidationReport)
	if err != nil {
		return fmt.Errorf("encode Repository Binding Validation Report: %w", err)
	}
	result := repository.db.WithContext(ctx).Model(&bindingRecord{}).
		Where("organization_id = ? AND team_id = ? AND id = ? AND version = ?", binding.OrganizationID, binding.TeamID, binding.ID, expectedVersion).
		Updates(map[string]any{"validation_report": bindingJSON(report), "validated_at": binding.ValidatedAt, "updated_at": binding.UpdatedAt, "version": binding.Version})
	if result.Error != nil {
		return fmt.Errorf("update Repository Binding validation: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrBindingConcurrentUpdate
	}
	return nil
}

func (repository *Repository) UpdateBinding(ctx context.Context, binding domain.RepositoryBinding, expectedVersion int64) error {
	record, err := bindingToRecord(binding)
	if err != nil {
		return err
	}
	result := repository.db.WithContext(ctx).Model(&bindingRecord{}).
		Where("organization_id = ? AND team_id = ? AND id = ? AND version = ?", binding.OrganizationID, binding.TeamID, binding.ID, expectedVersion).
		Updates(map[string]any{
			"source_control_provider_id": record.SourceControlProviderID, "name": record.Name,
			"repository_ssh_url": record.RepositorySSHURL, "default_branch": record.DefaultBranch,
			"ssh_credential_profile_id": record.SSHCredentialProfileID, "build_credential_profile_ids": record.BuildCredentialProfileIDs,
			"git_author_name": record.GitAuthorName, "git_author_email": record.GitAuthorEmail,
			"allowed_runtime_image_ids": record.AllowedRuntimeImageIDs, "default_runtime_image_id": record.DefaultRuntimeImageID,
			"default_model_id": record.DefaultModelID, "model_budget": record.ModelBudget,
			"instructions": record.Instructions, "quality_commands": record.QualityCommands, "egress_policy": record.EgressPolicy,
			"validation_report": nil, "validated_at": nil, "updated_at": record.UpdatedAt, "version": record.Version,
		})
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.ErrBindingNameExists
		}
		return fmt.Errorf("update Repository Binding: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrBindingConcurrentUpdate
	}
	return nil
}

func bindingToRecord(binding domain.RepositoryBinding) (bindingRecord, error) {
	buildCredentials, err := json.Marshal(binding.BuildCredentialProfileIDs)
	if err != nil {
		return bindingRecord{}, err
	}
	runtimes, err := json.Marshal(binding.AllowedRuntimeImageIDs)
	if err != nil {
		return bindingRecord{}, err
	}
	budget, err := json.Marshal(binding.ModelBudget)
	if err != nil {
		return bindingRecord{}, err
	}
	commands, err := json.Marshal(binding.QualityCommands)
	if err != nil {
		return bindingRecord{}, err
	}
	egress, err := json.Marshal(binding.EgressPolicy)
	if err != nil {
		return bindingRecord{}, err
	}
	return bindingRecord{
		ID: binding.ID, OrganizationID: binding.OrganizationID, TeamID: binding.TeamID,
		SourceControlProviderID: binding.SourceControlProviderID, Name: binding.Name,
		RepositorySSHURL: binding.RepositorySSHURL, DefaultBranch: binding.DefaultBranch,
		SSHCredentialProfileID: binding.SSHCredentialProfileID, BuildCredentialProfileIDs: buildCredentials,
		GitAuthorName: binding.GitAuthorName, GitAuthorEmail: binding.GitAuthorEmail,
		AllowedRuntimeImageIDs: runtimes, DefaultRuntimeImageID: binding.DefaultRuntimeImageID,
		DefaultModelID: binding.DefaultModelID, ModelBudget: budget, Instructions: binding.Instructions,
		QualityCommands: commands, EgressPolicy: egress, CreatedAt: binding.CreatedAt,
		UpdatedAt: binding.UpdatedAt, Version: binding.Version,
	}, nil
}

func restoreBindingRecord(record bindingRecord) (domain.RepositoryBinding, error) {
	var buildCredentials, runtimes []string
	var budget domain.ModelBudget
	var commands []domain.QualityCommand
	var egress domain.EgressPolicy
	for _, decode := range []struct {
		data   []byte
		target any
	}{
		{record.BuildCredentialProfileIDs, &buildCredentials}, {record.AllowedRuntimeImageIDs, &runtimes},
		{record.ModelBudget, &budget}, {record.QualityCommands, &commands}, {record.EgressPolicy, &egress},
	} {
		if err := json.Unmarshal(decode.data, decode.target); err != nil {
			return domain.RepositoryBinding{}, fmt.Errorf("decode persisted Repository Binding: %w", err)
		}
	}
	var report *domain.ValidationReport
	if len(record.ValidationReport) != 0 {
		report = &domain.ValidationReport{}
		if err := json.Unmarshal(record.ValidationReport, report); err != nil {
			return domain.RepositoryBinding{}, fmt.Errorf("decode Repository Binding Validation Report: %w", err)
		}
	}
	return domain.RestoreBinding(domain.PersistedBinding{
		Registration: domain.BindingRegistration{
			ID: record.ID, OrganizationID: record.OrganizationID, TeamID: record.TeamID,
			SourceControlProviderID: record.SourceControlProviderID, Name: record.Name,
			RepositorySSHURL: record.RepositorySSHURL, DefaultBranch: record.DefaultBranch,
			SSHCredentialProfileID: record.SSHCredentialProfileID, BuildCredentialProfileIDs: buildCredentials,
			GitAuthorName: record.GitAuthorName, GitAuthorEmail: record.GitAuthorEmail,
			AllowedRuntimeImageIDs: runtimes, DefaultRuntimeImageID: record.DefaultRuntimeImageID,
			DefaultModelID: record.DefaultModelID, ModelBudget: budget, Instructions: record.Instructions,
			QualityCommands: commands, EgressPolicy: egress,
		},
		ValidationReport: report, ValidatedAt: record.ValidatedAt,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, Version: record.Version,
	})
}
