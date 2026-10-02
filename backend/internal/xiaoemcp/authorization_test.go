package xiaoemcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBrowserPKCERegistrationCallbackExchangeAndRefresh(t *testing.T) {
	var verifier string
	var tokenCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Error("OAuth must use POST")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/register":
			var input struct {
				RedirectURIs []string `json:"redirect_uris"`
				AuthMethod   string   `json:"token_endpoint_auth_method"`
			}
			if json.NewDecoder(r.Body).Decode(&input) != nil || len(input.RedirectURIs) != 1 || input.RedirectURIs[0] != "https://workspace.test/api/v1/connectors/xiaoe/oauth/callback" || input.AuthMethod != "none" {
				t.Error("invalid DCR")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "client-1", "redirect_uris": input.RedirectURIs, "token_endpoint_auth_method": "none"})
		case "/oauth/token":
			tokenCalls++
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("client_id") != "client-1" || r.Form.Get("resource") != Resource {
				t.Error("unbound token request")
			}
			if r.Form.Get("grant_type") == "authorization_code" {
				if r.Form.Get("code") != "one-time-code" || r.Form.Get("code_verifier") != verifier || r.Form.Get("redirect_uri") != "https://workspace.test/api/v1/connectors/xiaoe/oauth/callback" {
					t.Error("PKCE/code binding lost")
				}
			} else if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "refresh-1" {
				t.Error("invalid refresh")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1", "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": 3600})
		default:
			t.Error("unexpected endpoint")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := NewClient("https://workspace.test/api/v1/connectors/xiaoe/oauth/callback")
	client.endpoint = server.URL
	challenge, err := client.Begin(context.Background(), []string{"mcp", "offline_access"})
	if err != nil {
		t.Fatal(err)
	}
	var pending Pending
	if err := json.Unmarshal([]byte(challenge.State), &pending); err != nil {
		t.Fatal(err)
	}
	verifier = pending.Verifier
	action, _ := url.Parse(challenge.ActionURL)
	hash := sha256.Sum256([]byte(verifier))
	if action.Scheme+"://"+action.Host != Issuer || action.Query().Get("code_challenge") != base64.RawURLEncoding.EncodeToString(hash[:]) || action.Query().Get("code_challenge_method") != "S256" || strings.Contains(challenge.ActionURL, verifier) {
		t.Fatal("unsafe PKCE URL")
	}
	if _, err := client.Poll(context.Background(), challenge.State); !errors.Is(err, ErrPending) || tokenCalls != 0 {
		t.Fatal("pending flow exchanged a code")
	}
	received, err := AcceptCallback(challenge.State, "one-time-code", Issuer)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcceptCallback(received, "replayed", Issuer); err == nil {
		t.Fatal("accepted replay")
	}
	grant, err := client.Poll(context.Background(), received)
	if err != nil || grant.AccessToken != "access-1" || len(grant.Scopes) != 2 || !grant.ExpiresAt.After(time.Now()) {
		t.Fatalf("exchange failed: %v", err)
	}
	if _, err := client.Refresh(context.Background(), grant.ClientID, grant.RefreshToken); err != nil || tokenCalls != 2 {
		t.Fatalf("refresh: %v", err)
	}
}

func TestCallbackRejectsExpiredMalformedAndWrongIssuer(t *testing.T) {
	state := Pending{ClientID: "client", RedirectURI: "https://workspace.test/callback", Verifier: strings.Repeat("v", 43), ExpiresAt: time.Now().Add(time.Minute)}
	for _, test := range []struct {
		name, code, issuer string
		expired            bool
	}{
		{"issuer", "code", "https://attacker.test", false}, {"missing", "", Issuer, false}, {"header injection", "code\r\n", Issuer, false}, {"expired", "code", Issuer, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := state
			if test.expired {
				input.ExpiresAt = time.Now().Add(-time.Minute)
			}
			raw, _ := json.Marshal(input)
			if _, err := AcceptCallback(string(raw), test.code, test.issuer); err == nil {
				t.Fatal("accepted invalid callback")
			}
		})
	}
	if _, err := AcceptCallback("{}", "code", Issuer); err == nil {
		t.Fatal("accepted malformed state")
	}
	client := NewClient("https://other.test/callback")
	raw, _ := json.Marshal(state)
	if _, err := client.Poll(context.Background(), string(raw)); err == nil || errors.Is(err, ErrPending) {
		t.Fatal("accepted redirected state")
	}
}

func TestBeginRejectsUnreviewedScopesAndNonHTTPSCallback(t *testing.T) {
	for _, test := range []struct {
		redirect string
		scopes   []string
	}{{"http://workspace.test/callback", nil}, {"https://workspace.test/callback", []string{"project:write"}}, {"https://workspace.test/callback?override=1", nil}} {
		if _, err := NewClient(test.redirect).Begin(context.Background(), test.scopes); err == nil {
			t.Fatal("accepted invalid flow")
		}
	}
}

func TestProviderErrorsAndRedirectsDoNotExposeAuthorizationMaterial(t *testing.T) {
	for _, status := range []int{400, 302, 200, 566} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://attacker.test/refresh-canary")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":"refresh-canary","access_token":"access-canary","token_type":"Bearer","expires_in":0}`))
			}))
			defer server.Close()
			client := NewClient("https://workspace.test/callback")
			client.endpoint = server.URL
			_, err := client.Refresh(context.Background(), "client", "refresh-canary")
			if err == nil || strings.Contains(err.Error(), "canary") {
				t.Fatal("provider error leaked credentials or accepted invalid grant")
			}
		})
	}
}

func TestTokenScopeExpansionIsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":60,"scope":"unreviewed:write"}`))
	}))
	defer server.Close()
	client := NewClient("https://workspace.test/callback")
	client.endpoint = server.URL
	if _, err := client.exchange(context.Background(), url.Values{}, "client", []string{"mcp"}); err == nil {
		t.Fatal("accepted expanded scope")
	}
}

func TestRegistrationBlockIsActionableAndDoesNotExposeProviderBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(566)
		_, _ = w.Write([]byte("secret-canary"))
	}))
	defer server.Close()
	client := NewClient("https://workspace.test/callback")
	client.endpoint = server.URL
	if _, err := client.Begin(context.Background(), nil); !errors.Is(err, ErrCallbackBlocked) || strings.Contains(err.Error(), "canary") {
		t.Fatal("registration policy block was hidden or leaked provider body")
	}
}
