package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutAllowsLongLivedEventStreams(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantTimeout time.Duration
	}{
		{name: "unary", path: "/api/v1/sessions", wantTimeout: time.Second},
		{name: "legacy Run SSE", path: "/v1/runs/00000000-0000-4000-8000-000000000001/events", wantTimeout: 30 * time.Minute},
		{name: "Session message SSE", path: "/api/v1/sessions/00000000-0000-4000-8000-000000000001/messages/2/events", wantTimeout: 30 * time.Minute},
		{name: "Workflow Run SSE", path: "/api/v1/workflows/00000000-0000-4000-8000-000000000001/runs/00000000-0000-4000-8000-000000000002/events", wantTimeout: 30 * time.Minute},
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
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, test.path, nil))
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
