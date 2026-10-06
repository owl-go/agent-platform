package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"agent-platform/backend/internal/biz/account/domain"
)

// ConfigureRegistration changes only the two platform-owned broker entries.
// A failed synchronization leaves the application method unready and closed.
func (p *Provider) ConfigureRegistration(ctx context.Context, s domain.RegistrationSettings, issuer, secret string) error {
	if err := p.validateRegistrationProfile(ctx); err != nil {
		return err
	}
	alias := domain.RegistrationAlias(s.Provider)
	name := "飞书扫码登录 / 注册"
	if s.Provider == domain.RegistrationWeChat {
		name = "微信公众号登录 / 注册"
	}
	representation := map[string]any{"alias": alias, "displayName": name, "providerId": "oidc", "enabled": true, "trustEmail": false, "storeToken": false, "addReadTokenRoleOnCreate": false, "linkOnly": false, "firstBrokerLoginFlowAlias": "first broker login", "config": map[string]string{
		"clientId": "agent-workspace-registration", "clientSecret": secret, "issuer": issuer, "authorizationUrl": issuer + "/authorize", "tokenUrl": issuer + "/token", "jwksUrl": issuer + "/jwks", "useJwksUrl": "true", "validateSignature": "true", "disableUserInfo": "true", "defaultScope": "openid profile", "syncMode": "IMPORT", "pkceEnabled": "true", "pkceMethod": "S256", "hideOnLoginPage": fmt.Sprint(!s.Enabled), "clientAuthMethod": "client_secret_basic", "backchannelSupported": "false",
	}}
	endpoint := p.adminURL("identity-provider", "instances", alias)
	resp, err := p.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("read registration identity entry")
	}
	status := resp.StatusCode
	_ = resp.Body.Close()
	method := http.MethodPut
	if status == http.StatusNotFound {
		method = http.MethodPost
		endpoint = p.adminURL("identity-provider", "instances")
	} else if status != http.StatusOK {
		return fmt.Errorf("registration identity entry returned HTTP %d", status)
	}
	resp, err = p.request(ctx, method, endpoint, representation)
	if err != nil {
		return fmt.Errorf("configure registration identity entry")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("configure registration identity entry returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type registrationUser struct {
	ID         string              `json:"id"`
	Username   string              `json:"username"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes"`
}
type registrationLink struct {
	IdentityProvider string `json:"identityProvider"`
	UserID           string `json:"userId"`
	UserName         string `json:"userName"`
}

func (p *Provider) registrationUsers(ctx context.Context, username string) ([]registrationUser, error) {
	resp, err := p.request(ctx, http.MethodGet, p.adminURL("users")+"?exact=true&username="+url.QueryEscape(username), nil)
	if err != nil {
		return nil, fmt.Errorf("read registered identity")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read registered identity returned HTTP %d", resp.StatusCode)
	}
	var users []registrationUser
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&users) != nil {
		return nil, fmt.Errorf("invalid registered identity response")
	}
	return users, nil
}
func (p *Provider) EnsureRegistrationUser(ctx context.Context, provider string, identity domain.RegistrationIdentity) (domain.User, error) {
	return p.ensureRegistrationUser(ctx, provider, identity, false)
}

func (p *Provider) ensureRegistrationUser(ctx context.Context, provider string, identity domain.RegistrationIdentity, retried bool) (domain.User, error) {
	if !domain.RegistrationProvider(provider) || identity.Subject == "" || identity.Username == "" {
		return domain.User{}, domain.ErrUnauthenticated
	}
	users, err := p.registrationUsers(ctx, identity.Username)
	if err != nil {
		return domain.User{}, err
	}
	if len(users) == 0 {
		first, last := identityNames(identity.DisplayName)
		payload := map[string]any{"username": identity.Username, "firstName": first, "lastName": last, "enabled": true, "emailVerified": false, "requiredActions": []string{}, "attributes": map[string][]string{"aw_registration_subject": {identity.Subject}, "aw_registration_provider": {provider}}}
		resp, err := p.request(ctx, http.MethodPost, p.adminURL("users"), payload)
		if err != nil {
			return domain.User{}, fmt.Errorf("create registered identity")
		}
		status := resp.StatusCode
		_ = resp.Body.Close()
		if status != http.StatusCreated && status != http.StatusConflict {
			return domain.User{}, fmt.Errorf("create registered identity returned HTTP %d", status)
		}
		users, err = p.registrationUsers(ctx, identity.Username)
		if err != nil {
			return domain.User{}, err
		}
	}
	if len(users) != 1 {
		return domain.User{}, domain.ErrUnauthenticated
	}
	user := users[0]
	if !user.Enabled || user.ID == "" || user.Username != identity.Username || len(user.Attributes["aw_registration_subject"]) != 1 || user.Attributes["aw_registration_subject"][0] != identity.Subject || len(user.Attributes["aw_registration_provider"]) != 1 || user.Attributes["aw_registration_provider"][0] != provider {
		return domain.User{}, domain.ErrUnauthenticated
	}
	// Link only the identity created with the above platform-owned attributes.
	// Never merge an existing password account by email or display name.
	endpoint := p.adminURL("users", user.ID, "federated-identity")
	resp, err := p.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.User{}, fmt.Errorf("read registered identity link")
	}
	var links []registrationLink
	status := resp.StatusCode
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&links)
	_ = resp.Body.Close()
	if status != http.StatusOK || decodeErr != nil {
		return domain.User{}, fmt.Errorf("read registered identity link failed")
	}
	linked := false
	alias := domain.RegistrationAlias(provider)
	for _, link := range links {
		if link.IdentityProvider == alias {
			if link.UserID != identity.Subject {
				return domain.User{}, domain.ErrUnauthenticated
			}
			linked = true
		}
	}
	if !linked {
		resp, err = p.request(ctx, http.MethodPost, p.adminURL("users", user.ID, "federated-identity", alias), registrationLink{IdentityProvider: alias, UserID: identity.Subject, UserName: identity.Username})
		if err != nil {
			return domain.User{}, fmt.Errorf("link registered identity")
		}
		status = resp.StatusCode
		_ = resp.Body.Close()
		if status == http.StatusConflict {
			if retried {
				return domain.User{}, domain.ErrConflict
			}
			return p.ensureRegistrationUser(ctx, provider, identity, true)
		} // concurrent first login
		if status != http.StatusNoContent {
			return domain.User{}, fmt.Errorf("link registered identity returned HTTP %d", status)
		}
	}
	return domain.User{OIDCSubject: user.ID, Username: identity.Username, DisplayName: identity.DisplayName, Enabled: true}, nil
}

// Keycloak drops unmanaged attributes by default. Deployment declares these
// admin-only markers once; the product service never changes realm/profile
// policy and does not require manage-realm privileges.
func (p *Provider) validateRegistrationProfile(ctx context.Context) error {
	resp, err := p.request(ctx, http.MethodGet, p.adminURL("users", "profile"), nil)
	if err != nil {
		return fmt.Errorf("read registration identity profile")
	}
	var profile struct {
		Attributes []struct {
			Name        string          `json:"name"`
			Required    json.RawMessage `json:"required"`
			Permissions struct {
				View []string `json:"view"`
				Edit []string `json:"edit"`
			} `json:"permissions"`
		} `json:"attributes"`
	}
	status := resp.StatusCode
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&profile)
	_ = resp.Body.Close()
	if status != http.StatusOK || decodeErr != nil {
		return fmt.Errorf("read registration identity profile failed")
	}
	for _, attribute := range profile.Attributes {
		if attribute.Name == "email" && len(attribute.Required) > 0 && string(attribute.Required) != "null" {
			return fmt.Errorf("registration identity profile must allow users without email")
		}
	}
	for _, name := range []string{"aw_registration_subject", "aw_registration_provider"} {
		found := false
		for _, attribute := range profile.Attributes {
			if attribute.Name == name && len(attribute.Permissions.View) == 1 && attribute.Permissions.View[0] == "admin" && len(attribute.Permissions.Edit) == 1 && attribute.Permissions.Edit[0] == "admin" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("registration identity profile needs deployment configuration")
		}
	}
	return nil
}
