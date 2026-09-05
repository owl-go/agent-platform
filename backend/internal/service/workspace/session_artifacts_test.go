package workspace

import (
	"testing"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

func TestArtifactResponseIncludesSessionMessageIdentityAndExpiry(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	response := artifactResponse(workspacedomain.Artifact{ID: "artifact-1", MessageID: 42, Kind: "file", Name: "report.md", Path: "report.md", Size: 12, ExpiresAt: &expiresAt})
	if response.MessageId != 42 || response.Name != "report.md" || response.Expired || response.ExpiresAt == nil {
		t.Fatalf("Artifact response = %#v", response)
	}
}
