package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/modelcatalog/domain"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

var _ domain.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type credentialRecord struct {
	ID             string     `gorm:"column:id;primaryKey"`
	OrganizationID string     `gorm:"column:organization_id"`
	TeamID         *string    `gorm:"column:team_id"`
	Name           string     `gorm:"column:name"`
	Kind           string     `gorm:"column:kind"`
	SecretRef      string     `gorm:"column:secret_ref"`
	DisabledAt     *time.Time `gorm:"column:disabled_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	Version        int64      `gorm:"column:version"`
}

func (credentialRecord) TableName() string { return "credential_profiles" }

type modelRecord struct {
	ID                  string    `gorm:"column:id;primaryKey"`
	OrganizationID      string    `gorm:"column:organization_id"`
	Name                string    `gorm:"column:name"`
	ModelID             string    `gorm:"column:model_id"`
	Endpoint            string    `gorm:"column:endpoint"`
	CredentialProfileID string    `gorm:"column:credential_profile_id"`
	Enabled             bool      `gorm:"column:enabled"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
	Version             int64     `gorm:"column:version"`
}

func (modelRecord) TableName() string { return "configured_models" }

func (repository *Repository) CreateCredential(ctx context.Context, profile domain.CredentialProfile) error {
	record := credentialRecord{
		ID: profile.ID, OrganizationID: profile.OrganizationID, TeamID: profile.TeamID,
		Name: profile.Name, Kind: string(profile.Kind), SecretRef: profile.SecretRef,
		DisabledAt: profile.DisabledAt, CreatedAt: profile.CreatedAt, UpdatedAt: profile.UpdatedAt, Version: profile.Version,
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrCatalogNameExists
		}
		return fmt.Errorf("create Credential Profile: %w", err)
	}
	return nil
}

func (repository *Repository) GetCredential(ctx context.Context, organizationID, id string) (domain.CredentialProfile, error) {
	var record credentialRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND id = ?", organizationID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.CredentialProfile{}, domain.ErrCredentialProfileNotFound
		}
		return domain.CredentialProfile{}, fmt.Errorf("load Credential Profile: %w", err)
	}
	return restoreCredential(record)
}

func (repository *Repository) ListCredentials(ctx context.Context, organizationID string) ([]domain.CredentialProfile, error) {
	var records []credentialRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("name, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Credential Profiles: %w", err)
	}
	profiles := make([]domain.CredentialProfile, 0, len(records))
	for _, record := range records {
		profile, err := restoreCredential(record)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (repository *Repository) UpdateCredentialStatus(ctx context.Context, profile domain.CredentialProfile, expectedVersion int64) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&credentialRecord{}).
			Where("organization_id = ? AND id = ? AND version = ?", profile.OrganizationID, profile.ID, expectedVersion).
			Updates(map[string]any{"disabled_at": profile.DisabledAt, "updated_at": profile.UpdatedAt, "version": profile.Version})
		if result.Error != nil {
			return fmt.Errorf("update Credential Profile status: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrConcurrentUpdate
		}
		if profile.DisabledAt != nil {
			if err := tx.Model(&modelRecord{}).
				Where("organization_id = ? AND credential_profile_id = ? AND enabled = true", profile.OrganizationID, profile.ID).
				Updates(map[string]any{"enabled": false, "updated_at": profile.UpdatedAt, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return fmt.Errorf("disable Configured Models for revoked Credential Profile: %w", err)
			}
		}
		return nil
	})
}

func (repository *Repository) CreateModel(ctx context.Context, model domain.ConfiguredModel) error {
	record := modelRecord{
		ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, ModelID: model.ModelID,
		Endpoint: model.Endpoint, CredentialProfileID: model.CredentialProfileID, Enabled: model.Enabled,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt, Version: model.Version,
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrCatalogNameExists
		}
		return fmt.Errorf("create Configured Model: %w", err)
	}
	return nil
}

func (repository *Repository) GetModel(ctx context.Context, organizationID, id string) (domain.ConfiguredModel, error) {
	var record modelRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND id = ?", organizationID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ConfiguredModel{}, domain.ErrConfiguredModelNotFound
		}
		return domain.ConfiguredModel{}, fmt.Errorf("load Configured Model: %w", err)
	}
	return restoreModel(record)
}

func (repository *Repository) ListModels(ctx context.Context, organizationID string) ([]domain.ConfiguredModel, error) {
	var records []modelRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("name, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Configured Models: %w", err)
	}
	models := make([]domain.ConfiguredModel, 0, len(records))
	for _, record := range records {
		model, err := restoreModel(record)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, nil
}

func (repository *Repository) UpdateModelStatus(ctx context.Context, model domain.ConfiguredModel, expectedVersion int64) error {
	result := repository.db.WithContext(ctx).Model(&modelRecord{}).
		Where("organization_id = ? AND id = ? AND version = ?", model.OrganizationID, model.ID, expectedVersion).
		Updates(map[string]any{"enabled": model.Enabled, "updated_at": model.UpdatedAt, "version": model.Version})
	if result.Error != nil {
		return fmt.Errorf("update Configured Model status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func restoreCredential(record credentialRecord) (domain.CredentialProfile, error) {
	return domain.RestoreCredential(
		record.ID, record.OrganizationID, record.TeamID, record.Name, record.Kind, record.SecretRef,
		record.DisabledAt, record.CreatedAt, record.UpdatedAt, record.Version,
	)
}

func restoreModel(record modelRecord) (domain.ConfiguredModel, error) {
	return domain.RestoreModel(
		record.ID, record.OrganizationID, record.Name, record.ModelID, record.Endpoint,
		record.CredentialProfileID, record.Enabled, record.CreatedAt, record.UpdatedAt, record.Version,
	)
}
