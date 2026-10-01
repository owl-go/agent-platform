package notioncli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Login uses the pinned official ntn binary's documented no-browser flow.
// Its temporary CLI state is encrypted by the caller between HTTP requests.
type Login struct {
	Binary string
	run    func(context.Context, string, []string, string) ([]byte, error)
}

type Challenge struct {
	State     string
	ActionURL string
	ExpiresAt time.Time
}

type Grant struct {
	WorkspaceID string
	Token       string
}

var (
	ErrPending = errors.New("Notion login is pending")
	ErrExpired = errors.New("Notion login expired")
	loginURL   = regexp.MustCompile(`https://[^\s]+`)
	code       = regexp.MustCompile(`^[A-Za-z0-9-]{5,32}$`)
	sessionID  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type pendingLogin struct {
	SessionID string `json:"sessionId"`
	IssuedAt  int64  `json:"issuedAt"`
}

func NewLogin() *Login { return &Login{Binary: "/usr/local/bin/ntn-auth"} }

func (login *Login) execute(ctx context.Context, args []string, home string) ([]byte, error) {
	if login.run != nil {
		return login.run(ctx, login.Binary, args, home)
	}
	command := exec.CommandContext(ctx, login.Binary, args...)
	command.Env = []string{"HOME=" + home, "NOTION_HOME=" + home, "NOTION_KEYRING=0", "PATH=/usr/bin:/bin"}
	command.Dir = home
	return command.CombinedOutput()
}

func temporaryHome() (string, func(), error) {
	home, err := os.MkdirTemp("", "notion-login-")
	if err != nil {
		return "", nil, err
	}
	return home, func() { _ = os.RemoveAll(home) }, nil
}

func validatePending(raw []byte) (pendingLogin, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return pendingLogin{}, errors.New("Notion login state has invalid size")
	}
	var state pendingLogin
	if err := json.Unmarshal(raw, &state); err != nil || !sessionID.MatchString(state.SessionID) || state.IssuedAt <= 0 || state.IssuedAt > time.Now().Add(time.Minute).Unix() {
		return pendingLogin{}, errors.New("Notion login state is invalid")
	}
	return state, nil
}

func (login *Login) Begin(ctx context.Context) (Challenge, error) {
	home, cleanup, err := temporaryHome()
	if err != nil {
		return Challenge{}, err
	}
	defer cleanup()
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := login.execute(bounded, []string{"login", "--no-browser"}, home)
	if err != nil {
		return Challenge{}, fmt.Errorf("start Notion login: %w", err)
	}
	var actionURL string
	for _, candidate := range loginURL.FindAllString(string(output), -1) {
		parsed, parseErr := url.Parse(strings.TrimRight(candidate, ".,;"))
		if parseErr == nil && parsed.Scheme == "https" && parsed.Host == "app.notion.com" && parsed.Path == "/workers/cli-login" && code.MatchString(parsed.Query().Get("verificationCode")) {
			actionURL = parsed.String()
			break
		}
	}
	if actionURL == "" {
		return Challenge{}, errors.New("Notion login did not return a valid authorization URL")
	}
	raw, err := os.ReadFile(filepath.Join(home, "cache", "pending-login.json"))
	if err != nil {
		return Challenge{}, fmt.Errorf("read Notion login state: %w", err)
	}
	state, err := validatePending(raw)
	if err != nil {
		return Challenge{}, err
	}
	return Challenge{State: string(raw), ActionURL: actionURL, ExpiresAt: time.Unix(state.IssuedAt, 0).Add(10 * time.Minute)}, nil
}

func (login *Login) Poll(ctx context.Context, opaqueState string) (Grant, error) {
	state, err := validatePending([]byte(opaqueState))
	if err != nil {
		return Grant{}, err
	}
	if time.Since(time.Unix(state.IssuedAt, 0)) > 10*time.Minute {
		return Grant{}, ErrExpired
	}
	home, cleanup, err := temporaryHome()
	if err != nil {
		return Grant{}, err
	}
	defer cleanup()
	if err := os.MkdirAll(filepath.Join(home, "cache"), 0700); err != nil {
		return Grant{}, err
	}
	if err := os.WriteFile(filepath.Join(home, "cache", "pending-login.json"), []byte(opaqueState), 0600); err != nil {
		return Grant{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	output, runErr := login.execute(bounded, []string{"login", "poll"}, home)
	if raw, readErr := os.ReadFile(filepath.Join(home, "auth.json")); readErr == nil {
		return parseLoginGrant(raw)
	}
	if runErr != nil {
		if bounded.Err() == context.DeadlineExceeded {
			return Grant{}, ErrPending
		}
		if strings.Contains(strings.ToLower(string(output)), "expired") {
			return Grant{}, ErrExpired
		}
		return Grant{}, fmt.Errorf("complete Notion login: %w", runErr)
	}
	return Grant{}, errors.New("Notion login did not store a usable credential")
}

func parseLoginGrant(raw []byte) (Grant, error) {
	if len(raw) == 0 || len(raw) > 64*1024 {
		return Grant{}, errors.New("Notion login did not store a usable credential")
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Grant{}, errors.New("Notion login credential has invalid format")
	}
	var grant Grant
	for workspaceID, value := range entries {
		var token string
		if json.Unmarshal(value, &token) == nil && (strings.HasPrefix(token, "ntn_") || strings.HasPrefix(token, "development_ntn_")) && len(token) >= 8 && len(token) <= 4096 {
			if grant.Token != "" {
				return Grant{}, errors.New("Notion login returned multiple workspaces")
			}
			grant = Grant{WorkspaceID: workspaceID, Token: token}
		}
	}
	if grant.Token == "" || grant.WorkspaceID == "" {
		return Grant{}, errors.New("Notion login credential is missing")
	}
	return grant, nil
}
