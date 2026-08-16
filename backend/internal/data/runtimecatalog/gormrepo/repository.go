package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/domain"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

type runtimeImageRecord struct {
	ID             string          `gorm:"column:id;primaryKey"`
	Runtime        string          `gorm:"column:runtime"`
	CLIVersion     string          `gorm:"column:cli_version"`
	AdapterVersion string          `gorm:"column:adapter_version"`
	ImageDigest    string          `gorm:"column:image_digest"`
	Capabilities   json.RawMessage `gorm:"column:capabilities;type:jsonb"`
	Status         string          `gorm:"column:status"`
	BlockedReason  *string         `gorm:"column:blocked_reason"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at"`
	Version        int64           `gorm:"column:version"`
}

func (runtimeImageRecord) TableName() string { return "runtime_images" }

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) Create(ctx context.Context, image domain.RuntimeImage) error {
	record, err := toRecord(image)
	if err != nil {
		return err
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrImageDigestExists
		}
		return fmt.Errorf("create Runtime Image: %w", err)
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, id string) (domain.RuntimeImage, error) {
	var record runtimeImageRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RuntimeImage{}, domain.ErrRuntimeImageNotFound
		}
		return domain.RuntimeImage{}, fmt.Errorf("load Runtime Image: %w", err)
	}
	return restore(record)
}

func (repository *Repository) List(ctx context.Context) ([]domain.RuntimeImage, error) {
	var records []runtimeImageRecord
	if err := repository.db.WithContext(ctx).Order("runtime, created_at DESC, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Runtime Images: %w", err)
	}
	images := make([]domain.RuntimeImage, 0, len(records))
	for _, record := range records {
		image, err := restore(record)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, nil
}

func (repository *Repository) UpdateStatus(ctx context.Context, image domain.RuntimeImage, expectedVersion int64) error {
	var blockedReason any
	if image.Status == domain.Blocked {
		blockedReason = image.BlockedReason
	}
	result := repository.db.WithContext(ctx).Model(&runtimeImageRecord{}).
		Where("id = ? AND version = ?", image.ID, expectedVersion).
		Updates(map[string]any{
			"status": image.Status, "blocked_reason": blockedReason,
			"updated_at": image.UpdatedAt, "version": image.Version,
		})
	if result.Error != nil {
		return fmt.Errorf("update Runtime Image status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func toRecord(image domain.RuntimeImage) (runtimeImageRecord, error) {
	capabilities, err := json.Marshal(image.Capabilities)
	if err != nil {
		return runtimeImageRecord{}, fmt.Errorf("encode Runtime Capabilities: %w", err)
	}
	var blockedReason *string
	if image.Status == domain.Blocked {
		reason := image.BlockedReason
		blockedReason = &reason
	}
	return runtimeImageRecord{
		ID: image.ID, Runtime: string(image.Runtime), CLIVersion: image.CLIVersion,
		AdapterVersion: image.AdapterVersion, ImageDigest: image.ImageDigest,
		Capabilities: capabilities, Status: string(image.Status), BlockedReason: blockedReason,
		CreatedAt: image.CreatedAt, UpdatedAt: image.UpdatedAt, Version: image.Version,
	}, nil
}

func restore(record runtimeImageRecord) (domain.RuntimeImage, error) {
	capabilities := make(map[string]bool)
	if err := json.Unmarshal(record.Capabilities, &capabilities); err != nil {
		return domain.RuntimeImage{}, fmt.Errorf("decode Runtime Capabilities: %w", err)
	}
	blockedReason := ""
	if record.BlockedReason != nil {
		blockedReason = *record.BlockedReason
	}
	return domain.Restore(
		record.ID, record.Runtime, record.CLIVersion, record.AdapterVersion, record.ImageDigest,
		capabilities, record.Status, blockedReason, record.CreatedAt, record.UpdatedAt, record.Version,
	)
}
