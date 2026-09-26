package application

import "context"

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Endpoint string
	Protocol string
	ModelID  string
	APIKey   []byte
	Messages []ChatMessage
	Stream   bool
}

type ChatResult struct {
	Text         string
	InputTokens  int64
	OutputTokens int64
	UsageKnown   bool
}

// ChatModel is the narrow model boundary for assistant classification,
// summarization, and answer generation. The caller owns Credits and storage.
type ChatModel interface {
	Generate(context.Context, ChatRequest, func(string) error) (ChatResult, error)
}
