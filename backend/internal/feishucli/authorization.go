package feishucli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	deviceAuthorizationPath = "/oauth/v1/device_authorization"
	oauthTokenPath          = "/open-apis/authen/v2/oauth/token"
	userInfoPath            = "/open-apis/authen/v1/user_info"
)

type AuthorizationRequest struct {
	DeviceCode string
	ActionURL  string
	ExpiresAt  time.Time
	Scopes     []string
}

type Authorization struct {
	ExternalID, DisplayName string
	AccessToken             string
	RefreshToken            string
	Scopes                  []string
	ExpiresAt               time.Time
}

func (registrar *Registrar) BeginAuthorization(ctx context.Context, appID, appSecret string, scopes []string) (AuthorizationRequest, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return AuthorizationRequest{}, errors.New("begin Feishu authorization: application credentials are required")
	}
	scopes = normalizedScopes(scopes)
	if !contains(scopes, "offline_access") {
		scopes = append(scopes, "offline_access")
	}
	values := url.Values{"client_id": {appID}, "scope": {strings.Join(scopes, " ")}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registrar.accountsBase+deviceAuthorizationPath, strings.NewReader(values.Encode()))
	if err != nil {
		return AuthorizationRequest{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(appID+":"+appSecret)))
	body, err := registrar.doJSON(request)
	if err != nil {
		return AuthorizationRequest{}, fmt.Errorf("begin Feishu authorization: %w", err)
	}
	deviceCode, actionURL, expiresIn := stringField(body, "device_code"), stringField(body, "verification_uri_complete"), integerField(body, "expires_in")
	if actionURL == "" {
		actionURL = stringField(body, "verification_uri")
	}
	parsed, parseErr := url.Parse(actionURL)
	accountsURL, accountsErr := url.Parse(registrar.accountsBase)
	openURL, openErr := url.Parse(registrar.openBaseURL)
	allowedHost := parseErr == nil && accountsErr == nil && openErr == nil && (parsed.Hostname() == accountsURL.Hostname() || parsed.Hostname() == openURL.Hostname())
	if deviceCode == "" || expiresIn <= 0 || expiresIn > 24*60*60 || parsed.Scheme != "https" || !allowedHost {
		return AuthorizationRequest{}, errors.New("begin Feishu authorization: invalid provider response")
	}
	return AuthorizationRequest{DeviceCode: deviceCode, ActionURL: actionURL, ExpiresAt: registrar.now().Add(time.Duration(expiresIn) * time.Second), Scopes: scopes}, nil
}

func (registrar *Registrar) PollAuthorization(ctx context.Context, appID, appSecret, deviceCode string) (Authorization, error) {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" || strings.TrimSpace(deviceCode) == "" {
		return Authorization{}, errors.New("poll Feishu authorization: application credentials and device code are required")
	}
	values := url.Values{
		"grant_type":    {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code":   {deviceCode},
		"client_id":     {appID},
		"client_secret": {appSecret},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registrar.openBaseURL+oauthTokenPath, strings.NewReader(values.Encode()))
	if err != nil {
		return Authorization{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := registrar.doJSON(request)
	if err != nil {
		return Authorization{}, fmt.Errorf("poll Feishu authorization: %w", err)
	}
	switch stringField(body, "error") {
	case "authorization_pending", "slow_down":
		return Authorization{}, ErrPending
	case "access_denied":
		return Authorization{}, ErrDenied
	case "expired_token", "invalid_grant":
		return Authorization{}, ErrExpired
	case "":
	default:
		return Authorization{}, errors.New("poll Feishu authorization: provider rejected request")
	}
	accessToken := stringField(body, "access_token")
	if accessToken == "" {
		return Authorization{}, ErrPending
	}
	expiresIn := integerField(body, "expires_in")
	if expiresIn <= 0 || expiresIn > 30*24*60*60 {
		return Authorization{}, errors.New("poll Feishu authorization: invalid token expiry")
	}
	externalID, displayName, err := registrar.userInfo(ctx, accessToken)
	if err != nil {
		return Authorization{}, err
	}
	return Authorization{
		ExternalID: externalID, DisplayName: displayName, AccessToken: accessToken,
		RefreshToken: stringField(body, "refresh_token"), Scopes: normalizedScopes(strings.Fields(stringField(body, "scope"))),
		ExpiresAt: registrar.now().Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func (registrar *Registrar) userInfo(ctx context.Context, accessToken string) (string, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, registrar.openBaseURL+userInfoPath, nil)
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	body, err := registrar.doJSON(request)
	if err != nil {
		return "", "", fmt.Errorf("get Feishu user info: %w", err)
	}
	data, _ := body["data"].(map[string]any)
	externalID, displayName := stringField(data, "open_id"), stringField(data, "name")
	if externalID == "" {
		return "", "", errors.New("get Feishu user info: invalid provider response")
	}
	if displayName == "" {
		displayName = externalID
	}
	return externalID, displayName, nil
}

func (registrar *Registrar) doJSON(request *http.Request) (map[string]any, error) {
	response, err := registrar.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("provider response is too large")
	}
	body := map[string]any{}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, errors.New("provider returned invalid JSON")
	}
	if response.StatusCode >= 400 && stringField(body, "error") == "" {
		return nil, fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	return body, nil
}

func normalizedScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; !ok {
			seen[scope] = struct{}{}
			result = append(result, scope)
		}
	}
	sort.Strings(result)
	return result
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
