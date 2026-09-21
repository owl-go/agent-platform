package gormrepo

import (
	"context"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type externalConversationRecord struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	AssistantID        string    `gorm:"column:assistant_id"`
	OwnerID            string    `gorm:"column:owner_user_id"`
	ExecutionSessionID string    `gorm:"column:execution_session_id"`
	VisitorHash        string    `gorm:"column:visitor_hash"`
	AssistantSnapshot  []byte    `gorm:"column:assistant_snapshot;type:jsonb"`
	ShareTokenRevision int64     `gorm:"column:share_token_revision"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (externalConversationRecord) TableName() string { return "external_conversations" }

type externalResponseRecord struct {
	ID                 string    `gorm:"column:id;primaryKey"`
	ConversationID     string    `gorm:"column:conversation_id"`
	AssistantMessageID int64     `gorm:"column:assistant_message_id"`
	CreatedAt          time.Time `gorm:"column:created_at"`
}

func (externalResponseRecord) TableName() string { return "external_conversation_responses" }

func (r *Repository) CreateExternalConversation(ctx context.Context, conversation domain.ExternalConversation) (domain.ExternalConversation, error) {
	now := time.Now().UTC()
	conversation.ID, conversation.CreatedAt, conversation.UpdatedAt = uuid.NewString(), now, now
	row := externalConversationRecord{ID: conversation.ID, AssistantID: conversation.AssistantID, OwnerID: conversation.OwnerID, ExecutionSessionID: conversation.ExecutionSessionID, VisitorHash: conversation.VisitorHash, AssistantSnapshot: conversation.AssistantSnapshot, ShareTokenRevision: conversation.ShareTokenRevision, CreatedAt: now, UpdatedAt: now}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ExternalConversation{}, mapDBError(err)
	}
	return conversation, nil
}

func (r *Repository) GetExternalConversation(ctx context.Context, id, assistantID, visitorHash string) (domain.ExternalConversation, error) {
	query := r.db.WithContext(ctx).Where("id = ? AND visitor_hash = ?", id, visitorHash)
	if assistantID != "" {
		query = query.Where("assistant_id = ?", assistantID)
	}
	var row externalConversationRecord
	if err := query.Take(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ExternalConversation{}, domain.ErrNotFound
		}
		return domain.ExternalConversation{}, err
	}
	return domain.ExternalConversation{ID: row.ID, AssistantID: row.AssistantID, OwnerID: row.OwnerID, ExecutionSessionID: row.ExecutionSessionID, VisitorHash: row.VisitorHash, AssistantSnapshot: row.AssistantSnapshot, ShareTokenRevision: row.ShareTokenRevision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) CreateExternalResponse(ctx context.Context, response domain.ExternalResponse) (domain.ExternalResponse, error) {
	response.ID = uuid.NewString()
	response.CreatedAt = time.Now().UTC()
	row := externalResponseRecord{ID: response.ID, ConversationID: response.ConversationID, AssistantMessageID: response.AssistantMessageID, CreatedAt: response.CreatedAt}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ExternalResponse{}, mapDBError(err)
	}
	return response, nil
}

func (r *Repository) GetExternalResponse(ctx context.Context, id, conversationID, visitorHash string) (domain.ExternalResponse, error) {
	var row struct {
		externalResponseRecord
		State     string  `gorm:"column:state"`
		Content   string  `gorm:"column:content"`
		ErrorText *string `gorm:"column:error"`
	}
	err := r.db.WithContext(ctx).Table("external_conversation_responses AS response").
		Select("response.*, message.state, message.content, message.error").
		Joins("JOIN external_conversations AS conversation ON conversation.id = response.conversation_id AND conversation.visitor_hash = ?", visitorHash).
		Joins("JOIN session_messages AS message ON message.id = response.assistant_message_id").
		Where("response.id = ? AND response.conversation_id = ?", id, conversationID).Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domain.ExternalResponse{}, domain.ErrNotFound
		}
		return domain.ExternalResponse{}, err
	}
	response := domain.ExternalResponse{ID: row.ID, ConversationID: row.ConversationID, AssistantMessageID: row.AssistantMessageID, State: row.State, Content: row.Content, CreatedAt: row.CreatedAt}
	if row.ErrorText != nil {
		response.Error = *row.ErrorText
	}
	return response, nil
}
