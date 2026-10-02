package githubcli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeviceFlowProtocolAndPolling(t *testing.T) {
	mode := "pending"
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") == "" {
			t.Error("missing required headers")
		}
		switch r.URL.Path {
		case "/login/device/code":
			if r.Method != "POST" || r.ParseForm() != nil || r.Form.Get("client_id") != ClientID || r.Form.Get("scope") != "repo read:org gist project" {
				t.Error("device request mismatch")
			}
			_, _ = w.Write([]byte(`{"device_code":"abcdefghijklmnopqrstuvwxyz12345678901234","user_code":"ABCD-1234","verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`))
		case "/login/oauth/access_token":
			if r.Method != "POST" || r.ParseForm() != nil || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" || r.Form.Get("client_id") != ClientID || r.Form.Get("device_code") == "" || r.Header.Get("Authorization") != "" {
				t.Error("exchange request mismatch")
			}
			switch mode {
			case "pending":
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			case "slow":
				_, _ = w.Write([]byte(`{"error":"slow_down"}`))
			case "denied":
				_, _ = w.Write([]byte(`{"error":"access_denied"}`))
			case "expired":
				_, _ = w.Write([]byte(`{"error":"expired_token"}`))
			default:
				_, _ = w.Write([]byte(`{"access_token":"gho_canary","token_type":"bearer","scope":"repo,read:org,gist,project"}`))
			}
		case "/user":
			if r.Header.Get("Authorization") != "Bearer gho_canary" || r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
				t.Error("identity request mismatch")
			}
			if mode == "identity-fail" {
				w.WriteHeader(500)
				return
			}
			_, _ = w.Write([]byte(`{"id":42,"login":"octocat"}`))
		default:
			t.Error("unexpected route")
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := NewClient()
	c.oauth = server.URL
	c.api = server.URL
	challenge, err := c.Begin(context.Background(), []string{"project"})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.ActionURL != "https://github.com/login/device?user_code=ABCD-1234" || strings.Contains(challenge.ActionURL, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatal("device credential exposed")
	}
	if _, err = ReservePoll(challenge.State); !errors.Is(err, ErrPending) {
		t.Fatal("initial polling interval ignored")
	}
	var state Pending
	_ = json.Unmarshal([]byte(challenge.State), &state)
	state.NextPollAt = time.Now().Add(-time.Minute)
	raw, _ := json.Marshal(state)
	reserved, err := ReservePoll(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReservePoll(reserved); !errors.Is(err, ErrPending) {
		t.Fatal("concurrent poll allowed")
	}
	for _, row := range []struct {
		mode     string
		expected error
	}{{"pending", ErrPending}, {"slow", ErrSlowDown}, {"denied", ErrDenied}, {"expired", ErrExpired}, {"identity-fail", ErrConsumed}} {
		mode = row.mode
		if _, e := c.Poll(context.Background(), reserved); !errors.Is(e, row.expected) {
			t.Fatalf("%s: %v", mode, e)
		}
	}
	slowed, e := SlowPoll(reserved)
	if e != nil {
		t.Fatal(e)
	}
	_ = json.Unmarshal([]byte(slowed), &state)
	if state.Interval != 10 {
		t.Fatal("slow down ignored")
	}
	mode = "ok"
	grant, e := c.Poll(context.Background(), reserved)
	if e != nil || grant.ExternalID != "42" || grant.DisplayName != "octocat" || !grant.ExpiresAt.IsZero() {
		t.Fatalf("grant: %+v %v", grant, e)
	}
	before := requests
	if _, e := c.Begin(context.Background(), []string{"admin:org"}); e == nil || requests != before {
		t.Fatal("unreviewed scope reached upstream")
	}
	state.ExpiresAt = time.Now().Add(-time.Minute)
	raw, _ = json.Marshal(state)
	if _, e := c.Poll(context.Background(), string(raw)); !errors.Is(e, ErrExpired) || requests != before {
		t.Fatal("expired state reached upstream")
	}
}
func TestRejectUnsafeChallengeAndRedirect(t *testing.T) {
	for _, uri := range []string{"https://evil.test/login/device", "https://github.com:443/login/device", "https://github.com/login/device?token=x"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"device_code": strings.Repeat("a", 40), "user_code": "ABCD-1234", "verification_uri": uri, "expires_in": 900, "interval": 5})
		}))
		c := NewClient()
		c.oauth = server.URL
		if _, e := c.Begin(context.Background(), nil); e == nil {
			t.Errorf("accepted unsafe challenge %s", uri)
		}
		server.Close()
	}
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++ }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	c := NewClient()
	c.oauth = server.URL
	if _, e := c.Begin(context.Background(), nil); e == nil || targetCalls != 0 {
		t.Fatal("followed OAuth redirect")
	}
}
