package memory_test

import (
	"testing"

	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/conformance"
	"agent-platform/backend/internal/objectstore/memory"
)

func TestProviderConformance(t *testing.T) {
	conformance.Run(t, func(t *testing.T) objectstore.Provider {
		t.Helper()
		return memory.New()
	})
}
