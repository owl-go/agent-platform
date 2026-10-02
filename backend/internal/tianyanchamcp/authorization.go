// Package tianyanchamcp adapts the official MCP OAuth + PKCE protocol to a remote Workspace.
package tianyanchamcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

const Issuer = "https://capi.tianyancha.com/oauth"
const Resource = "https://mcp.tianyancha.com/mcp"

var bearerTokenPattern = regexp.MustCompile(`^[A-Za-z0-9._~+/-]+={0,}$`)

var ErrRegionBlocked = errors.New("Tianyancha does not support the deployment region")

var ErrPending = errors.New("Tianyancha authorization pending")
var ErrExpired = errors.New("Tianyancha authorization expired")

type Client struct {
	HTTP        *http.Client
	RedirectURI string
	endpoint    string
}
type Pending struct {
	ClientID    string    `json:"client_id"`
	RedirectURI string    `json:"redirect_uri"`
	Verifier    string    `json:"verifier"`
	Code        string    `json:"code,omitempty"`
	Scopes      []string  `json:"scopes"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type Challenge struct {
	State, ActionURL string
	Scopes           []string
	ExpiresAt        time.Time
}
type Grant struct {
	AccessToken, RefreshToken, ClientID string
	Scopes                              []string
	ExpiresAt, RefreshExpiresAt         time.Time
}

func NewClient(redirect string) *Client {
	return &Client{RedirectURI: redirect, endpoint: Issuer, HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("OAuth redirect rejected") }}}
}
func (c *Client) call(ctx context.Context, path, contentType string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, body)
	if err != nil {
		return errors.New("invalid OAuth request")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	response, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("Tianyancha OAuth service unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode == 419 {
		var blocked struct {
			ErrorCode int    `json:"errorCode"`
			Message   string `json:"message"`
		}
		if json.NewDecoder(io.LimitReader(response.Body, 2048)).Decode(&blocked) == nil && blocked.ErrorCode == 301000 && blocked.Message == "bannedLocation" {
			return ErrRegionBlocked
		}
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		return fmt.Errorf("Tianyancha OAuth rejected request (HTTP %d)", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(out); err != nil {
		return errors.New("invalid Tianyancha OAuth response")
	}
	return nil
}
func (c *Client) Begin(ctx context.Context, scopes []string) (Challenge, error) {
	u, err := url.Parse(c.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Challenge{}, errors.New("Tianyancha requires configured HTTPS callback")
	}
	if len(scopes) == 0 {
		scopes = []string{"mcp:tools.call"}
	}
	allowed := map[string]bool{"mcp:tools.call": true}
	for _, scope := range scopes {
		if !allowed[scope] {
			return Challenge{}, errors.New("unreviewed Tianyancha OAuth scope")
		}
	}
	var registered struct {
		ClientID     string   `json:"client_id"`
		RedirectURIs []string `json:"redirect_uris"`
		AuthMethod   string   `json:"token_endpoint_auth_method"`
	}
	body, _ := json.Marshal(map[string]any{"client_name": "Agent Workspace Tianyancha", "redirect_uris": []string{c.RedirectURI}, "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}, "token_endpoint_auth_method": "none"})
	if err := c.call(ctx, "/register", "application/json", strings.NewReader(string(body)), &registered); err != nil {
		return Challenge{}, err
	}
	if registered.ClientID == "" || len(registered.ClientID) > 512 || len(registered.RedirectURIs) != 1 || registered.RedirectURIs[0] != c.RedirectURI || registered.AuthMethod != "none" {
		return Challenge{}, errors.New("Tianyancha OAuth client registration mismatch")
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return Challenge{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(verifier))
	state := Pending{ClientID: registered.ClientID, RedirectURI: c.RedirectURI, Verifier: verifier, ExpiresAt: time.Now().UTC().Add(10 * time.Minute), Scopes: append([]string(nil), scopes...)}
	encoded, _ := json.Marshal(state)
	q := url.Values{"client_id": {state.ClientID}, "redirect_uri": {c.RedirectURI}, "response_type": {"code"}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "resource": {Resource}, "scope": {strings.Join(scopes, " ")}}
	return Challenge{State: string(encoded), ActionURL: Issuer + "/authorize?" + q.Encode(), Scopes: scopes, ExpiresAt: state.ExpiresAt}, nil
}
func AcceptCallback(raw, code, issuer string) (string, error) {
	var state Pending
	if len(raw) > 8192 || json.Unmarshal([]byte(raw), &state) != nil || state.ClientID == "" || len(state.Verifier) != 43 || state.RedirectURI == "" {
		return "", errors.New("invalid Tianyancha OAuth state")
	}
	if !state.ExpiresAt.After(time.Now()) {
		return "", ErrExpired
	}
	if state.Code != "" || code == "" || len(code) > 4096 || strings.ContainsAny(code, "\r\n\x00") || (issuer != "" && issuer != Issuer) {
		return "", errors.New("invalid Tianyancha OAuth callback")
	}
	state.Code = code
	encoded, _ := json.Marshal(state)
	return string(encoded), nil
}
func (c *Client) Poll(ctx context.Context, raw string) (Grant, error) {
	var state Pending
	if len(raw) > 8192 || json.Unmarshal([]byte(raw), &state) != nil || state.RedirectURI != c.RedirectURI || state.ClientID == "" || len(state.Verifier) != 43 {
		return Grant{}, errors.New("invalid Tianyancha OAuth state")
	}
	if !state.ExpiresAt.After(time.Now()) {
		return Grant{}, ErrExpired
	}
	if state.Code == "" {
		return Grant{}, ErrPending
	}
	return c.exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {state.ClientID}, "redirect_uri": {state.RedirectURI}, "code": {state.Code}, "code_verifier": {state.Verifier}, "resource": {Resource}}, state.ClientID, state.Scopes)
}
func (c *Client) Refresh(ctx context.Context, id, refresh string) (Grant, error) {
	if id == "" || refresh == "" {
		return Grant{}, errors.New("Tianyancha must be reconnected")
	}
	return c.exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {id}, "refresh_token": {refresh}, "resource": {Resource}}, id, nil)
}
func (c *Client) exchange(ctx context.Context, form url.Values, id string, requestedScopes []string) (Grant, error) {
	var result struct {
		AccessToken      string `json:"access_token"`
		RefreshToken     string `json:"refresh_token"`
		TokenType        string `json:"token_type"`
		Scope            string `json:"scope"`
		ExpiresIn        int64  `json:"expires_in"`
		RefreshExpiresIn int64  `json:"refresh_expires_in"`
	}
	if err := c.call(ctx, "/token", "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), &result); err != nil {
		return Grant{}, err
	}
	if result.AccessToken == "" || len(result.AccessToken) > 32768 || !bearerTokenPattern.MatchString(result.AccessToken) || len(result.RefreshToken) > 32768 || (result.RefreshToken != "" && !bearerTokenPattern.MatchString(result.RefreshToken)) || !strings.EqualFold(result.TokenType, "Bearer") || result.ExpiresIn <= 0 || result.ExpiresIn > 86400*365 {
		return Grant{}, errors.New("invalid Tianyancha OAuth grant")
	}
	scopes := strings.Fields(result.Scope)
	if len(scopes) == 0 {
		scopes = []string{"mcp:tools.call"}
	}
	for _, scope := range scopes {
		if scope != "mcp:tools.call" {
			return Grant{}, errors.New("unreviewed Tianyancha grant scope")
		}
		if len(requestedScopes) > 0 {
			found := false
			for _, requested := range requestedScopes {
				if scope == requested {
					found = true
				}
			}
			if !found {
				return Grant{}, errors.New("unexpected Tianyancha grant scope")
			}
		}
	}
	now := time.Now().UTC()
	grant := Grant{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken, ClientID: id, Scopes: scopes, ExpiresAt: now.Add(time.Duration(result.ExpiresIn) * time.Second)}
	if result.RefreshExpiresIn > 0 && result.RefreshExpiresIn <= 86400*365 {
		grant.RefreshExpiresAt = now.Add(time.Duration(result.RefreshExpiresIn) * time.Second)
	}
	return grant, nil
}
