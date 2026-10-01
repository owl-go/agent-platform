package notioncli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testState(t *testing.T) string {
	t.Helper()
	raw, err := json.Marshal(pendingLogin{SessionID: strings.Repeat("a", 64), IssuedAt: time.Now().Unix()})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestBrowserLoginStartsAndCompletesWithoutProvidedToken(t *testing.T) {
	state := testState(t)
	login := &Login{run: func(_ context.Context, _ string, args []string, home string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "login --no-browser":
			if err := os.MkdirAll(filepath.Join(home, "cache"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, "cache", "pending-login.json"), []byte(state), 0600); err != nil {
				t.Fatal(err)
			}
			return []byte("Open https://app.notion.com/workers/cli-login?verificationCode=ABC-123 and confirm ABC-123"), nil
		case "login poll":
			if raw, err := os.ReadFile(filepath.Join(home, "cache", "pending-login.json")); err != nil || string(raw) != state {
				t.Fatalf("pending state = %q, %v", raw, err)
			}
			if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"workspace-1":"ntn_secret-value"}`), 0600); err != nil {
				t.Fatal(err)
			}
			return []byte("Login complete"), nil
		}
		t.Fatalf("unexpected CLI args %v", args)
		return nil, nil
	}}
	challenge, err := login.Begin(context.Background())
	if err != nil || challenge.State != state || challenge.ActionURL != "https://app.notion.com/workers/cli-login?verificationCode=ABC-123" {
		t.Fatalf("challenge = %#v, %v", challenge, err)
	}
	grant, err := login.Poll(context.Background(), challenge.State)
	if err != nil || grant.WorkspaceID != "workspace-1" || grant.Token != "ntn_secret-value" {
		t.Fatalf("grant = %#v, %v", grant, err)
	}
}

func TestBrowserLoginRejectsUntrustedURLAndState(t *testing.T) {
	login := &Login{run: func(_ context.Context, _ string, _ []string, home string) ([]byte, error) {
		_ = os.MkdirAll(filepath.Join(home, "cache"), 0700)
		_ = os.WriteFile(filepath.Join(home, "cache", "pending-login.json"), []byte(testState(t)), 0600)
		return []byte("https://attacker.example/workers/cli-login?verificationCode=ABC-123"), nil
	}}
	if _, err := login.Begin(context.Background()); err == nil {
		t.Fatal("accepted untrusted login URL")
	}
	if _, err := login.Poll(context.Background(), `{"sessionId":"x","issuedAt":1}`); err == nil || errors.Is(err, ErrPending) {
		t.Fatalf("accepted invalid pending state: %v", err)
	}
}

func TestBrowserLoginKeepsCompletedTokenWhenPollProcessTimesOut(t *testing.T) {
	login := &Login{run: func(_ context.Context, _ string, _ []string, home string) ([]byte, error) {
		if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"workspace-1":"ntn_secret-value"}`), 0600); err != nil {
			t.Fatal(err)
		}
		return nil, context.DeadlineExceeded
	}}
	grant, err := login.Poll(context.Background(), testState(t))
	if err != nil || grant.Token != "ntn_secret-value" {
		t.Fatalf("completed login was lost: %#v, %v", grant, err)
	}
}
