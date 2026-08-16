package gormrepo

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/audit/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

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

type eventRecord struct {
	ID             int64     `gorm:"column:id;primaryKey"`
	OrganizationID string    `gorm:"column:organization_id"`
	TeamID         *string   `gorm:"column:team_id"`
	ActorUserID    *string   `gorm:"column:actor_user_id"`
	Action         string    `gorm:"column:action"`
	ResourceType   string    `gorm:"column:resource_type"`
	ResourceID     string    `gorm:"column:resource_id"`
	Details        jsonValue `gorm:"column:details;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (eventRecord) TableName() string { return "audit_events" }

func (repository *Repository) Search(ctx context.Context, query domain.Query) ([]domain.Event, error) {
	database := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ?", query.OrganizationID, query.TeamID)
	if query.Action != "" {
		database = database.Where("action = ?", query.Action)
	}
	if query.ResourceType != "" {
		database = database.Where("resource_type = ?", query.ResourceType)
	}
	if query.ResourceID != "" {
		database = database.Where("resource_id = ?", query.ResourceID)
	}
	if query.ActorUserID != "" {
		database = database.Where("actor_user_id = ?", query.ActorUserID)
	}
	if query.CreatedFrom != nil {
		database = database.Where("created_at >= ?", query.CreatedFrom.UTC())
	}
	if query.CreatedTo != nil {
		database = database.Where("created_at <= ?", query.CreatedTo.UTC())
	}
	var records []eventRecord
	if err := database.Order("created_at DESC, id DESC").Limit(query.Limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("search Audit Events: %w", err)
	}
	values := make([]domain.Event, 0, len(records))
	for _, record := range records {
		teamID, actorID := "", ""
		if record.TeamID != nil {
			teamID = *record.TeamID
		}
		if record.ActorUserID != nil {
			actorID = *record.ActorUserID
		}
		value, err := domain.Restore(domain.Event{ID: record.ID, OrganizationID: record.OrganizationID, TeamID: teamID, ActorUserID: actorID, Action: record.Action, ResourceType: record.ResourceType, ResourceID: record.ResourceID, Details: append(json.RawMessage(nil), record.Details...), CreatedAt: record.CreatedAt})
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
