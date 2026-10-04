package application

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var errAuthorizationRenewalBusy = errors.New("connector authorization refresh is in progress")

// Renewal operates only on existing grants. It cannot start authorization or
// change the selected account, resource selection, or reviewed scopes.
type ConnectorAuthorizationRenewalRepository interface {
	ListDueFeishuAuthorizations(context.Context, string, time.Time) ([]domain.ConnectorAuthorization, error)
	LockConnectorAuthorizationRefresh(context.Context, string, string, string) (func(), bool, error)
	GetRenewableFeishuAuthorization(context.Context, string, string, string) (domain.ConnectorAuthorization, error)
	GetConnectorProviderApplication(context.Context, string, string) (domain.ConnectorProviderApplication, error)
	SaveRenewedFeishuAuthorization(context.Context, domain.ConnectorAuthorization, int64) error
}

type ConnectorRenewalGrant struct {
	ExternalID, AccessToken, RefreshToken string
	Scopes                                []string
	ExpiresAt                             time.Time
}
type ConnectorAuthorizationRefresh func(context.Context, string, string, string) (ConnectorRenewalGrant, error)

type ConnectorAuthorizationRenewal struct {
	repository ConnectorAuthorizationRenewalRepository
	cipher     ChannelCipher
	refresh    ConnectorAuthorizationRefresh
	mu         sync.Mutex
	retry      map[string]time.Time
	now        func() time.Time
	observer   func(string)
}

func NewConnectorAuthorizationRenewal(repository ConnectorAuthorizationRenewalRepository, cipher ChannelCipher, refresh ConnectorAuthorizationRefresh, observer func(string)) *ConnectorAuthorizationRenewal {
	return &ConnectorAuthorizationRenewal{repository: repository, cipher: cipher, refresh: refresh, retry: map[string]time.Time{}, now: time.Now, observer: observer}
}

func (s *ConnectorAuthorizationRenewal) ProcessNext(ctx context.Context) (bool, error) {
	return s.RenewOwner(ctx, "")
}

func (s *ConnectorAuthorizationRenewal) RenewOwner(ctx context.Context, owner string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, until := range s.retry {
		if !now.Before(until) {
			delete(s.retry, key)
		}
	}
	items, err := s.repository.ListDueFeishuAuthorizations(ctx, owner, now.Add(5*time.Minute))
	if err != nil {
		return false, err
	}
	worked := false
	for _, item := range items {
		if ctx.Err() != nil {
			return worked, ctx.Err()
		}
		if until, ok := s.retry[item.ID]; ok && now.Before(until) {
			continue
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		attempted, renewErr := s.renew(attemptCtx, item)
		cancel()
		if errors.Is(renewErr, errAuthorizationRenewalBusy) {
			if owner != "" {
				return worked, renewErr
			}
			continue
		}
		if renewErr != nil {
			// Provider responses can contain credentials. Publish only a fixed event.
			s.retry[item.ID] = now.Add(5 * time.Minute)
			if s.observer != nil {
				s.observer("failed")
			}
		} else if attempted && s.observer != nil {
			s.observer("renewed")
		}
		worked = worked || attempted
		if owner == "" && attempted {
			break
		}
	}
	return worked, nil
}

func (s *ConnectorAuthorizationRenewal) renew(ctx context.Context, candidate domain.ConnectorAuthorization) (bool, error) {
	release, locked, err := s.repository.LockConnectorAuthorizationRefresh(ctx, candidate.OwnerID, candidate.InstallationID, candidate.ID)
	if err != nil {
		return false, err
	}
	if !locked {
		return false, errAuthorizationRenewalBusy
	}
	defer release()
	current, err := s.repository.GetRenewableFeishuAuthorization(ctx, candidate.OwnerID, candidate.InstallationID, candidate.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if current.ExpiresAt == nil || current.ExpiresAt.After(s.now().Add(5*time.Minute)) {
		return false, nil
	}
	var credentials map[string]string
	plaintext, err := s.cipher.Decrypt(current.CredentialCiphertext, current.CredentialAAD)
	if err != nil {
		return true, err
	}
	if current.CredentialFormat == "access_token" {
		credentials = map[string]string{"access_token": string(plaintext)}
	} else {
		err = json.Unmarshal(plaintext, &credentials)
	}
	clear(plaintext)
	if err != nil {
		return true, err
	}
	refreshToken := credentials["refresh_token"]
	if refreshToken == "" && len(current.RefreshCredentialCiphertext) > 0 {
		raw, decryptErr := s.cipher.Decrypt(current.RefreshCredentialCiphertext, current.RefreshCredentialAAD)
		if decryptErr != nil {
			return true, decryptErr
		}
		refreshToken = string(raw)
		clear(raw)
	}
	if refreshToken == "" {
		return true, domain.ErrConflict
	}
	app, err := s.repository.GetConnectorProviderApplication(ctx, current.OwnerID, current.InstallationID)
	if err != nil {
		return true, err
	}
	aad := "feishu-cli-application:" + current.OwnerID
	appID, err := s.cipher.Decrypt(app.AppIDCiphertext, aad)
	if err != nil {
		return true, err
	}
	defer clear(appID)
	appSecret, err := s.cipher.Decrypt(app.AppSecretCiphertext, aad)
	if err != nil {
		return true, err
	}
	defer clear(appSecret)
	grant, err := s.refresh(ctx, string(appID), string(appSecret), refreshToken)
	if err != nil {
		return true, err
	}
	if grant.AccessToken == "" || !grant.ExpiresAt.After(s.now()) || grant.ExternalID == "" || current.ExternalIdentityID != "" && current.ExternalIdentityID != grant.ExternalID {
		return true, domain.ErrConflict
	}
	// Renewal cannot broaden authority; omitted scope retains the prior grant.
	if len(grant.Scopes) > 0 {
		allowed := map[string]bool{}
		for _, scope := range current.Scopes {
			allowed[scope] = true
		}
		for _, scope := range grant.Scopes {
			if !allowed[scope] {
				return true, domain.ErrConflict
			}
		}
		current.Scopes = append([]string(nil), grant.Scopes...)
	}
	if grant.RefreshToken == "" {
		grant.RefreshToken = refreshToken
	}
	credentials["access_token"] = grant.AccessToken
	credentials["refresh_token"] = grant.RefreshToken
	credentials["access_expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
	encoded, err := json.Marshal(credentials)
	if err != nil {
		return true, err
	}
	defer clear(encoded)
	current.CredentialCiphertext, err = s.cipher.Encrypt(encoded, current.CredentialAAD)
	if err != nil {
		return true, err
	}
	current.CredentialFormat = "json"
	current.State = domain.ConnectorAuthorizationActive
	current.ExpiresAt = &grant.ExpiresAt
	current.RefreshCredentialCiphertext = nil
	current.RefreshCredentialAAD = ""
	return true, s.repository.SaveRenewedFeishuAuthorization(ctx, current, current.Version)
}
