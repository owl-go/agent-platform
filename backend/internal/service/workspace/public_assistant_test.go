package workspace

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-platform/backend/internal/platformconfig"
)

func TestPublicVisitorCookieUsesCompatibleSameSiteMode(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	service := &Service{}
	_, cookie, err := service.publicVisitor(request)
	if err != nil {
		t.Fatal(err)
	}
	if cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("HTTP cookie = %#v, want insecure Lax cookie", cookie)
	}

	secureRequest := httptest.NewRequest("GET", "https://example.com/", nil)
	secureRequest.TLS = &tls.ConnectionState{}
	_, secureCookie, err := service.publicVisitor(secureRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !secureCookie.Secure || secureCookie.SameSite != http.SameSiteNoneMode || !secureCookie.Partitioned {
		t.Fatalf("HTTPS cookie = %#v, want Secure None cookie", secureCookie)
	}
}

func TestPublicVisitorCookieUsesConfiguredHTTPSWithoutTrustingRequestHeaders(t *testing.T) {
	for _, test := range []struct {
		name, redirect, forwarded string
		secure                    bool
	}{
		{"HTTPS edge", "https://platform.example.test/auth/callback", "http", true},
		{"HTTP development", "http://localhost:5173/auth/callback", "https", false},
		{"unconfigured", "", "https", false},
		{"invalid URL", "https://%/callback", "https", false},
		{"missing host", "https:/callback", "https", false},
		{"credentials", "https://user:pass@example.test/callback", "https", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &Service{config: platformconfig.Config{Authentication: platformconfig.AuthenticationConfig{RedirectURI: test.redirect}}}
			request := httptest.NewRequest(http.MethodGet, "http://api.internal/", nil)
			request.Header.Set("X-Forwarded-Proto", test.forwarded)
			request.Header.Set("Forwarded", "proto=https;host=forged.example.test")
			issuedHash, cookie, err := service.publicVisitor(request)
			if err != nil || cookie == nil || cookie.Secure != test.secure || cookie.Partitioned != test.secure || !cookie.HttpOnly || cookie.Valid() != nil {
				t.Fatalf("unexpected visitor cookie policy: err=%v", err)
			}
			wantSameSite := http.SameSiteLaxMode
			if test.secure {
				wantSameSite = http.SameSiteNoneMode
			}
			if cookie.SameSite != wantSameSite {
				t.Fatal("unexpected SameSite policy")
			}
			resumed := httptest.NewRequest(http.MethodGet, "http://api.internal/", nil)
			resumed.AddCookie(cookie)
			resumedHash, renewed, err := service.publicVisitor(resumed)
			if err != nil || renewed != nil || issuedHash != resumedHash {
				t.Fatal("visitor identity was not retained")
			}
		})
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
