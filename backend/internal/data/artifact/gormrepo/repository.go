package gormrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/artifact/domain"
	"agent-platform/backend/internal/biz/authz"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Repository struct{ database *gorm.DB }

type record struct {
	ID, RunID, Kind, ObjectKey, SHA256, ContentType string
	SizeBytes                                       int64
	Metadata                                        jsonValue `gorm:"type:jsonb"`
	ExpiresAt, DeletedAt                            *time.Time
	CreatedAt                                       time.Time
}

type jsonValue []byte

func (value jsonValue) Value() (driver.Value, error) { return string(value), nil }
func (value *jsonValue) Scan(source any) error {
	switch typed := source.(type) {
	case []byte:
		*value = append((*value)[:0], typed...)
	case string:
		*value = append((*value)[:0], typed...)
	default:
		return fmt.Errorf("scan Artifact JSON from %T", source)
	}
	return nil
}
func (jsonValue) GormDataType() string                          { return "json" }
func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

func (record) TableName() string { return "artifacts" }

func New(database *gorm.DB) *Repository { return &Repository{database: database} }

func (repository *Repository) Create(ctx context.Context, artifact domain.Artifact) error {
	metadata, err := json.Marshal(artifact.Metadata)
	if err != nil {
		return fmt.Errorf("encode Artifact metadata: %w", err)
	}
	value := record{ID: artifact.ID, RunID: artifact.RunID, Kind: artifact.Kind, ObjectKey: artifact.ObjectKey, SizeBytes: artifact.SizeBytes, SHA256: artifact.SHA256, ContentType: artifact.ContentType, Metadata: jsonValue(metadata), ExpiresAt: artifact.ExpiresAt, CreatedAt: artifact.CreatedAt}
	if err := repository.database.WithContext(ctx).Create(&value).Error; err != nil {
		return fmt.Errorf("create Artifact: %w", err)
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, id string) (domain.Artifact, error) {
	var value record
	if err := repository.database.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Take(&value).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Artifact{}, domain.ErrNotFound
		}
		return domain.Artifact{}, fmt.Errorf("load Artifact: %w", err)
	}
	return restore(value)
}

func (repository *Repository) GetInScope(ctx context.Context, id string, scope authz.ReadScope) (domain.Artifact, error) {
	if !scope.Valid() {
		return domain.Artifact{}, domain.ErrNotFound
	}
	query := repository.database.WithContext(ctx).
		Table("artifacts AS artifact").
		Select("artifact.*").
		Joins("JOIN runs AS run ON run.id = artifact.run_id").
		Joins("JOIN sessions AS session ON session.id = run.session_id").
		Joins("JOIN coding_tasks AS task ON task.id = session.coding_task_id").
		Where("artifact.id = ? AND artifact.deleted_at IS NULL AND task.organization_id = ?", id, scope.OrganizationID)
	if !scope.AllTeams {
		query = query.Where("task.team_id IN ?", scope.TeamIDs)
	}
	var value record
	if err := query.Take(&value).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Artifact{}, domain.ErrNotFound
		}
		return domain.Artifact{}, fmt.Errorf("load scoped Artifact: %w", err)
	}
	return restore(value)
}

func (repository *Repository) ListByRun(ctx context.Context, runID string) ([]domain.Artifact, error) {
	var records []record
	if err := repository.database.WithContext(ctx).Where("run_id = ? AND deleted_at IS NULL", runID).Order("created_at, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Run Artifacts: %w", err)
	}
	artifacts := make([]domain.Artifact, 0, len(records))
	for _, value := range records {
		artifact, err := restore(value)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func restore(value record) (domain.Artifact, error) {
	metadata := map[string]string{}
	if err := json.Unmarshal(value.Metadata, &metadata); err != nil {
		return domain.Artifact{}, fmt.Errorf("decode Artifact metadata: %w", err)
	}
	return domain.Restore(domain.Artifact{ID: value.ID, RunID: value.RunID, Kind: value.Kind, ObjectKey: value.ObjectKey, SizeBytes: value.SizeBytes, SHA256: value.SHA256, ContentType: value.ContentType, Metadata: metadata, ExpiresAt: value.ExpiresAt, DeletedAt: value.DeletedAt, CreatedAt: value.CreatedAt})
}
