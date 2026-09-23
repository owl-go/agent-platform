package workspace

import (
	"encoding/json"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

func TestEmbeddingProviderPayloadUsesServerManagedVectorSettings(t *testing.T) {
	var payload embeddingProviderPayload
	if err := json.Unmarshal([]byte(`{"endpoint":"https://example.test/v1/embeddings","model":"embedding-model","dimensions":768,"enabled":false,"version":2}`), &payload); err != nil {
		t.Fatal(err)
	}
	configuration := payload.configuration()
	if configuration.Dimensions != domain.EmbeddingDimensions || !configuration.Enabled || configuration.Version != 2 {
		t.Fatalf("server-managed embedding configuration = %+v", configuration)
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("configuration.Validate() = %v", err)
	}
}
