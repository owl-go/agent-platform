package promptoptimizer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
	"agent-platform/backend/internal/data/aicreation/openaiimages"
	"agent-platform/backend/internal/data/aicreation/promptoptimizer"
)

func TestOptimizerUsesIndependentChatCompletionsConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !body.Stream || len(body.Messages) != 2 || body.Messages[0].Role != "system" || !strings.Contains(body.Messages[0].Content, "Improve the image prompt.") || body.Messages[1].Role != "user" || body.Messages[1].Content != "cat" {
			t.Fatalf("body = %#v", body)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Detailed \"}}]}\n\n"))
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"prompt\"}}]}\n\n"))
		_, _ = writer.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":11}}\n\n"))
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	provider := mustOptimizer(t, resolver{openaiimages.Connection{Endpoint: server.URL + "/v1", APIKey: []byte("secret")}})
	result, err := provider.Optimize(context.Background(), application.OptimizationRequest{Candidate: domain.PromptOptimizationCandidate{ModelID: "gpt-5-mini", Protocol: "openai_chat", Instruction: "Improve the image prompt."}, Prompt: "cat", Locale: "en-US"})
	if err != nil || result.Prompt != "Detailed prompt" || result.InputTokens != 7 || result.OutputTokens != 11 {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
}

type resolver struct{ connection openaiimages.Connection }

func (resolver resolver) ResolvePromptOptimization(context.Context) (openaiimages.Connection, error) {
	return resolver.connection, nil
}
func mustOptimizer(t *testing.T, resolver resolver) *promptoptimizer.Provider {
	t.Helper()
	provider, err := promptoptimizer.New(resolver, nil)
	if err != nil {
		t.Fatal(err)
	}
	return provider
}
