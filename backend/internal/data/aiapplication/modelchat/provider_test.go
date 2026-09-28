package modelchat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/application"
)

type codedAssistantModelError interface {
	FailureCode() string
}

func TestAssistantModelStreamsProviderDeltasAndUsage(t *testing.T) {
	tests := []struct {
		name, protocol, path, authHeader, payload string
	}{
		{"chat", "openai_chat", "/v1/chat/completions", "Authorization", `data: {"choices":[{"delta":{"content":"你"}}]}

data: {"choices":[{"delta":{"content":"好"}}],"usage":{"prompt_tokens":11,"completion_tokens":2}}

data: [DONE]

`},
		{"responses", "openai_responses", "/v1/responses", "Authorization", `data: {"type":"response.output_text.delta","delta":"你"}

data: {"type":"response.output_text.delta","delta":"好"}

data: {"type":"response.completed","response":{"usage":{"input_tokens":11,"output_tokens":2}}}

`},
		{"anthropic", "anthropic_messages", "/v1/messages", "x-api-key", `data: {"type":"message_start","message":{"usage":{"input_tokens":11}}}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"你"}}

data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"好"}}

data: {"type":"message_delta","usage":{"output_tokens":2}}

`},
		{"gemini", "gemini", "/v1/models/test-model:streamGenerateContent", "x-goog-api-key", `data: {"candidates":[{"content":{"parts":[{"text":"你"}]}}]}

data: {"candidates":[{"content":{"parts":[{"text":"好"}]}}],"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":2}}

`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.path || request.Header.Get(test.authHeader) == "" {
					t.Errorf("request path/header = %s/%q", request.URL.Path, request.Header.Get(test.authHeader))
				}
				writer.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprint(writer, test.payload)
			}))
			defer server.Close()
			provider := New()
			var chunks []string
			result, err := provider.Generate(context.Background(), application.ChatRequest{Endpoint: server.URL + "/v1", Protocol: test.protocol, ModelID: "test-model", APIKey: []byte("secret"), Messages: []application.ChatMessage{{Role: "user", Content: "hello"}}, Stream: true}, func(delta string) error { chunks = append(chunks, delta); return nil })
			if err != nil {
				t.Fatal(err)
			}
			if result.Text != "你好" || strings.Join(chunks, "") != "你好" || result.InputTokens != 11 || result.OutputTokens != 2 || !result.UsageKnown {
				t.Fatalf("stream = %+v, chunks=%v", result, chunks)
			}
		})
	}
}

func TestAssistantModelRejectsNonStreamingFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"choices":[{"message":{"content":"whole answer"}}]}`)
	}))
	defer server.Close()
	_, err := New().Generate(context.Background(), application.ChatRequest{Endpoint: server.URL, Protocol: "openai_chat", ModelID: "test-model", APIKey: []byte("secret"), Messages: []application.ChatMessage{{Role: "user", Content: "hello"}}, Stream: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "streaming response") {
		t.Fatalf("non-streaming fallback = %v", err)
	}
}

func TestAssistantModelClassifiesRejectedCredentialWithoutExposingProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(writer, `{"error":"Invalid API key","secret_detail":"do not expose"}`)
	}))
	defer server.Close()

	_, err := New().Generate(context.Background(), application.ChatRequest{Endpoint: server.URL, Protocol: "openai_chat", ModelID: "test-model", APIKey: []byte("stale-secret"), Messages: []application.ChatMessage{{Role: "user", Content: "hello"}}}, nil)
	var coded codedAssistantModelError
	if !errors.As(err, &coded) || coded.FailureCode() != "model_authentication" {
		t.Fatalf("credential rejection code = %T %v", err, err)
	}
	if strings.Contains(err.Error(), "secret_detail") {
		t.Fatalf("provider response body leaked through error: %v", err)
	}
}
