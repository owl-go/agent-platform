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
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Provider{client: &copyClient, baseURL: strings.TrimRight(config.KeycloakBaseURL, "/"), realm: config.Realm, clientID: config.AdminClientID, clientSecret: config.AdminClientSecret}, nil
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

type keycloakGroup struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Path       string              `json:"path"`
	Attributes map[string][]string `json:"attributes"`
	SubGroups  []keycloakGroup     `json:"subGroups"`
}

type keycloakGroupMember struct {
	ID string `json:"id"`
}

func (provider *Provider) ListGroups(ctx context.Context) ([]accountdomain.IdentityGroupSnapshot, error) {
	const pageSize = 500
	roots := make([]keycloakGroup, 0)
	for first := 0; ; first += pageSize {
		endpoint := fmt.Sprintf("%s?briefRepresentation=false&first=%d&max=%d", provider.adminURL("groups"), first, pageSize)
		response, err := provider.request(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			err := responseError("list Keycloak Groups", response)
			_ = response.Body.Close()
			return nil, err
		}
		var page []keycloakGroup
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&page)
		_ = response.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Keycloak Groups: %w", decodeErr)
		}
		roots = append(roots, page...)
		if len(roots) > 10_000 {
			return nil, fmt.Errorf("Keycloak Group synchronization exceeds 10000 root groups")
		}
		if len(page) < pageSize {
			break
		}
	}
	flattened := make([]keycloakGroup, 0, len(roots))
	var flatten func([]keycloakGroup) error
	flatten = func(groups []keycloakGroup) error {
		for _, group := range groups {
			if strings.TrimSpace(group.ID) == "" || strings.TrimSpace(group.Name) == "" {
				return fmt.Errorf("Keycloak Group response omitted identity")
			}
			flattened = append(flattened, group)
			if len(flattened) > 10_000 {
				return fmt.Errorf("Keycloak Group synchronization exceeds 10000 groups")
			}
			if err := flatten(group.SubGroups); err != nil {
				return err
			}
		}
		return nil
	}
	if err := flatten(roots); err != nil {
		return nil, err
	}
	result := make([]accountdomain.IdentityGroupSnapshot, 0, len(flattened))
	for _, group := range flattened {
		members := make([]keycloakGroupMember, 0)
		for first := 0; ; first += pageSize {
			membersEndpoint := fmt.Sprintf("%s?briefRepresentation=true&first=%d&max=%d", provider.adminURL("groups", group.ID, "members"), first, pageSize)
			membersResponse, err := provider.request(ctx, http.MethodGet, membersEndpoint, nil)
			if err != nil {
				return nil, err
			}
			if membersResponse.StatusCode != http.StatusOK {
				err := responseError("list Keycloak Group members", membersResponse)
				_ = membersResponse.Body.Close()
				return nil, err
			}
			var page []keycloakGroupMember
			decodeErr := json.NewDecoder(io.LimitReader(membersResponse.Body, 4<<20)).Decode(&page)
			_ = membersResponse.Body.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("decode Keycloak Group members: %w", decodeErr)
			}
			members = append(members, page...)
			if len(members) > 10_000 {
				return nil, fmt.Errorf("Keycloak Group %q exceeds 10000 members", group.Name)
			}
			if len(page) < pageSize {
				break
			}
		}
		subjects := make([]string, 0, len(members))
		for _, member := range members {
			if strings.TrimSpace(member.ID) != "" {
				subjects = append(subjects, member.ID)
			}
		}
		department := false
		for _, value := range group.Attributes["agent_workspace_department"] {
			if strings.EqualFold(strings.TrimSpace(value), "true") {
				department = true
			}
		}
		path := strings.TrimSpace(group.Path)
		if path == "" {
			path = "/" + strings.TrimSpace(group.Name)
		}
		result = append(result, accountdomain.IdentityGroupSnapshot{ExternalID: group.ID, Name: group.Name, Path: path, Department: department, MemberSubjects: subjects})
	}
	return result, nil
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
