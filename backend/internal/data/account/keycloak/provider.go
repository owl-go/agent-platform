package keycloak

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/platformconfig"
)

type Provider struct {
	client       *http.Client
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
}

func New(config platformconfig.AccountsConfig, client *http.Client) (*Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Provider{client: client, baseURL: strings.TrimRight(config.KeycloakBaseURL, "/"), realm: config.Realm, clientID: config.AdminClientID, clientSecret: config.AdminClientSecret}, nil
}

func (provider *Provider) CreateUser(ctx context.Context, input accountdomain.NewUser) (string, string, error) {
	password, err := temporaryPassword()
	if err != nil {
		return "", "", err
	}
	firstName, lastName := identityNames(input.DisplayName)
	payload := map[string]any{
		"username": input.Username, "email": input.Email, "firstName": firstName, "lastName": lastName,
		"enabled": true, "emailVerified": false,
		"credentials":     []map[string]any{{"type": "password", "value": password, "temporary": true}},
		"requiredActions": []string{"UPDATE_PASSWORD"},
	}
	response, err := provider.request(ctx, http.MethodPost, provider.adminURL("users"), payload)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return "", "", responseError("create Keycloak User", response)
	}
	location := response.Header.Get("Location")
	identifier := strings.TrimSpace(location[strings.LastIndex(location, "/")+1:])
	if identifier == "" {
		return "", "", fmt.Errorf("Keycloak create User response omitted its identifier")
	}
	return identifier, password, nil
}

func identityNames(displayName string) (string, string) {
	parts := strings.Fields(displayName)
	if len(parts) == 1 {
		return parts[0], parts[0]
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

func (provider *Provider) SetEnabled(ctx context.Context, subject string, enabled bool) error {
	response, err := provider.request(ctx, http.MethodPut, provider.adminURL("users", subject), map[string]any{"enabled": enabled})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return responseError("update Keycloak User", response)
	}
	return nil
}

func (provider *Provider) ResetPassword(ctx context.Context, subject string) (string, error) {
	password, err := temporaryPassword()
	if err != nil {
		return "", err
	}
	response, err := provider.request(ctx, http.MethodPut, provider.adminURL("users", subject, "reset-password"), map[string]any{"type": "password", "value": password, "temporary": true})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return "", responseError("reset Keycloak User password", response)
	}
	return password, nil
}

func (provider *Provider) request(ctx context.Context, method, endpoint string, payload any) (*http.Response, error) {
	token, err := provider.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	return provider.client.Do(request)
}

func (provider *Provider) accessToken(ctx context.Context) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {provider.clientID}, "client_secret": {provider.clientSecret}}
	endpoint := provider.baseURL + "/realms/" + url.PathEscape(provider.realm) + "/protocol/openid-connect/token"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := provider.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request Keycloak Admin token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", responseError("request Keycloak Admin token", response)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("decode Keycloak Admin token response")
	}
	return result.AccessToken, nil
}

func (provider *Provider) adminURL(segments ...string) string {
	path := provider.baseURL + "/admin/realms/" + url.PathEscape(provider.realm)
	for _, segment := range segments {
		path += "/" + url.PathEscape(segment)
	}
	return path
}

func temporaryPassword() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "Aw!" + base64.RawURLEncoding.EncodeToString(value), nil
}

func responseError(operation string, response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
	return fmt.Errorf("%s returned HTTP %d: %s", operation, response.StatusCode, strings.TrimSpace(string(body)))
}
