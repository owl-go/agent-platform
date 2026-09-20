package workspace

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicVisitorCookieUsesCompatibleSameSiteMode(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	_, cookie, err := publicVisitor(request)
	if err != nil {
		t.Fatal(err)
	}
	if cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("HTTP cookie = %#v, want insecure Lax cookie", cookie)
	}

	secureRequest := httptest.NewRequest("GET", "https://example.com/", nil)
	secureRequest.TLS = &tls.ConnectionState{}
	_, secureCookie, err := publicVisitor(secureRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !secureCookie.Secure || secureCookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("HTTPS cookie = %#v, want Secure None cookie", secureCookie)
	}
}

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
