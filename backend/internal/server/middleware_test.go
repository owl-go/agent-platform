package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUnaryTimeoutDoesNotBoundRunEventSSE(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantDeadline bool
	}{
		{name: "unary", path: "/v1/runs", wantDeadline: true},
		{name: "sse", path: "/v1/runs/00000000-0000-4000-8000-000000000001/events", wantDeadline: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var found bool
			handler := unaryTimeoutFilter(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				_, found = request.Context().Deadline()
			}))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, test.path, nil))
			if found != test.wantDeadline {
				t.Fatalf("deadline found=%t, want %t", found, test.wantDeadline)
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
