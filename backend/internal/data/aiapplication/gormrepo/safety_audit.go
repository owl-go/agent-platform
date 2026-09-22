package gormrepo

import (
	"context"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
)

type safetyAuditRecord struct {
	ID            string    `gorm:"column:id"`
	OwnerID       string    `gorm:"column:owner_user_id"`
	AssistantID   string    `gorm:"column:assistant_id"`
	Source        string    `gorm:"column:access_source"`
	Decision      string    `gorm:"column:classification"`
	PolicyVersion string    `gorm:"column:policy_version"`
	CreditOutcome string    `gorm:"column:credit_outcome"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (safetyAuditRecord) TableName() string { return "ai_application_safety_audits" }

func (r *Repository) RecordSafetyAudit(ctx context.Context, owner, assistantID, source string, decision domain.SafetyDecision, creditOutcome string) error {
	return r.db.WithContext(ctx).Create(&safetyAuditRecord{
		ID: uuid.NewString(), OwnerID: owner, AssistantID: assistantID, Source: source,
		Decision: string(decision), PolicyVersion: "platform-v1", CreditOutcome: creditOutcome, CreatedAt: time.Now().UTC(),
	}).Error
}
