package modelchat

import (
	"context"
	"encoding/json"
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

func TestAssistantModelUsesStreamingTransportForInternalOpenAIChatStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if !payload.Stream {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(writer, `{"detail":"Stream must be set to true"}`)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()

	result, err := New().Generate(context.Background(), application.ChatRequest{
		Endpoint: server.URL,
		Protocol: "openai_chat",
		ModelID:  "gpt-6-sol",
		APIKey:   []byte("secret"),
		Messages: []application.ChatMessage{{Role: "user", Content: "ping"}},
		Stream:   false,
	}, nil)
	if err != nil || result.Text != "OK" {
		t.Fatalf("internal OpenAI Chat stage = %+v, %v", result, err)
	}
}

func TestAssistantModelUsesStreamingTransportForInternalResponsesStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if request.URL.Path != "/responses" || !payload.Stream {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(writer, `{"detail":"Stream must be set to true"}`)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}}\n\n")
	}))
	defer server.Close()

	result, err := New().Generate(context.Background(), application.ChatRequest{
		Endpoint: server.URL,
		Protocol: "openai_responses",
		ModelID:  "gpt-6-sol",
		APIKey:   []byte("secret"),
		Messages: []application.ChatMessage{{Role: "user", Content: "ping"}},
		Stream:   false,
	}, nil)
	if err != nil || result.Text != "OK" || !result.UsageKnown {
		t.Fatalf("internal Responses stage = %+v, %v", result, err)
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
