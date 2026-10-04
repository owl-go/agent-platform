package application

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type renewalRepo struct {
	grant    domain.ConnectorAuthorization
	saved    *domain.ConnectorAuthorization
	locked   bool
	released int
	missing  bool
	saveErr  error
	owners   []string
}

func (r *renewalRepo) ListDueFeishuAuthorizations(_ context.Context, owner string, _ time.Time) ([]domain.ConnectorAuthorization, error) {
	r.owners = append(r.owners, owner)
	return []domain.ConnectorAuthorization{r.grant}, nil
}
func (r *renewalRepo) LockConnectorAuthorizationRefresh(context.Context, string, string, string) (func(), bool, error) {
	return func() { r.released++ }, r.locked, nil
}
func (r *renewalRepo) GetRenewableFeishuAuthorization(context.Context, string, string, string) (domain.ConnectorAuthorization, error) {
	if r.missing {
		return domain.ConnectorAuthorization{}, domain.ErrNotFound
	}
	return r.grant, nil
}
func (r *renewalRepo) GetConnectorProviderApplication(context.Context, string, string) (domain.ConnectorProviderApplication, error) {
	return domain.ConnectorProviderApplication{AppIDCiphertext: []byte("app"), AppSecretCiphertext: []byte("secret")}, nil
}
func (r *renewalRepo) SaveRenewedFeishuAuthorization(_ context.Context, a domain.ConnectorAuthorization, version int64) error {
	if version != r.grant.Version {
		return domain.ErrConflict
	}
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = &a
	return nil
}

type renewalCipher struct{}

func (renewalCipher) Encrypt(raw []byte, _ string) ([]byte, error) {
	return append([]byte(nil), raw...), nil
}
func (renewalCipher) Decrypt(raw []byte, _ string) ([]byte, error) {
	return append([]byte(nil), raw...), nil
}

func TestConnectorAuthorizationRenewal(t *testing.T) {
	for _, scenario := range []string{"expired", "near_expiry", "manual_already_renewed", "revoked", "locked", "provider_failure", "no_refresh", "account_changed", "scope_expanded", "scope_narrowed", "legacy", "save_conflict"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC()
			expiry := now.Add(-time.Minute)
			repo := &renewalRepo{locked: true, grant: domain.ConnectorAuthorization{ID: "grant", OwnerID: "owner", InstallationID: "installation", IdentityRef: "user", ExternalIdentityID: "account", Scopes: []string{"read", "write"}, CredentialCiphertext: []byte(`{"access_token":"old","refresh_token":"refresh"}`), CredentialAAD: "retained-aad", CredentialFormat: "json", State: domain.ConnectorAuthorizationActive, ExpiresAt: &expiry, Version: 7}}
			result := ConnectorRenewalGrant{ExternalID: "account", AccessToken: "new", RefreshToken: "rotated", ExpiresAt: now.Add(time.Hour)}
			var providerErr error
			switch scenario {
			case "near_expiry":
				expiry = now.Add(4 * time.Minute)
			case "manual_already_renewed":
				expiry = now.Add(time.Hour)
			case "revoked":
				repo.missing = true
			case "locked":
				repo.locked = false
			case "provider_failure":
				providerErr = errors.New("provider error containing a secret")
			case "no_refresh":
				repo.grant.CredentialCiphertext = []byte(`{"access_token":"old"}`)
			case "account_changed":
				result.ExternalID = "other"
			case "scope_expanded":
				result.Scopes = []string{"admin"}
			case "scope_narrowed":
				result.Scopes = []string{"read"}
			case "legacy":
				repo.grant.CredentialFormat = "access_token"
				repo.grant.CredentialCiphertext = []byte("old")
				repo.grant.RefreshCredentialCiphertext = []byte("refresh")
				repo.grant.RefreshCredentialAAD = "refresh-aad"
			case "save_conflict":
				repo.saveErr = domain.ErrConflict
			}
			calls := 0
			var events []string
			svc := NewConnectorAuthorizationRenewal(repo, renewalCipher{}, func(ctx context.Context, app, secret, token string) (ConnectorRenewalGrant, error) {
				calls++
				if app != "app" || secret != "secret" || token != "refresh" {
					t.Fatal("wrong credential context")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded request")
				}
				return result, providerErr
			}, func(event string) { events = append(events, event) })
			svc.now = func() time.Time { return now }
			if _, err := svc.RenewOwner(context.Background(), "owner"); scenario == "locked" {
				if !errors.Is(err, errAuthorizationRenewalBusy) {
					t.Fatal("busy renewal must defer admission")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			success := scenario == "expired" || scenario == "near_expiry" || scenario == "legacy" || scenario == "scope_narrowed"
			if (repo.saved != nil) != success {
				t.Fatalf("saved=%t expected=%t", repo.saved != nil, success)
			}
			if success {
				if len(events) != 1 || events[0] != "renewed" {
					t.Fatal("success event absent")
				}
				var fields map[string]string
				if json.Unmarshal(repo.saved.CredentialCiphertext, &fields) != nil || fields["access_token"] != "new" || fields["refresh_token"] != "rotated" {
					t.Fatal("rotated credential not saved")
				}
				if repo.saved.ExternalIdentityID != "account" || repo.saved.CredentialAAD != "retained-aad" || repo.saved.Version != 7 {
					t.Fatal("identity/AAD/version changed")
				}
				if scenario == "scope_narrowed" && len(repo.saved.Scopes) != 1 {
					t.Fatal("narrower scope not retained")
				}
			} else if scenario != "locked" && scenario != "revoked" && scenario != "manual_already_renewed" {
				if len(events) != 1 || events[0] != "failed" {
					t.Fatal("unsafe error propagation")
				}
				before := calls
				svc.RenewOwner(context.Background(), "owner")
				if calls != before {
					t.Fatal("failed provider hammered without cooldown")
				}
			}
			if scenario == "locked" && repo.released != 0 {
				t.Fatal("unheld lock released")
			}
			if repo.locked && repo.released < 1 {
				t.Fatal("lock leaked")
			}
			if repo.owners[0] != "owner" {
				t.Fatal("owner lost")
			}
		})
	}
}
