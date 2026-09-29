package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutSelection(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantTimeout time.Duration
	}{
		{name: "unary", path: "/api/v1/sessions", wantTimeout: time.Second},
		{name: "Image Model verification", method: http.MethodPost, path: "/api/v1/admin/ai-creation/image-models/00000000-0000-4000-8000-000000000001/test", wantTimeout: 4 * time.Minute},
		{name: "Image Model read", method: http.MethodGet, path: "/api/v1/admin/ai-creation/image-models/00000000-0000-4000-8000-000000000001/test", wantTimeout: time.Second},
		{name: "Connector Package upload", method: http.MethodPost, path: "/api/v1/admin/connectors/packages", wantTimeout: 5 * time.Minute},
		{name: "Connector Package read", method: http.MethodGet, path: "/api/v1/admin/connectors/packages", wantTimeout: time.Second},
		{name: "similar Connector Package path", method: http.MethodPost, path: "/api/v1/admin/connectors/packages/", wantTimeout: time.Second},
		{name: "similar Image Model path", method: http.MethodPost, path: "/api/v1/admin/ai-creation/image-models/00000000-0000-4000-8000-000000000001/test/extra", wantTimeout: time.Second},
		{name: "legacy Run SSE", path: "/v1/runs/00000000-0000-4000-8000-000000000001/events", wantTimeout: 30 * time.Minute},
		{name: "Session message SSE", path: "/api/v1/sessions/00000000-0000-4000-8000-000000000001/messages/2/events", wantTimeout: 30 * time.Minute},
		{name: "Workflow Run SSE", path: "/api/v1/workflows/00000000-0000-4000-8000-000000000001/runs/00000000-0000-4000-8000-000000000002/events", wantTimeout: 30 * time.Minute},
		{name: "Image Generation SSE", path: "/api/v1/ai-creation/image-generations/00000000-0000-4000-8000-000000000001/events", wantTimeout: 30 * time.Minute},
		{name: "Assistant Turn SSE", method: http.MethodPost, path: "/api/v1/ai-apps/assistants/00000000-0000-4000-8000-000000000001/conversations/00000000-0000-4000-8000-000000000002/turns", wantTimeout: 30 * time.Minute},
		{name: "Assistant Turn cancellation", method: http.MethodPost, path: "/api/v1/ai-apps/assistants/00000000-0000-4000-8000-000000000001/conversations/00000000-0000-4000-8000-000000000002/turns/00000000-0000-4000-8000-000000000003/cancel", wantTimeout: time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var remaining time.Duration
			handler := unaryTimeoutFilter(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				deadline, found := request.Context().Deadline()
				if !found {
					t.Fatal("request context has no deadline")
				}
				remaining = time.Until(deadline)
			}))
			method := test.method
			if method == "" {
				method = http.MethodGet
			}
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, test.path, nil))
			if remaining < test.wantTimeout-time.Second || remaining > test.wantTimeout {
				t.Fatalf("remaining timeout=%s, want approximately %s", remaining, test.wantTimeout)
			}
		})
	}
}

func TestSecurityHeadersApplyToSharedHTTPListener(t *testing.T) {
	response := httptest.NewRecorder()
	securityHeadersFilter(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/runs/id/events", nil))
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers = %v", response.Header())
	}
}
