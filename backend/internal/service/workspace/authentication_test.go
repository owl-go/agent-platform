package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/productanalytics"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestIssueWorkflowTokenCreates72HourJWT(t *testing.T) {
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
	if !expires.Equal(now.Add(72*time.Hour)) || claims.ExpiresAt != expires.Unix() {
		t.Fatalf("expiry = %v / %d", expires, claims.ExpiresAt)
	}
}

func TestOnlyExactTeambitionGETCallbackBypassesBearerAuthentication(t *testing.T) {
	filter, err := NewAuthenticationFilter(&accountapplication.Service{}, &workspaceapplication.Service{}, productanalytics.Nop{})
	if err != nil {
		t.Fatal(err)
	}
	handler := filter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		method, path string
		status       int
	}{{"GET", linearOAuthCallbackPath, 204}, {"POST", linearOAuthCallbackPath, 401}, {"GET", linearOAuthCallbackPath + "/other", 401}, {"GET", "/api/v1/connectors/linear/authorizations", 401}, {"GET", klingOAuthCallbackPath, 204}, {"POST", klingOAuthCallbackPath, 401}, {"GET", klingOAuthCallbackPath + "/other", 401}, {"GET", teambitionOAuthCallbackPath, 204}, {"POST", teambitionOAuthCallbackPath, 401}, {"GET", teambitionOAuthCallbackPath + "/other", 401}, {"GET", "/api/v1/connectors/teambition/authorizations", 401}} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, recorder.Code)
		}
	}
}
