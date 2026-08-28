package modeldiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type Catalog struct{ client *http.Client }

func New(client *http.Client) *Catalog {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Catalog{client: client}
}

var _ application.ModelCatalog = (*Catalog)(nil)

func (catalog *Catalog) Discover(ctx context.Context, connection domain.ModelProviderConnection, apiKey string) (application.ModelCatalogResult, error) {
	models, err := catalog.discoverProvider(ctx, connection, apiKey)
	if err == nil && len(models) > 0 {
		return application.ModelCatalogResult{Models: withCompatibility(models, connection.Protocols), Source: "provider"}, nil
	}
	if defaults, ok := defaultModels(connection.ProviderType); ok {
		return application.ModelCatalogResult{Models: withCompatibility(defaults, connection.Protocols), Source: "default"}, nil
	}
	return application.ModelCatalogResult{}, err
}

func (catalog *Catalog) discoverProvider(ctx context.Context, connection domain.ModelProviderConnection, apiKey string) ([]domain.ProviderModel, error) {
	endpoint, err := modelsEndpoint(connection)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build Provider Model request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	switch connection.ProviderType {
	case "anthropic":
		request.Header.Set("x-api-key", apiKey)
		request.Header.Set("anthropic-version", "2023-06-01")
	case "google_gemini":
		request.Header.Set("x-goog-api-key", apiKey)
	default:
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response, err := catalog.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("list Provider Models: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read Provider Model response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Provider Models returned HTTP %d", response.StatusCode)
	}
	models, err := decodeModels(connection.ProviderType, body)
	if err != nil {
		return nil, err
	}
	return models, nil
}

// Defaults keep a connection usable when a provider does not expose /models.
// Users can add deployment-specific identifiers from the UI.
func defaultModels(providerType string) ([]domain.ProviderModel, bool) {
	catalogs := map[string][]domain.ProviderModel{
		"openai": {
			{ModelID: "gpt-5.6-sol", DisplayName: "GPT 5.6 Sol", Available: true},
		},
		"anthropic": {
			{ModelID: "claude-opus-4-1", DisplayName: "Claude Opus 4.1", Available: true},
			{ModelID: "claude-sonnet-4", DisplayName: "Claude Sonnet 4", Available: true},
		},
		"google_gemini": {
			{ModelID: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro", Available: true},
			{ModelID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash", Available: true},
		},
		"xai": {
			{ModelID: "grok-4", DisplayName: "Grok 4", Available: true},
			{ModelID: "grok-3", DisplayName: "Grok 3", Available: true},
		},
		"deepseek": {
			{ModelID: "deepseek-chat", DisplayName: "DeepSeek Chat", Available: true},
			{ModelID: "deepseek-reasoner", DisplayName: "DeepSeek Reasoner", Available: true},
		},
		"alibaba_bailian": {
			{ModelID: "qwen-plus", DisplayName: "Qwen Plus", Available: true},
			{ModelID: "qwen3-coder-plus", DisplayName: "Qwen3 Coder Plus", Available: true},
		},
		"volcengine_ark": {
			{ModelID: "doubao-seed-1-6", DisplayName: "Doubao Seed 1.6", Available: true},
		},
		"moonshot": {
			{ModelID: "kimi-k2", DisplayName: "Kimi K2", Available: true},
		},
		"zhipu": {
			{ModelID: "glm-4.5", DisplayName: "GLM-4.5", Available: true},
			{ModelID: "glm-4.5-air", DisplayName: "GLM-4.5 Air", Available: true},
		},
		"minimax": {
			{ModelID: "MiniMax-M2.1", DisplayName: "MiniMax M2.1", Available: true},
		},
	}
	models, ok := catalogs[providerType]
	if !ok {
		return nil, false
	}
	result := make([]domain.ProviderModel, len(models))
	copy(result, models)
	return result, true
}

func withCompatibility(models []domain.ProviderModel, protocols []string) []domain.ProviderModel {
	for index := range models {
		models[index].Compatibility = domain.CompatibilityForProtocols(protocols)
	}
	return models
}

func modelsEndpoint(connection domain.ModelProviderConnection) (string, error) {
	parsed, err := url.Parse(connection.Endpoint)
	if err != nil {
		return "", fmt.Errorf("parse Provider Endpoint: %w", err)
	}
	base := strings.TrimSuffix(parsed.Path, "/")
	switch connection.ProviderType {
	case "anthropic":
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
	case "google_gemini":
		if !strings.HasSuffix(base, "/v1beta") && !strings.HasSuffix(base, "/v1") {
			base += "/v1beta"
		}
	}
	parsed.Path = path.Join(base, "models")
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func decodeModels(providerType string, body []byte) ([]domain.ProviderModel, error) {
	type modelItem struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		DisplayName string   `json:"display_name"`
		GeminiName  string   `json:"displayName"`
		Methods     []string `json:"supportedGenerationMethods"`
	}
	var payload struct {
		Data   []modelItem `json:"data"`
		Models []modelItem `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Provider Models: %w", err)
	}
	items := payload.Data
	if providerType == "google_gemini" {
		items = payload.Models
	}
	models := make([]domain.ProviderModel, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		identifier := strings.TrimSpace(item.ID)
		if identifier == "" {
			identifier = strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		}
		if identifier == "" {
			continue
		}
		if _, duplicate := seen[identifier]; duplicate {
			continue
		}
		seen[identifier] = struct{}{}
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(item.GeminiName)
		}
		if displayName == "" {
			displayName = identifier
		}
		models = append(models, domain.ProviderModel{ModelID: identifier, DisplayName: displayName, Available: true})
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Provider returned no models")
	}
	sort.Slice(models, func(i, j int) bool { return models[i].DisplayName < models[j].DisplayName })
	return models, nil
}
