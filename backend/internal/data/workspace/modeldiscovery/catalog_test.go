package modeldiscovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestDiscoverOpenAICompatibleModels(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", got)
		}
		_, _ = response.Write([]byte(`{"data":[{"id":"text-model"},{"id":"embed-model"}]}`))
	}))
	defer server.Close()

	connection := domain.ModelProviderConnection{ProviderType: "custom_openai", Endpoint: server.URL + "/v1", Protocols: []string{"openai_responses"}}
	result, err := New(server.Client()).Discover(context.Background(), connection, "secret")
	if err != nil {
		t.Fatalf("discover models: %v", err)
	}
	if result.Source != "provider" || len(result.Models) != 2 || result.Models[0].ModelID != "embed-model" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Models[1].Compatibility[1].RuntimeEngine != domain.RuntimeCodex || result.Models[1].Compatibility[1].Status != "unverified" {
		t.Fatalf("unexpected compatibility: %#v", result.Models[1].Compatibility)
	}
}

func TestDiscoverGeminiModels(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1beta/models" || request.Header.Get("x-goog-api-key") != "secret" {
			t.Fatalf("unexpected request: %s %#v", request.URL.Path, request.Header)
		}
		_, _ = response.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test","supportedGenerationMethods":["generateContent"]}]}`))
	}))
	defer server.Close()

	result, err := New(server.Client()).Discover(context.Background(), domain.ModelProviderConnection{ProviderType: "google_gemini", Endpoint: server.URL, Protocols: []string{"gemini"}}, "secret")
	if err != nil {
		t.Fatalf("discover Gemini models: %v", err)
	}
	if result.Source != "provider" || len(result.Models) != 1 || result.Models[0].ModelID != "gemini-test" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestDiscoverFallsBackToProviderDefaultsWhenModelsEndpointIsUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := New(server.Client()).Discover(context.Background(), domain.ModelProviderConnection{ProviderType: "openai", Endpoint: server.URL, Protocols: []string{"openai_responses"}}, "secret")
	if err != nil {
		t.Fatalf("fall back to defaults: %v", err)
	}
	if result.Source != "default" || len(result.Models) == 0 || result.Models[0].ModelID != "gpt-5.6-sol" {
		t.Fatalf("unexpected fallback result: %#v", result)
	}
	if result.Models[0].Compatibility[1].Status != "unverified" {
		t.Fatalf("unexpected compatibility: %#v", result.Models[0].Compatibility)
	}
}
