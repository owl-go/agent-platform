package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"

	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

var _ domain.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type providerRecord struct {
	ID             string    `gorm:"column:id;primaryKey"`
	OrganizationID string    `gorm:"column:organization_id"`
	Name           string    `gorm:"column:name"`
	Kind           string    `gorm:"column:kind"`
	BaseURL        string    `gorm:"column:base_url"`
	Enabled        bool      `gorm:"column:enabled"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
	Version        int64     `gorm:"column:version"`
}

func (providerRecord) TableName() string { return "source_control_providers" }

func (repository *Repository) Create(ctx context.Context, provider domain.Provider) error {
	record := providerRecord{
		ID: provider.ID, OrganizationID: provider.OrganizationID, Name: provider.Name, Kind: string(provider.Kind),
		BaseURL: provider.BaseURL, Enabled: provider.Enabled, CreatedAt: provider.CreatedAt,
		UpdatedAt: provider.UpdatedAt, Version: provider.Version,
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrNameExists
		}
		return fmt.Errorf("create Source Control Provider: %w", err)
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, organizationID, id string) (domain.Provider, error) {
	var record providerRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND id = ?", organizationID, id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Provider{}, domain.ErrProviderNotFound
		}
		return domain.Provider{}, fmt.Errorf("load Source Control Provider: %w", err)
	}
	return restore(record)
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]domain.Provider, error) {
	var records []providerRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("name, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Source Control Providers: %w", err)
	}
	providers := make([]domain.Provider, 0, len(records))
	for _, record := range records {
		provider, err := restore(record)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

func (repository *Repository) UpdateStatus(ctx context.Context, provider domain.Provider, expectedVersion int64) error {
	result := repository.db.WithContext(ctx).Model(&providerRecord{}).
		Where("organization_id = ? AND id = ? AND version = ?", provider.OrganizationID, provider.ID, expectedVersion).
		Updates(map[string]any{"enabled": provider.Enabled, "updated_at": provider.UpdatedAt, "version": provider.Version})
	if result.Error != nil {
		return fmt.Errorf("update Source Control Provider status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func restore(record providerRecord) (domain.Provider, error) {
	return domain.Restore(
		record.ID, record.OrganizationID, record.Name, record.Kind, record.BaseURL,
		record.Enabled, record.CreatedAt, record.UpdatedAt, record.Version,
	)
}
