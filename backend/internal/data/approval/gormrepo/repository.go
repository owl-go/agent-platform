package gormrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/approval/domain"
	"agent-platform/backend/internal/biz/authz"

	"gorm.io/gorm"
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
		return fmt.Errorf("scan JSON from %T", source)
	}
	return nil
}
func (jsonValue) GormDataType() string                          { return "json" }
func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type approvalRecord struct {
	ID             string     `gorm:"column:id;primaryKey"`
	RunID          string     `gorm:"column:run_id"`
	Kind           string     `gorm:"column:kind"`
	Request        jsonValue  `gorm:"column:request;type:jsonb"`
	State          string     `gorm:"column:state"`
	RequestedAt    time.Time  `gorm:"column:requested_at"`
	DecidedBy      *string    `gorm:"column:decided_by"`
	DecidedAt      *time.Time `gorm:"column:decided_at"`
	DecisionReason string     `gorm:"column:decision_reason"`
	Version        int64      `gorm:"column:version"`
}

func (approvalRecord) TableName() string { return "approvals" }

func (repository *Repository) Get(ctx context.Context, id string) (domain.Approval, error) {
	var record approvalRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Approval{}, domain.ErrNotFound
		}
		return domain.Approval{}, fmt.Errorf("load Run Approval: %w", err)
	}
	return restore(record)
}

func (repository *Repository) GetInScope(ctx context.Context, id string, scope authz.ReadScope) (domain.Approval, error) {
	if !scope.Valid() {
		return domain.Approval{}, domain.ErrNotFound
	}
	query := repository.db.WithContext(ctx).
		Table("approvals AS approval").
		Select("approval.*").
		Joins("JOIN runs AS run ON run.id = approval.run_id").
		Joins("JOIN sessions AS session ON session.id = run.session_id").
		Joins("JOIN coding_tasks AS task ON task.id = session.coding_task_id").
		Where("approval.id = ? AND task.organization_id = ?", id, scope.OrganizationID)
	if !scope.AllTeams {
		query = query.Where("task.team_id IN ?", scope.TeamIDs)
	}
	var record approvalRecord
	if err := query.Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Approval{}, domain.ErrNotFound
		}
		return domain.Approval{}, fmt.Errorf("load scoped Run Approval: %w", err)
	}
	return restore(record)
}

func (repository *Repository) ListByRun(ctx context.Context, runID string) ([]domain.Approval, error) {
	var records []approvalRecord
	if err := repository.db.WithContext(ctx).Where("run_id = ?", runID).Order("requested_at, id").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Run Approvals: %w", err)
	}
	values := make([]domain.Approval, 0, len(records))
	for _, record := range records {
		value, err := restore(record)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (repository *Repository) PendingExists(ctx context.Context, runID string) (bool, error) {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&approvalRecord{}).Where("run_id = ? AND state = 'pending'", runID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check pending Run Approval: %w", err)
	}
	return count > 0, nil
}

func (repository *Repository) Create(ctx context.Context, approval domain.Approval) error {
	record := approvalRecord{ID: approval.ID, RunID: approval.RunID, Kind: string(approval.Kind), Request: jsonValue(approval.Request), State: string(approval.State), RequestedAt: approval.RequestedAt, Version: approval.Version}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create Run Approval: %w", err)
	}
	return nil
}

func (repository *Repository) Decide(ctx context.Context, approval domain.Approval, expectedVersion int64) error {
	decidedBy := approval.DecidedBy
	update := repository.db.WithContext(ctx).Model(&approvalRecord{}).
		Where("id = ? AND version = ? AND state = ?", approval.ID, expectedVersion, domain.StatePending).
		Updates(map[string]any{"state": approval.State, "decided_by": &decidedBy, "decided_at": approval.DecidedAt, "decision_reason": approval.DecisionReason, "version": approval.Version})
	if update.Error != nil {
		return fmt.Errorf("decide Run Approval: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func restore(record approvalRecord) (domain.Approval, error) {
	decidedBy := ""
	if record.DecidedBy != nil {
		decidedBy = *record.DecidedBy
	}
	return domain.Restore(domain.Approval{ID: record.ID, RunID: record.RunID, Kind: domain.Kind(record.Kind), Request: append(json.RawMessage(nil), record.Request...), State: domain.State(record.State), RequestedAt: record.RequestedAt, DecidedBy: decidedBy, DecidedAt: record.DecidedAt, DecisionReason: record.DecisionReason, Version: record.Version})
}
