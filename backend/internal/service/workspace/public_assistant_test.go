package workspace

import (
	"net/http/httptest"
	"testing"
)

func TestPublicRemoteHashNormalizesHostPort(t *testing.T) {
	request := httptest.NewRequest("POST", "/", nil)
	request.RemoteAddr = "203.0.113.10:43122"
	if got, want := publicRemoteHash(request), publicHash("203.0.113.10"); got != want {
		t.Fatalf("remote hash = %q, want %q", got, want)
	}
}

func TestPublicRemoteHashDoesNotTrustForwardedHeaders(t *testing.T) {
	request := httptest.NewRequest("POST", "/", nil)
	request.RemoteAddr = "203.0.113.10:43122"
	request.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got, want := publicRemoteHash(request), publicHash("203.0.113.10"); got != want {
		t.Fatalf("remote hash = %q, want %q", got, want)
	}
}
