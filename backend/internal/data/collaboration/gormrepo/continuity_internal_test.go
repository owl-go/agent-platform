package gormrepo

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/collaboration/domain"
)

func TestBoundedSessionMemoryAlwaysFitsRuntimeContinuityBudget(t *testing.T) {
	values := make([]string, 200)
	for index := range values {
		values[index] = strings.Repeat("x", 4_000)
	}
	memory := domain.SessionMemory{
		Summary: strings.Repeat("<", 20_000), ConfirmedDecisions: values,
		Results: values, WorkspaceSnapshots: values,
	}
	encoded, err := boundedSessionMemory(memory, 50_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 50_000 {
		t.Fatalf("bounded Session Memory has %d bytes", len(encoded))
	}
	var projected domain.SessionMemory
	if err := json.Unmarshal(encoded, &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Summary != memory.Summary {
		t.Fatal("bounded Session Memory lost its summary")
	}
}
