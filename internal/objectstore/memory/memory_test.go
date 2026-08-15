package memory_test

import (
	"testing"

	"agent-platform/internal/objectstore"
	"agent-platform/internal/objectstore/conformance"
	"agent-platform/internal/objectstore/memory"
)

func TestProviderConformance(t *testing.T) {
	conformance.Run(t, func(t *testing.T) objectstore.Provider {
		t.Helper()
		return memory.New()
	})
}
