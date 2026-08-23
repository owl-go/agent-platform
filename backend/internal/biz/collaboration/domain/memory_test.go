package domain

import (
	"testing"
	"time"
)

func TestDurableMemoryRejectsSensitiveOrSourceContent(t *testing.T) {
	now := time.Now().UTC()
	for _, content := range []string{
		"Store password acceptance-only-secret for later.",
		"Use token abc123 for deployments.",
		"SELECT * FROM users;",
		"Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456",
		"```go\nfunc main() {}\n```",
		"Preserve the private reasoning scratchpad.",
	} {
		if _, err := ProposeMemory("candidate", "agent", "task", content, now); err == nil {
			t.Fatalf("ProposeMemory accepted unsafe durable content %q", content)
		}
	}
}

func TestDurableMemoryAcceptsReviewedOperatingGuidance(t *testing.T) {
	if _, err := ProposeMemory("candidate", "agent", "task", "quality-gate:test:parser-regression", time.Now().UTC()); err != nil {
		t.Fatalf("ProposeMemory rejected operating guidance: %v", err)
	}
}
