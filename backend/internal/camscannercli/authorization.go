// Package camscannercli adapts the pinned CLI's official headless browser login.
package camscannercli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Endpoint = "https://ai-tools.camscanner.com"
const LoginURL = "https://www.camscanner.com/agent-auth"

var ErrPending = errors.New("CamScanner authorization pending")
var ErrExpired = errors.New("CamScanner authorization expired")
var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/-]+={0,}$`)

type Client struct {
	HTTP     *http.Client
	endpoint string
}
type pending struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Challenge struct {
	State, ActionURL string
	ExpiresAt        time.Time
}
type Grant struct {
	Token, UserID, IsDomestic string
	ExpiresAt                 time.Time
}

func NewClient() *Client {
	return &Client{endpoint: Endpoint, HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("CamScanner redirect rejected") }}}
}
func (c *Client) call(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, body)
	if err != nil {
		return errors.New("invalid CamScanner request")
	}
	req.Header.Set("X-IS-AGENT", "general")
	req.Header.Set("User-Agent", "camscanner-cli/v1.1.8 (linux/amd64)")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("CamScanner authorization service unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("CamScanner authorization rejected request (HTTP %d)", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(out); err != nil {
		return errors.New("invalid CamScanner authorization response")
	}
	return nil
}
func (c *Client) Begin(ctx context.Context) (Challenge, error) {
	var result struct {
		Code      string `json:"code"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := c.call(ctx, http.MethodPost, "/auth/user/code", strings.NewReader(`{"client_id":"camscanner-cli"}`), &result); err != nil {
		return Challenge{}, err
	}
	if result.Code == "" || len(result.Code) > 512 || !tokenPattern.MatchString(result.Code) || result.ExpiresIn <= 0 || result.ExpiresIn > 600 {
		return Challenge{}, errors.New("invalid CamScanner login code")
	}
	state := pending{Code: result.Code, ExpiresAt: time.Now().UTC().Add(time.Duration(result.ExpiresIn) * time.Second)}
	raw, _ := json.Marshal(state)
	// The original CLI encodes the provider callback as the browser's `from` URL.
	callback := Endpoint + "/auth/user/callback?code=" + url.QueryEscape(result.Code)
	return Challenge{State: string(raw), ActionURL: LoginURL + "?" + url.Values{"from": {callback}}.Encode(), ExpiresAt: state.ExpiresAt}, nil
}
func (c *Client) Poll(ctx context.Context, raw string) (Grant, error) {
	var state pending
	if len(raw) > 4096 || json.Unmarshal([]byte(raw), &state) != nil || state.Code == "" || !tokenPattern.MatchString(state.Code) {
		return Grant{}, errors.New("invalid CamScanner login state")
	}
	if !state.ExpiresAt.After(time.Now()) {
		return Grant{}, ErrExpired
	}
	var result struct {
		Status     string `json:"status"`
		Token      string `json:"token"`
		UserID     string `json:"user_id"`
		ExpiresAt  int64  `json:"expires_at"`
		IsDomestic string `json:"is_domestic"`
	}
	if err := c.call(ctx, http.MethodGet, "/auth/user/status?code="+url.QueryEscape(state.Code), nil, &result); err != nil {
		return Grant{}, err
	}
	switch result.Status {
	case "pending", "confirming":
		return Grant{}, ErrPending
	case "expired", "cancelled":
		return Grant{}, ErrExpired
	case "authorized":
		v, e := validateGrant(result.Token, result.UserID, result.ExpiresAt)
		if len(result.IsDomestic) > 8 || strings.ContainsAny(result.IsDomestic, "\r\n\x00") {
			return Grant{}, errors.New("invalid CamScanner account region")
		}
		v.IsDomestic = result.IsDomestic
		return v, e
	default:
		return Grant{}, errors.New("unknown CamScanner login status")
	}
}
func (c *Client) Refresh(ctx context.Context, userID, token string) (Grant, error) {
	if token == "" || len(token) > 32768 || !tokenPattern.MatchString(token) {
		return Grant{}, errors.New("invalid CamScanner renewal credential")
	}
	body, _ := json.Marshal(map[string]string{"token": token})
	var result struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := c.call(ctx, http.MethodPost, "/auth/user/refresh", strings.NewReader(string(body)), &result); err != nil {
		return Grant{}, err
	}
	return validateGrant(result.Token, userID, result.ExpiresAt)
}
func validateGrant(token, userID string, expires int64) (Grant, error) {
	if token == "" || len(token) > 32768 || !tokenPattern.MatchString(token) || userID == "" || len(userID) > 512 || strings.ContainsAny(userID, "\r\n\x00") || expires <= time.Now().Add(time.Minute).Unix() || expires > time.Now().Add(365*24*time.Hour).Unix() {
		return Grant{}, errors.New("invalid CamScanner grant")
	}
	return Grant{Token: token, UserID: userID, ExpiresAt: time.Unix(expires, 0).UTC()}, nil
}
