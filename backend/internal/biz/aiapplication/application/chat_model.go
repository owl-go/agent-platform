package application

import (
	"context"
	"errors"
)

const (
	ChatFailureAuthentication  = "model_authentication"
	ChatFailureRateLimited     = "model_rate_limited"
	ChatFailureUnavailable     = "model_unavailable"
	ChatFailureConfiguration   = "model_configuration"
	ChatFailureInvalidResponse = "model_response_invalid"
	ChatFailureRequest         = "model_request_failed"
)

// ChatError carries a stable, credential-safe failure code across the model
// boundary. Message and Cause are for server diagnostics and are never sent
// to the client as turn data.
type ChatError struct {
	Code    string
	Message string
	Cause   error
}

func (failure *ChatError) Error() string {
	if failure.Message != "" {
		return failure.Message
	}
	return failure.Code
}

func (failure *ChatError) Unwrap() error { return failure.Cause }

func (failure *ChatError) FailureCode() string { return failure.Code }

func ChatFailureCode(err error) string {
	var failure *ChatError
	if errors.As(err, &failure) {
		return failure.Code
	}
	return ""
}

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
