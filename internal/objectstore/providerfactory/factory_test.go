package providerfactory_test

import (
	"testing"

	"agent-platform/internal/objectstore/memory"
	"agent-platform/internal/objectstore/providerfactory"
)

func TestNewSelectsConfiguredProvider(t *testing.T) {
	provider, err := providerfactory.New(providerfactory.Config{Provider: providerfactory.ProviderMemory})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, ok := provider.(*memory.Provider); !ok {
		t.Fatalf("provider type = %T, want memory provider", provider)
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	if _, err := providerfactory.New(providerfactory.Config{Provider: "s3"}); err == nil {
		t.Fatal("expected unsupported provider error")
	}
}
