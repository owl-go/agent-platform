// Package githubcli implements the pinned GitHub CLI browser device flow.
package githubcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Public OAuth application identity from cli/cli v2.102.0 internal/authflow.
const ClientID = "178c6fc778ccc68e1d6a"

var ErrPending = errors.New("GitHub authorization pending")
var ErrDenied = errors.New("GitHub authorization denied")
var ErrExpired = errors.New("GitHub authorization expired")
var ErrConsumed = errors.New("GitHub device code exchanged; reconnect")
var ErrSlowDown = errors.New("GitHub requested slower polling")
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
var codePattern = regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)
var reviewedScopes = map[string]bool{"repo": true, "read:org": true, "gist": true, "project": true, "workflow": true}

type Client struct {
	HTTP       *http.Client
	oauth, api string
}
type Pending struct {
	DeviceCode string    `json:"device_code"`
	Scopes     []string  `json:"scopes"`
	ExpiresAt  time.Time `json:"expires_at"`
	NextPollAt time.Time `json:"next_poll_at"`
	Interval   int       `json:"interval"`
}
type Challenge struct {
	State, ActionURL string
	Scopes           []string
	ExpiresAt        time.Time
}
type Grant struct {
	AccessToken, ExternalID, DisplayName string
	Scopes                               []string
	ExpiresAt                            time.Time
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("GitHub authorization redirect rejected")
	}}, oauth: "https://github.com", api: "https://api.github.com"}
}
func (c *Client) call(ctx context.Context, method, endpoint string, form url.Values, token string, target any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return errors.New("invalid GitHub authorization request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Agent-Workspace-GitHub")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("GitHub authorization service unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub authorization rejected request (HTTP %d)", response.StatusCode)
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(target) != nil {
		return errors.New("invalid GitHub authorization response")
	}
	return nil
}
func (c *Client) Begin(ctx context.Context, requested []string) (Challenge, error) {
	scopes := []string{"repo", "read:org", "gist"}
	for _, s := range requested {
		if !reviewedScopes[s] {
			return Challenge{}, errors.New("unreviewed GitHub scope")
		}
		found := false
		for _, v := range scopes {
			if v == s {
				found = true
			}
		}
		if !found {
			scopes = append(scopes, s)
		}
	}
	var response struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
		URL        string `json:"verification_uri"`
		ExpiresIn  int    `json:"expires_in"`
		Interval   int    `json:"interval"`
	}
	if err := c.call(ctx, http.MethodPost, c.oauth+"/login/device/code", url.Values{"client_id": {ClientID}, "scope": {strings.Join(scopes, " ")}}, "", &response); err != nil {
		return Challenge{}, err
	}
	if response.URL != "https://github.com/login/device" || !codePattern.MatchString(response.UserCode) || len(response.DeviceCode) < 20 || len(response.DeviceCode) > 256 || !tokenPattern.MatchString(response.DeviceCode) || response.ExpiresIn < 1 || response.ExpiresIn > 900 || response.Interval < 1 || response.Interval > 60 {
		return Challenge{}, errors.New("invalid GitHub device challenge")
	}
	now := time.Now().UTC()
	state := Pending{DeviceCode: response.DeviceCode, Scopes: scopes, ExpiresAt: now.Add(time.Duration(response.ExpiresIn) * time.Second), NextPollAt: now.Add(time.Duration(response.Interval) * time.Second), Interval: response.Interval}
	encoded, _ := json.Marshal(state)
	// user_code is the public verification code, never the device credential.
	return Challenge{State: string(encoded), ActionURL: response.URL + "?user_code=" + url.QueryEscape(response.UserCode), Scopes: scopes, ExpiresAt: state.ExpiresAt}, nil
}
func parseState(raw string) (Pending, error) {
	var state Pending
	if len(raw) > 8192 || json.Unmarshal([]byte(raw), &state) != nil || len(state.DeviceCode) < 20 || !tokenPattern.MatchString(state.DeviceCode) || state.Interval < 1 || state.Interval > 900 {
		return state, errors.New("invalid GitHub device state")
	}
	if !state.ExpiresAt.After(time.Now()) {
		return state, ErrExpired
	}
	return state, nil
}

// ReservePoll is persisted with an owner-scoped CAS before contacting GitHub.
// Thirty seconds exceeds both bounded HTTP requests, preventing overlapping exchanges.
func ReservePoll(raw string) (string, error) {
	state, err := parseState(raw)
	if err != nil {
		return "", err
	}
	if time.Now().Before(state.NextPollAt) {
		return "", ErrPending
	}
	interval := max(30, state.Interval)
	state.NextPollAt = time.Now().UTC().Add(time.Duration(interval) * time.Second)
	encoded, _ := json.Marshal(state)
	return string(encoded), nil
}
func SlowPoll(raw string) (string, error) {
	state, err := parseState(raw)
	if err != nil {
		return "", err
	}
	state.Interval = min(900, state.Interval+5)
	state.NextPollAt = time.Now().UTC().Add(time.Duration(max(30, state.Interval)) * time.Second)
	encoded, _ := json.Marshal(state)
	return string(encoded), nil
}
func (c *Client) Poll(ctx context.Context, raw string) (Grant, error) {
	state, err := parseState(raw)
	if err != nil {
		return Grant{}, err
	}
	var response struct {
		Token     string `json:"access_token"`
		Type      string `json:"token_type"`
		Scope     string `json:"scope"`
		Error     string `json:"error"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := c.call(ctx, http.MethodPost, c.oauth+"/login/oauth/access_token", url.Values{"client_id": {ClientID}, "device_code": {state.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, "", &response); err != nil {
		return Grant{}, err
	}
	switch response.Error {
	case "authorization_pending":
		return Grant{}, ErrPending
	case "slow_down":
		return Grant{}, ErrSlowDown
	case "access_denied":
		return Grant{}, ErrDenied
	case "expired_token":
		return Grant{}, ErrExpired
	case "":
	default:
		return Grant{}, ErrDenied
	}
	if response.Token == "" || len(response.Token) > 32768 || !tokenPattern.MatchString(response.Token) || !strings.EqualFold(response.Type, "bearer") || response.ExpiresIn < 0 || response.ExpiresIn > 86400*365 {
		return Grant{}, ErrConsumed
	}
	scopes := strings.Fields(strings.ReplaceAll(response.Scope, ",", " "))
	for _, required := range state.Scopes {
		found := false
		for _, scope := range scopes {
			if scope == required {
				found = true
			}
		}
		if !found {
			return Grant{}, ErrConsumed
		}
	}
	var identity struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := c.call(ctx, http.MethodGet, c.api+"/user", nil, response.Token, &identity); err != nil {
		return Grant{}, errors.Join(ErrConsumed, err)
	}
	if identity.ID <= 0 || identity.Login == "" {
		return Grant{}, ErrConsumed
	}
	grant := Grant{AccessToken: response.Token, ExternalID: strconv.FormatInt(identity.ID, 10), DisplayName: identity.Login, Scopes: scopes}
	// The CLI application's default grant is non-expiring. Expiring grants require
	// reconnect at expiry; refresh material is never sent to the command process.
	if response.ExpiresIn > 0 {
		grant.ExpiresAt = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return grant, nil
}
