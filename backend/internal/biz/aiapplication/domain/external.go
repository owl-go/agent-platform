package domain

import "time"

type ExternalConversation struct {
	ID                 string    `json:"id"`
	AssistantID        string    `json:"assistant_id"`
	OwnerID            string    `json:"-"`
	ExecutionSessionID string    `json:"-"`
	VisitorHash        string    `json:"-"`
	AssistantSnapshot  []byte    `json:"-"`
	ShareTokenRevision int64     `json:"share_token_revision"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ExternalResponse struct {
	ID                 string    `json:"id"`
	ConversationID     string    `json:"conversation_id"`
	AssistantMessageID int64     `json:"-"`
	State              string    `json:"state"`
	Content            string    `json:"content"`
	Error              string    `json:"error,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
