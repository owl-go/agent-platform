package workspace

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorkflowCredentialAndTokenRoutesAreSeparated(t *testing.T) {
	if id, ok := workflowCredentialRoute("POST", "/api/v1/workflows/workflow-1/api-token"); !ok || id != "workflow-1" {
		t.Fatalf("credential route = %q, %v", id, ok)
	}
	if _, ok := workflowCredentialRoute("POST", "/api/v1/workflows/workflow-1/runs"); ok {
		t.Fatal("Basic credentials must not invoke a Workflow directly")
	}
	if id, ok := workflowTokenRoute("POST", "/api/v1/workflows/workflow-1/runs"); !ok || id != "workflow-1" {
		t.Fatalf("token route = %q, %v", id, ok)
	}
	if _, ok := workflowTokenRoute("GET", "/api/v1/me"); ok {
		t.Fatal("Workflow token must not access account APIs")
	}
}

func TestIssueWorkflowTokenCreatesShortLivedJWT(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	token, expires, err := issueWorkflowToken(workflowCredentialContext{WorkflowID: "workflow-1", OwnerID: "user-1", APIKey: "awk_key", SecretHash: "hash"}, now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims workflowTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Audience != "agent-platform-workflow" || claims.WorkflowID != "workflow-1" || claims.OwnerID != "user-1" {
		t.Fatalf("claims = %#v", claims)
	}
	if !expires.Equal(now.Add(15*time.Minute)) || claims.ExpiresAt != expires.Unix() {
		t.Fatalf("expiry = %v / %d", expires, claims.ExpiresAt)
	}
}
