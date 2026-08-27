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
	models, err := New(server.Client()).Discover(context.Background(), connection, "secret")
	if err != nil {
		t.Fatalf("discover models: %v", err)
	}
	if len(models) != 2 || models[0].ModelID != "embed-model" || models[0].ModelType != "embedding" {
		t.Fatalf("unexpected models: %#v", models)
	}
	if models[1].Compatibility[1].RuntimeEngine != domain.RuntimeCodex || models[1].Compatibility[1].Status != "unverified" {
		t.Fatalf("unexpected compatibility: %#v", models[1].Compatibility)
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

	models, err := New(server.Client()).Discover(context.Background(), domain.ModelProviderConnection{ProviderType: "google_gemini", Endpoint: server.URL, Protocols: []string{"gemini"}}, "secret")
	if err != nil {
		t.Fatalf("discover Gemini models: %v", err)
	}
	if len(models) != 1 || models[0].ModelID != "gemini-test" || models[0].ModelType != "text" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestMaintainedCatalogDoesNotCallProvider(t *testing.T) {
	catalog := New(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("maintained catalog must not call a provider list endpoint")
		return nil, nil
	})})
	models, err := catalog.Discover(context.Background(), domain.ModelProviderConnection{ProviderType: "alibaba_bailian", Protocols: []string{"openai_responses"}}, "secret")
	if err != nil {
		t.Fatalf("discover maintained models: %v", err)
	}
	if len(models) == 0 || models[0].Compatibility[1].Status != "unverified" {
		t.Fatalf("unexpected maintained models: %#v", models)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
