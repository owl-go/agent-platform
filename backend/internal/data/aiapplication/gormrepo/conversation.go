package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type assistantConversationRecord struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	OwnerID            string    `gorm:"column:owner_user_id"`
	AssistantID        string    `gorm:"column:assistant_id"`
	VisitorHash        string    `gorm:"column:visitor_hash"`
	ShareTokenRevision int64     `gorm:"column:share_token_revision"`
	AssistantName      string    `gorm:"column:assistant_name"`
	Welcome            string    `gorm:"column:welcome"`
	AssistantSnapshot  []byte    `gorm:"column:assistant_snapshot;type:jsonb"`
	ModelSnapshot      []byte    `gorm:"column:model_snapshot;type:jsonb"`
	Summary            string    `gorm:"column:summary"`
	SummaryThroughTurn int       `gorm:"column:summary_through_turn"`
	LastTurn           int       `gorm:"column:last_turn"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (assistantConversationRecord) TableName() string { return "assistant_conversations" }

type assistantTurnRecord struct {
	ID                string     `gorm:"column:id;primaryKey"`
	ConversationID    string     `gorm:"column:conversation_id"`
	TurnNumber        int        `gorm:"column:turn_number"`
	Question          string     `gorm:"column:question"`
	Answer            string     `gorm:"column:answer"`
	Source            string     `gorm:"column:source"`
	FAQID             *string    `gorm:"column:faq_id"`
	State             string     `gorm:"column:state"`
	CancelRequestedAt *time.Time `gorm:"column:cancel_requested_at"`
	InputTokens       int64      `gorm:"column:input_tokens"`
	OutputTokens      int64      `gorm:"column:output_tokens"`
	Error             string     `gorm:"column:error"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
	CompletedAt       *time.Time `gorm:"column:completed_at"`
}

func (assistantTurnRecord) TableName() string { return "assistant_conversation_turns" }

func (r *Repository) CreateAssistantConversation(ctx context.Context, value domain.AssistantConversation) (domain.AssistantConversation, error) {
	now := time.Now().UTC()
	value.ID, value.CreatedAt, value.UpdatedAt = uuid.NewString(), now, now
	assistantSnapshot, err := json.Marshal(value.AssistantSnapshot)
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	modelSnapshot, err := json.Marshal(value.ModelSnapshot)
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	row := assistantConversationRecord{ID: value.ID, OwnerID: value.OwnerID, AssistantID: value.AssistantID, VisitorHash: value.VisitorHash, ShareTokenRevision: value.ShareTokenRevision, AssistantName: value.AssistantName, Welcome: value.Welcome, AssistantSnapshot: assistantSnapshot, ModelSnapshot: modelSnapshot, CreatedAt: now, UpdatedAt: now}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.AssistantConversation{}, mapDBError(err)
	}
	return value, nil
}

func (r *Repository) ListAssistantConversations(ctx context.Context, owner, assistantID string) ([]domain.AssistantConversation, error) {
	var rows []assistantConversationRecord
	if err := r.db.WithContext(ctx).Where("owner_user_id = ? AND assistant_id = ? AND visitor_hash = ''", owner, assistantID).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AssistantConversation, 0, len(rows))
	for _, row := range rows {
		value, err := conversationFromRecord(row)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (r *Repository) GetAssistantConversation(ctx context.Context, owner, id string) (domain.AssistantConversation, error) {
	var row assistantConversationRecord
	if err := r.db.WithContext(ctx).Where("owner_user_id = ? AND id = ? AND visitor_hash = ''", owner, id).Take(&row).Error; err != nil {
		return domain.AssistantConversation{}, mapAssistantReadError(err)
	}
	return conversationFromRecord(row)
}

func (r *Repository) GetPublicAssistantConversation(ctx context.Context, owner, assistantID, id, visitorHash string, shareRevision int64) (domain.AssistantConversation, error) {
	if visitorHash == "" || shareRevision <= 0 {
		return domain.AssistantConversation{}, domain.ErrNotFound
	}
	var row assistantConversationRecord
	if err := r.db.WithContext(ctx).Where("owner_user_id = ? AND assistant_id = ? AND id = ? AND visitor_hash = ? AND share_token_revision = ?", owner, assistantID, id, visitorHash, shareRevision).Take(&row).Error; err != nil {
		return domain.AssistantConversation{}, mapAssistantReadError(err)
	}
	return conversationFromRecord(row)
}

func conversationFromRecord(row assistantConversationRecord) (domain.AssistantConversation, error) {
	value := domain.AssistantConversation{ID: row.ID, OwnerID: row.OwnerID, AssistantID: row.AssistantID, VisitorHash: row.VisitorHash, ShareTokenRevision: row.ShareTokenRevision, AssistantName: row.AssistantName, Welcome: row.Welcome, Summary: row.Summary, SummaryThroughTurn: row.SummaryThroughTurn, LastTurn: row.LastTurn, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if err := json.Unmarshal(row.AssistantSnapshot, &value.AssistantSnapshot); err != nil {
		return domain.AssistantConversation{}, fmt.Errorf("decode Assistant snapshot: %w", err)
	}
	if err := json.Unmarshal(row.ModelSnapshot, &value.ModelSnapshot); err != nil {
		return domain.AssistantConversation{}, fmt.Errorf("decode Assistant model snapshot: %w", err)
	}
	return value, nil
}

func (r *Repository) ListAssistantTurns(ctx context.Context, owner, conversationID string) ([]domain.AssistantTurn, error) {
	if err := r.assistantConversationExists(ctx, owner, conversationID); err != nil {
		return nil, err
	}
	var rows []assistantTurnRecord
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Order("turn_number ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.AssistantTurn, 0, len(rows))
	for _, row := range rows {
		result = append(result, turnFromRecord(row))
	}
	return result, nil
}

func (r *Repository) assistantConversationExists(ctx context.Context, owner, conversationID string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&assistantConversationRecord{}).Where("id = ? AND owner_user_id = ?", conversationID, owner).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) BeginAssistantTurn(ctx context.Context, owner, conversationID, question string) (domain.AssistantTurn, error) {
	var result domain.AssistantTurn
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation assistantConversationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", conversationID, owner).Take(&conversation).Error; err != nil {
			return mapAssistantReadError(err)
		}
		// An API process can disappear while a provider call is active. An old
		// generating turn remains in the audit, but cannot block the owner forever.
		cutoff := time.Now().UTC().Add(-10 * time.Minute)
		if err := tx.Model(&assistantTurnRecord{}).Where("conversation_id = ? AND state = 'generating' AND updated_at < ?", conversationID, cutoff).Updates(map[string]any{"state": "failed", "error": "interrupted", "completed_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		var active int64
		if err := tx.Model(&assistantTurnRecord{}).Where("conversation_id = ? AND state = 'generating'", conversationID).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return domain.ErrConflict
		}
		now := time.Now().UTC()
		row := assistantTurnRecord{ID: uuid.NewString(), ConversationID: conversationID, TurnNumber: conversation.LastTurn + 1, Question: question, State: "generating", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return mapDBError(err)
		}
		if err := tx.Model(&conversation).Updates(map[string]any{"last_turn": row.TurnNumber, "updated_at": now}).Error; err != nil {
			return err
		}
		result = turnFromRecord(row)
		return nil
	})
	return result, err
}

func (r *Repository) SaveAssistantTurnProgress(ctx context.Context, owner, conversationID, turnID, answer string) error {
	result := r.db.WithContext(ctx).Model(&assistantTurnRecord{}).Where("id = ? AND conversation_id = (SELECT id FROM assistant_conversations WHERE id = ? AND owner_user_id = ?) AND state = 'generating' AND cancel_requested_at IS NULL", turnID, conversationID, owner).Updates(map[string]any{"answer": answer, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		if err := r.assistantConversationExists(ctx, owner, conversationID); err != nil {
			return err
		}
		return domain.ErrConflict
	}
	return nil
}

func (r *Repository) FinishAssistantTurn(ctx context.Context, owner, conversationID, turnID, state, source, faqID, answer string, inputTokens, outputTokens int64) (domain.AssistantTurn, error) {
	var result domain.AssistantTurn
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation assistantConversationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", conversationID, owner).Take(&conversation).Error; err != nil {
			return mapAssistantReadError(err)
		}
		var row assistantTurnRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND conversation_id = ?", turnID, conversationID).Take(&row).Error; err != nil {
			return mapAssistantReadError(err)
		}
		if row.State != "generating" && row.State != "cancelled" {
			return domain.ErrConflict
		}
		if row.CancelRequestedAt != nil || row.State == "cancelled" {
			state = "cancelled"
		}
		now := time.Now().UTC()
		updates := map[string]any{"state": state, "source": source, "answer": answer, "input_tokens": inputTokens, "output_tokens": outputTokens, "updated_at": now, "completed_at": now}
		if faqID != "" {
			updates["faq_id"] = faqID
		}
		if err := tx.Model(&row).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&conversation).Update("updated_at", now).Error; err != nil {
			return err
		}
		row.State, row.Source, row.Answer, row.InputTokens, row.OutputTokens, row.CompletedAt, row.UpdatedAt = state, source, answer, inputTokens, outputTokens, &now, now
		if faqID != "" {
			row.FAQID = &faqID
		}
		result = turnFromRecord(row)
		return nil
	})
	return result, err
}

func (r *Repository) CancelAssistantTurn(ctx context.Context, owner, conversationID, turnID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation assistantConversationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", conversationID, owner).Take(&conversation).Error; err != nil {
			return mapAssistantReadError(err)
		}
		now := time.Now().UTC()
		result := tx.Model(&assistantTurnRecord{}).Where("id = ? AND conversation_id = ? AND state = 'generating'", turnID, conversationID).Updates(map[string]any{"state": "cancelled", "cancel_requested_at": now, "completed_at": now, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *Repository) SaveAssistantSummary(ctx context.Context, owner, conversationID string, throughTurn int, summary string) error {
	result := r.db.WithContext(ctx).Model(&assistantConversationRecord{}).Where("id = ? AND owner_user_id = ? AND summary_through_turn < ?", conversationID, owner, throughTurn).Updates(map[string]any{"summary": summary, "summary_through_turn": throughTurn, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.assistantConversationExists(ctx, owner, conversationID)
	}
	return nil
}

func (r *Repository) MarkAssistantInterruptedCreditsReleased(ctx context.Context, owner, conversationID, turnID string) error {
	if err := r.assistantConversationExists(ctx, owner, conversationID); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&assistantTurnRecord{}).Where("id = ? AND conversation_id = ? AND state = 'failed' AND error = 'interrupted'", turnID, conversationID).Update("error", "credits_released").Error
}

func turnFromRecord(row assistantTurnRecord) domain.AssistantTurn {
	value := domain.AssistantTurn{ID: row.ID, ConversationID: row.ConversationID, TurnNumber: row.TurnNumber, Question: row.Question, Answer: row.Answer, Source: row.Source, State: row.State, Error: row.Error, InputTokens: row.InputTokens, OutputTokens: row.OutputTokens, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, CompletedAt: row.CompletedAt}
	if row.FAQID != nil {
		value.FAQID = *row.FAQID
	}
	return value
}

func mapAssistantReadError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return mapDBError(err)
}
