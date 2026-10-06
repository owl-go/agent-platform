package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

type RegistrationRepository interface {
	Settings(context.Context, string) (domain.RegistrationSettings, error)
	SaveSettings(context.Context, string, domain.RegistrationSettings, int64, string) (domain.RegistrationSettings, error)
	MarkReady(context.Context, string, int64) error
	CreateAttempt(context.Context, domain.RegistrationAttempt) error
	Attempt(context.Context, string) (domain.RegistrationAttempt, error)
	AttemptByCode(context.Context, string) (domain.RegistrationAttempt, error)
	AttemptByLoginCode(context.Context, string) (domain.RegistrationAttempt, error)
	ReserveWeChatVerification(context.Context, string) error
	TransitionAttempt(context.Context, domain.RegistrationAttempt, string) error
}
type RegistrationIdentityProvider interface {
	ConfigureRegistration(context.Context, domain.RegistrationSettings, string, string) error
	EnsureRegistrationUser(context.Context, string, domain.RegistrationIdentity) (domain.User, error)
}
type RegistrationGateway interface {
	VerifySettings(context.Context, domain.RegistrationSettings) (domain.RegistrationSettings, error)
	FeishuURL(domain.RegistrationSettings, string, string) string
	FeishuIdentity(context.Context, domain.RegistrationSettings, string, string) (domain.RegistrationIdentity, error)
}
type Registration struct {
	accounts     *Service
	repo         RegistrationRepository
	identity     RegistrationIdentityProvider
	gateway      RegistrationGateway
	publicURL    string
	issuer       string
	clientSecret string
}

func NewRegistration(accounts *Service, repo RegistrationRepository, identity RegistrationIdentityProvider, gateway RegistrationGateway, publicURL, issuer, clientSecret string) *Registration {
	return &Registration{accounts: accounts, repo: repo, identity: identity, gateway: gateway, publicURL: strings.TrimRight(publicURL, "/"), issuer: strings.TrimRight(issuer, "/"), clientSecret: clientSecret}
}
func (s *Registration) Available() bool {
	return s != nil && s.publicURL != "" && s.issuer != "" && s.clientSecret != ""
}
func (s *Registration) PublicURL() string    { return s.publicURL }
func (s *Registration) ClientSecret() string { return s.clientSecret }
func (s *Registration) Issuer(provider string) string {
	return s.publicURL + "/api/v1/registration/" + provider
}
func (s *Registration) Callback(provider string) string { return s.Issuer(provider) + "/callback" }
func (s *Registration) RedirectURI(provider string) string {
	return s.issuer + "/broker/" + domain.RegistrationAlias(provider) + "/endpoint"
}
func (s *Registration) AdminSettings(ctx context.Context, provider string) (domain.RegistrationSettings, error) {
	principal, err := s.accounts.Current(ctx)
	if err != nil {
		return domain.RegistrationSettings{}, err
	}
	if err = principal.RequireAdministrator(); err != nil {
		return domain.RegistrationSettings{}, err
	}
	return s.repo.Settings(ctx, provider)
}
func (s *Registration) PublicSettings(ctx context.Context, provider string) (domain.RegistrationSettings, error) {
	if !s.Available() || !domain.RegistrationProvider(provider) {
		return domain.RegistrationSettings{}, domain.ErrForbidden
	}
	settings, err := s.repo.Settings(ctx, provider)
	if err != nil {
		return settings, err
	}
	if !settings.Enabled || !settings.Ready {
		return settings, domain.ErrForbidden
	}
	return settings, nil
}
func (s *Registration) Save(ctx context.Context, input domain.RegistrationSettings, expected int64, reason string) (domain.RegistrationSettings, error) {
	principal, err := s.accounts.Current(ctx)
	if err != nil {
		return input, err
	}
	if err = principal.RequireAdministrator(); err != nil {
		return input, err
	}
	if !s.Available() {
		return input, fmt.Errorf("registration broker is not configured")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 || expected < 0 || !domain.RegistrationProvider(input.Provider) {
		return input, fmt.Errorf("registration change requires a method, version and reason")
	}
	old, err := s.repo.Settings(ctx, input.Provider)
	if err != nil {
		return input, err
	}
	if old.Version != expected {
		return input, domain.ErrConflict
	}
	input.AppID = strings.TrimSpace(input.AppID)
	input.OfficialAccountID = strings.TrimSpace(input.OfficialAccountID)
	if input.AppID == old.AppID {
		if input.AppSecret == "" {
			input.AppSecret = old.AppSecret
		}
		if input.VerificationToken == "" {
			input.VerificationToken = old.VerificationToken
		}
		if input.EncodingAESKey == "" {
			input.EncodingAESKey = old.EncodingAESKey
		}
	}
	input.TenantKey = ""
	input.Ready = false
	// Validate credential shape before making external requests. Enterprise
	// identity is obtained from the application credentials, never browser input.
	if input.Enabled && input.Provider == domain.RegistrationFeishu {
		input.TenantKey = "pending"
	}
	if err = input.Validate(); err != nil {
		return input, err
	}
	if input.Enabled {
		input, err = s.gateway.VerifySettings(ctx, input)
		if err != nil {
			return input, err
		}
	}
	if err = input.Validate(); err != nil {
		return input, err
	}
	saved, err := s.repo.SaveSettings(ctx, principal.UserID, input, expected, reason)
	if err != nil {
		return input, err
	}
	if err = s.identity.ConfigureRegistration(ctx, saved, s.Issuer(input.Provider), s.clientSecret); err != nil {
		return saved, fmt.Errorf("registration entry could not be configured; refresh and save again: %w", err)
	}
	if err = s.repo.MarkReady(ctx, saved.Provider, saved.Version); err != nil {
		return saved, err
	}
	saved.Ready = true
	return saved, nil
}
func RegistrationHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", digest[:])
}
func RegistrationRandom() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (s *Registration) Begin(ctx context.Context, provider, redirect, state, nonce, challenge string) (domain.RegistrationAttempt, string, string, error) {
	settings, err := s.PublicSettings(ctx, provider)
	if err != nil {
		return domain.RegistrationAttempt{}, "", "", err
	}
	decoded, decodeErr := base64.RawURLEncoding.DecodeString(challenge)
	if redirect != s.RedirectURI(provider) || state == "" || len(state) > 2048 || nonce == "" || len(nonce) > 512 || decodeErr != nil || len(decoded) != 32 {
		return domain.RegistrationAttempt{}, "", "", domain.ErrUnauthenticated
	}
	id, err := RegistrationRandom()
	if err != nil {
		return domain.RegistrationAttempt{}, "", "", err
	}
	browser, err := RegistrationRandom()
	if err != nil {
		return domain.RegistrationAttempt{}, "", "", err
	}
	attempt := domain.RegistrationAttempt{ID: id, Provider: provider, ConfigVersion: settings.Version, BrowserHash: RegistrationHash(browser), RedirectURI: redirect, State: state, Nonce: nonce, Challenge: challenge, Status: "waiting", ExpiresAt: time.Now().UTC().Add(5 * time.Minute), Version: 1}
	destination := ""
	if provider == domain.RegistrationFeishu {
		destination = s.gateway.FeishuURL(settings, s.Callback(provider), id)
	} else {
		code, e := RegistrationLoginCode()
		if e != nil {
			return attempt, "", "", e
		}
		attempt.LoginCode = code
		attempt.LoginCodeHash = RegistrationHash(code)
		destination = "https://open.weixin.qq.com/qr/code?username=" + url.QueryEscape(settings.OfficialAccountID)
	}
	attempt.QRURL = destination
	for retry := 0; ; retry++ {
		err = s.repo.CreateAttempt(ctx, attempt)
		if err == nil {
			break
		}
		if provider != domain.RegistrationWeChat || !errors.Is(err, domain.ErrLoginCodeConflict) || retry >= 31 {
			return attempt, "", "", err
		}
		attempt.LoginCode, err = RegistrationLoginCode()
		if err != nil {
			return attempt, "", "", err
		}
		attempt.LoginCodeHash = RegistrationHash(attempt.LoginCode)
	}
	return attempt, browser, destination, nil
}
func (s *Registration) BrowserAttempt(ctx context.Context, id, browser string) (domain.RegistrationAttempt, error) {
	a, err := s.repo.Attempt(ctx, id)
	if err != nil {
		return a, err
	}
	if browser == "" || RegistrationHash(browser) != a.BrowserHash || !time.Now().Before(a.ExpiresAt) || a.Status == "consumed" {
		return a, domain.ErrUnauthenticated
	}
	settings, err := s.PublicSettings(ctx, a.Provider)
	if err != nil || settings.Version != a.ConfigVersion {
		return a, domain.ErrUnauthenticated
	}
	return a, nil
}
func (s *Registration) VerifyFeishu(ctx context.Context, id, browser, code string) error {
	a, err := s.BrowserAttempt(ctx, id, browser)
	if err != nil {
		return err
	}
	if a.Provider != domain.RegistrationFeishu || a.Status != "waiting" || code == "" {
		return domain.ErrUnauthenticated
	}
	settings, err := s.PublicSettings(ctx, a.Provider)
	if err != nil {
		return err
	}
	a.Identity, err = s.gateway.FeishuIdentity(ctx, settings, s.Callback(a.Provider), code)
	if err != nil {
		return err
	}
	a.Status = "verified"
	return s.repo.TransitionAttempt(ctx, a, "waiting")
}
func (s *Registration) VerifyWeChat(ctx context.Context, code, externalID string) error {
	code = NormalizeRegistrationLoginCode(code)
	if code == "" {
		return domain.ErrUnauthenticated
	}
	settings, err := s.PublicSettings(ctx, domain.RegistrationWeChat)
	if err != nil {
		return err
	}
	identity, err := domain.NewRegistrationIdentity(domain.RegistrationWeChat, settings.AppID, externalID, "微信用户")
	if err != nil {
		return err
	}
	a, lookupErr := s.repo.AttemptByLoginCode(ctx, RegistrationHash(code))
	valid := lookupErr == nil && a.Provider == domain.RegistrationWeChat && a.ConfigVersion == settings.Version && time.Now().Before(a.ExpiresAt) && a.LoginCodeHash == RegistrationHash(code)
	// Already confirmed messages from the same identity are provider retries, not
	// new guesses. They cannot bind a waiting attempt or replace another identity.
	if valid && a.Status != "waiting" && a.Identity.Subject == identity.Subject {
		return nil
	}
	if err = s.repo.ReserveWeChatVerification(ctx, RegistrationHash(settings.AppID+":"+externalID)); err != nil {
		return err
	}
	if lookupErr != nil {
		return lookupErr
	}
	if !valid {
		return domain.ErrUnauthenticated
	}
	if a.Status != "waiting" {
		return domain.ErrConflict
	}
	a.Identity = identity
	a.Status = "verified"
	err = s.repo.TransitionAttempt(ctx, a, "waiting")
	if errors.Is(err, domain.ErrConflict) {
		// WeChat may deliver the same message concurrently to different replicas.
		// A CAS winner is an idempotent success only for that same identity.
		current, readErr := s.repo.Attempt(ctx, a.ID)
		if readErr == nil && current.Provider == a.Provider && current.ConfigVersion == a.ConfigVersion && current.Status != "waiting" && time.Now().Before(current.ExpiresAt) && current.Identity.Subject == identity.Subject {
			return nil
		}
	}
	return err
}

// Login codes are short browser challenges, never OIDC or product tokens.
// rand.Int samples uniformly and formatting preserves leading zeroes.
func RegistrationLoginCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", value.Int64()), nil
}

func NormalizeRegistrationLoginCode(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 4 {
		return ""
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return ""
		}
	}
	return value
}
func (s *Registration) Complete(ctx context.Context, id, browser string) (domain.RegistrationAttempt, string, error) {
	a, err := s.BrowserAttempt(ctx, id, browser)
	if err != nil {
		return a, "", err
	}
	if a.Status != "verified" || a.Identity.Subject == "" {
		return a, "", domain.ErrUnauthenticated
	}
	user, err := s.identity.EnsureRegistrationUser(ctx, a.Provider, a.Identity)
	if err != nil {
		return a, "", err
	}
	principal, err := s.accounts.repo.FindPrincipal(ctx, user.OIDCSubject)
	if errors.Is(err, domain.ErrNotFound) {
		_, err = s.accounts.repo.CreateUser(ctx, user)
		if err != nil {
			principal, err = s.accounts.repo.FindPrincipal(ctx, user.OIDCSubject)
		} else {
			principal, err = s.accounts.repo.FindPrincipal(ctx, user.OIDCSubject)
		}
	}
	if err != nil {
		return a, "", err
	}
	if err = principal.Validate(); err != nil {
		return a, "", err
	}
	// Revalidate after external I/O so a concurrent disable/credential change
	// cannot issue a usable broker code for the old configuration.
	current, err := s.BrowserAttempt(ctx, id, browser)
	if err != nil || current.Version != a.Version {
		return a, "", domain.ErrConflict
	}
	code, err := RegistrationRandom()
	if err != nil {
		return a, "", err
	}
	a.CodeHash = RegistrationHash(code)
	a.Status = "issued"
	a.ExpiresAt = time.Now().UTC().Add(time.Minute)
	if err = s.repo.TransitionAttempt(ctx, a, "verified"); err != nil {
		return a, "", err
	}
	return a, code, nil
}
func (s *Registration) Exchange(ctx context.Context, provider, code, redirect, verifier string) (domain.RegistrationAttempt, error) {
	a, err := s.repo.AttemptByCode(ctx, RegistrationHash(code))
	if err != nil {
		return a, domain.ErrUnauthenticated
	}
	settings, err := s.PublicSettings(ctx, provider)
	digest := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(digest[:])
	if err != nil || a.Provider != provider || a.ConfigVersion != settings.Version || a.Status != "issued" || !time.Now().Before(a.ExpiresAt) || redirect != a.RedirectURI || len(verifier) < 43 || len(verifier) > 128 || challenge != a.Challenge {
		return a, domain.ErrUnauthenticated
	}
	a.Status = "consumed"
	if err = s.repo.TransitionAttempt(ctx, a, "issued"); err != nil {
		return a, domain.ErrUnauthenticated
	}
	return a, nil
}
