package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/webhook/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type deliveryRecord struct {
	ID             string          `gorm:"column:id"`
	OrganizationID string          `gorm:"column:organization_id"`
	EventType      string          `gorm:"column:event_type"`
	Payload        json.RawMessage `gorm:"column:payload;type:jsonb"`
	TargetURL      string          `gorm:"column:target_url"`
	State          domain.State    `gorm:"column:state"`
	AttemptCount   int             `gorm:"column:attempt_count"`
	NextAttemptAt  time.Time       `gorm:"column:next_attempt_at"`
	LockedUntil    *time.Time      `gorm:"column:locked_until"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
}

func (deliveryRecord) TableName() string { return "webhook_deliveries" }

func (repository *Repository) Claim(ctx context.Context, now time.Time, lease time.Duration) (domain.Delivery, bool, error) {
	var claimed deliveryRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&deliveryRecord{}).
			Where("state = ? AND locked_until <= ?", domain.StateDelivering, now).
			Updates(map[string]any{
				"state": domain.StateFailed, "next_attempt_at": now, "locked_until": nil,
				"last_error": "delivery lease expired",
			}).Error; err != nil {
			return fmt.Errorf("requeue expired Webhook Deliveries: %w", err)
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("state IN ? AND next_attempt_at <= ?", []domain.State{domain.StatePending, domain.StateFailed}, now).
			Order("next_attempt_at, id").Take(&claimed).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim Webhook Delivery: %w", err)
		}
		lockedUntil := now.Add(lease)
		result := tx.Model(&deliveryRecord{}).Where("id = ? AND state IN ?", claimed.ID, []domain.State{domain.StatePending, domain.StateFailed}).
			Updates(map[string]any{"state": domain.StateDelivering, "attempt_count": gorm.Expr("attempt_count + 1"), "locked_until": lockedUntil})
		if result.Error != nil {
			return fmt.Errorf("lease Webhook Delivery: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("Webhook Delivery claim was lost")
		}
		claimed.State = domain.StateDelivering
		claimed.AttemptCount++
		claimed.LockedUntil = &lockedUntil
		return nil
	})
	if err != nil {
		return domain.Delivery{}, false, err
	}
	if claimed.ID == "" {
		return domain.Delivery{}, false, nil
	}
	return toDomain(claimed), true, nil
}

func (repository *Repository) MarkDelivered(ctx context.Context, id string, now time.Time) error {
	result := repository.db.WithContext(ctx).Model(&deliveryRecord{}).
		Where("id = ? AND state = ?", id, domain.StateDelivering).
		Updates(map[string]any{"state": domain.StateDelivered, "delivered_at": now, "locked_until": nil, "last_error": nil})
	return transitionError(result, "mark Webhook Delivery delivered")
}

func (repository *Repository) MarkFailed(ctx context.Context, id, message string, next time.Time, cancel bool) error {
	state := domain.StateFailed
	if cancel {
		state = domain.StateCancelled
	}
	result := repository.db.WithContext(ctx).Model(&deliveryRecord{}).
		Where("id = ? AND state = ?", id, domain.StateDelivering).
		Updates(map[string]any{"state": state, "next_attempt_at": next, "locked_until": nil, "last_error": message})
	return transitionError(result, "mark Webhook Delivery failed")
}

func transitionError(result *gorm.DB, action string) error {
	if result.Error != nil {
		return fmt.Errorf("%s: %w", action, result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%s: active lease not found", action)
	}
	return nil
}

func toDomain(record deliveryRecord) domain.Delivery {
	return domain.Delivery{
		ID: record.ID, OrganizationID: record.OrganizationID, EventType: record.EventType,
		Payload: append(json.RawMessage(nil), record.Payload...), TargetURL: record.TargetURL,
		State: record.State, AttemptCount: record.AttemptCount, NextAttemptAt: record.NextAttemptAt,
		LockedUntil: record.LockedUntil, CreatedAt: record.CreatedAt,
	}
}
