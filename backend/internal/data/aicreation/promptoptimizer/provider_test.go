package promptoptimizer_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
	"agent-platform/backend/internal/data/aicreation/openaiimages"
	"agent-platform/backend/internal/data/aicreation/promptoptimizer"
)

func TestOptimizerUsesIndependentResponsesConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/responses" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("request = %s %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": "Detailed prompt"}}}}, "usage": map[string]any{"input_tokens": 7, "output_tokens": 11}})
	}))
	defer server.Close()
	provider := mustOptimizer(t, resolver{openaiimages.Connection{Endpoint: server.URL + "/v1", APIKey: []byte("secret")}})
	result, err := provider.Optimize(context.Background(), application.OptimizationRequest{Candidate: domain.PromptOptimizationCandidate{ModelID: "gpt-5-mini", Protocol: "openai_responses", Instruction: "Improve the image prompt."}, Prompt: "cat", Locale: "en-US"})
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
