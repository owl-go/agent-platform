package camscannercli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOfficialHeadlessProtocolAndRefresh(t *testing.T) {
	status := "pending"
	expiry := time.Now().Add(time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-IS-AGENT") != "general" || !strings.HasPrefix(r.Header.Get("User-Agent"), "camscanner-cli/v1.1.8") || r.Header.Get("Content-Type") != "application/json" {
			t.Error("required native protocol headers missing")
			w.WriteHeader(400)
			return
		}
		switch r.URL.Path {
		case "/auth/user/code":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || body["client_id"] != "camscanner-cli" {
				t.Error("wrong client registration")
			}
			json.NewEncoder(w).Encode(map[string]any{"code": "provider-code", "expires_in": 300})
		case "/auth/user/status":
			if r.Method != "GET" || r.URL.Query().Get("code") != "provider-code" {
				t.Error("wrong poll")
			}
			json.NewEncoder(w).Encode(map[string]any{"status": status, "token": "short-token", "user_id": "owner", "is_domestic": "1", "expires_at": expiry})
		case "/auth/user/refresh":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if r.Method != "POST" || body["token"] != "short-token" {
				t.Error("wrong refresh")
			}
			json.NewEncoder(w).Encode(map[string]any{"token": "renewed-token", "expires_at": expiry})
		default:
			t.Error("unreviewed protocol path")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := NewClient()
	c.endpoint = server.URL
	challenge, err := c.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	action, _ := url.Parse(challenge.ActionURL)
	callback, _ := url.Parse(action.Query().Get("from"))
	if action.Scheme != "https" || action.Host != "www.camscanner.com" || action.Path != "/agent-auth" || callback.Host != "ai-tools.camscanner.com" || callback.Path != "/auth/user/callback" || callback.Query().Get("code") != "provider-code" {
		t.Fatal("wrong browser callback")
	}
	for _, s := range []string{"pending", "confirming", "expired", "cancelled", "unexpected"} {
		status = s
		_, err = c.Poll(context.Background(), challenge.State)
		if s == "pending" || s == "confirming" {
			if !errors.Is(err, ErrPending) {
				t.Fatal(err)
			}
		} else if err == nil {
			t.Fatal("invalid login accepted")
		}
	}
	status = "authorized"
	grant, err := c.Poll(context.Background(), challenge.State)
	if err != nil || grant.UserID != "owner" || grant.IsDomestic != "1" || grant.Token != "short-token" {
		t.Fatalf("grant: %v", err)
	}
	renewed, err := c.Refresh(context.Background(), grant.UserID, grant.Token)
	if err != nil || renewed.UserID != grant.UserID || renewed.Token != "renewed-token" {
		t.Fatalf("refresh: %v", err)
	}
}
func TestInvalidStateAndGrantFailBeforeNetwork(t *testing.T) {
	c := NewClient()
	c.endpoint = "http://127.0.0.1:1"
	for _, raw := range []string{"{}", `{"code":"invalid\ncode"}`, strings.Repeat("x", 4097)} {
		if _, err := c.Poll(context.Background(), raw); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	raw, _ := json.Marshal(pending{Code: "code", ExpiresAt: time.Now().Add(-time.Minute)})
	if _, err := c.Poll(context.Background(), string(raw)); !errors.Is(err, ErrExpired) {
		t.Fatal(err)
	}
	for _, token := range []string{"", "secret\nheader", strings.Repeat("a", 32769)} {
		if _, err := validateGrant(token, "user", time.Now().Add(time.Hour).Unix()); err == nil {
			t.Fatal("unsafe token")
		}
	}
	if _, err := validateGrant("token", "", time.Now().Add(time.Hour).Unix()); err == nil {
		t.Fatal("missing identity")
	}
}
func TestRedirectAndProviderErrorsDoNotExposeSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://attacker.invalid/?token=secret", 302)
	}))
	defer server.Close()
	c := NewClient()
	c.endpoint = server.URL
	_, err := c.Begin(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("unsafe redirect")
	}
}
