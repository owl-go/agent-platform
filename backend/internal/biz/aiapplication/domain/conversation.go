package domain

import "time"

// AssistantConversation is a separate, owner-private transcript. Its snapshots
// contain no credential and remain historical evidence only; every new turn
// resolves the current Assistant configuration and Provider Model connection.
type AssistantConversation struct {
	ID                 string         `json:"id"`
	OwnerID            string         `json:"-"`
	AssistantID        string         `json:"assistant_id"`
	VisitorHash        string         `json:"-"`
	ShareTokenRevision int64          `json:"-"`
	AssistantName      string         `json:"assistant_name"`
	Welcome            string         `json:"welcome"`
	AssistantSnapshot  SmartAssistant `json:"-"`
	ModelSnapshot      AssistantModel `json:"-"`
	Summary            string         `json:"-"`
	SummaryThroughTurn int            `json:"-"`
	LastTurn           int            `json:"-"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type AssistantModel struct {
	ProviderModelID   string `json:"provider_model_id"`
	ConnectionID      string `json:"connection_id"`
	CredentialOwnerID string `json:"credential_owner_id"`
	ProviderType      string `json:"provider_type"`
	Protocol          string `json:"protocol"`
	ModelID           string `json:"model_id"`
	Endpoint          string `json:"endpoint"`
	ConnectionVersion int64  `json:"connection_version"`
}

type AssistantTurn struct {
	ID             string     `json:"id"`
	ConversationID string     `json:"conversation_id"`
	TurnNumber     int        `json:"turn_number"`
	Question       string     `json:"question"`
	Answer         string     `json:"answer"`
	Source         string     `json:"source"`
	FAQID          string     `json:"faq_id,omitempty"`
	State          string     `json:"state"`
	Error          string     `json:"failure_code,omitempty"`
	InputTokens    int64      `json:"input_tokens"`
	OutputTokens   int64      `json:"output_tokens"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
