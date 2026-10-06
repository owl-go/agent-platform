package application

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

type registrationMemory struct {
	mu       sync.Mutex
	settings map[string]domain.RegistrationSettings
	attempts map[string]domain.RegistrationAttempt
	audits   int
}

func (m *registrationMemory) Settings(_ context.Context, p string) (domain.RegistrationSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.settings[p]; ok {
		return s, nil
	}
	return domain.RegistrationSettings{Provider: p}, nil
}
func (m *registrationMemory) SaveSettings(_ context.Context, _ string, s domain.RegistrationSettings, expected int64, _ string) (domain.RegistrationSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settings[s.Provider].Version != expected {
		return s, domain.ErrConflict
	}
	s.Version = expected + 1
	s.Ready = false
	m.settings[s.Provider] = s
	m.audits++
	return s, nil
}
func (m *registrationMemory) MarkReady(_ context.Context, p string, v int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.settings[p]
	if s.Version != v {
		return domain.ErrConflict
	}
	s.Ready = true
	m.settings[p] = s
	return nil
}
func (m *registrationMemory) CreateAttempt(_ context.Context, a domain.RegistrationAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[a.ID] = a
	return nil
}
func (m *registrationMemory) Attempt(_ context.Context, id string) (domain.RegistrationAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.attempts[id]; ok {
		return a, nil
	}
	return domain.RegistrationAttempt{}, domain.ErrNotFound
}
func (m *registrationMemory) AttemptByCode(_ context.Context, hash string) (domain.RegistrationAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.attempts {
		if a.CodeHash == hash {
			return a, nil
		}
	}
	return domain.RegistrationAttempt{}, domain.ErrNotFound
}
func (m *registrationMemory) AttemptByLoginCode(_ context.Context, hash string) (domain.RegistrationAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.attempts {
		if a.LoginCodeHash == hash {
			return a, nil
		}
	}
	return domain.RegistrationAttempt{}, domain.ErrNotFound
}
func (m *registrationMemory) TransitionAttempt(_ context.Context, a domain.RegistrationAttempt, from string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.attempts[a.ID]
	if old.Version != a.Version || old.Status != from || !time.Now().Before(old.ExpiresAt) {
		return domain.ErrConflict
	}
	a.Version++
	m.attempts[a.ID] = a
	return nil
}

type registrationAccounts struct {
	Repository
	mu      sync.Mutex
	users   map[string]domain.User
	created int
}

func (r *registrationAccounts) FindPrincipal(_ context.Context, sub string) (domain.Principal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[sub]
	if !ok {
		return domain.Principal{}, domain.ErrNotFound
	}
	return domain.Principal{UserID: u.ID, Disabled: !u.Enabled}, nil
}
func (r *registrationAccounts) CreateUser(_ context.Context, u domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[u.OIDCSubject]; ok {
		return u, domain.ErrConflict
	}
	u.ID = "new-user"
	r.users[u.OIDCSubject] = u
	r.created++
	return u, nil
}

type registrationIdentityProvider struct {
	configureErr error
	mu           sync.Mutex
	calls        int
}

func (p *registrationIdentityProvider) ConfigureRegistration(context.Context, domain.RegistrationSettings, string, string) error {
	return p.configureErr
}
func (p *registrationIdentityProvider) EnsureRegistrationUser(_ context.Context, _ string, i domain.RegistrationIdentity) (domain.User, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return domain.User{OIDCSubject: "kc-" + i.Subject, Username: i.Username, DisplayName: i.DisplayName, Enabled: true}, nil
}

type registrationGateway struct {
	calls       int
	identityErr error
}

func (g *registrationGateway) VerifySettings(_ context.Context, s domain.RegistrationSettings) (domain.RegistrationSettings, error) {
	g.calls++
	if s.Provider == domain.RegistrationFeishu {
		s.TenantKey = "trusted-tenant"
	}
	return s, nil
}
func (g *registrationGateway) FeishuURL(domain.RegistrationSettings, string, string) string {
	return "https://accounts.feishu.cn/test"
}
func (g *registrationGateway) FeishuIdentity(_ context.Context, s domain.RegistrationSettings, _ string, _ string) (domain.RegistrationIdentity, error) {
	if g.identityErr != nil {
		return domain.RegistrationIdentity{}, g.identityErr
	}
	return domain.NewRegistrationIdentity(s.Provider, s.AppID, "external-person", "张三")
}
func registrationFixture() (*Registration, *registrationMemory, *registrationAccounts, *registrationIdentityProvider, *registrationGateway) {
	repo := &registrationMemory{settings: map[string]domain.RegistrationSettings{}, attempts: map[string]domain.RegistrationAttempt{}}
	users := &registrationAccounts{users: map[string]domain.User{}}
	accounts := &Service{repo: users}
	provider := &registrationIdentityProvider{}
	gateway := &registrationGateway{}
	service := NewRegistration(accounts, repo, provider, gateway, "https://workspace.test", "https://identity.test/realms/workspace", strings.Repeat("s", 32))
	for _, p := range []string{domain.RegistrationFeishu, domain.RegistrationWeChat} {
		repo.settings[p] = domain.RegistrationSettings{Provider: p, Enabled: true, Ready: true, AppID: "app", AppSecret: "stored-secret", TenantKey: "trusted-tenant", OfficialAccountID: "gh_account", VerificationToken: "token", EncodingAESKey: strings.Repeat("a", 43), Version: 1}
	}
	return service, repo, users, provider, gateway
}
func registrationChallenge() (string, string) {
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:]), verifier
}
func beginRegistration(t *testing.T, s *Registration, p string) (domain.RegistrationAttempt, string) {
	t.Helper()
	challenge, _ := registrationChallenge()
	a, b, _, err := s.Begin(t.Context(), p, s.RedirectURI(p), "oidc-state", "oidc-nonce", challenge)
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}
func TestRegistrationRequiresAdministratorAndPreservesOnlySameApplicationSecrets(t *testing.T) {
	s, repo, _, _, gateway := registrationFixture()
	input := domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: true, AppID: "app"}
	if _, err := s.Save(WithPrincipal(t.Context(), domain.Principal{UserID: "ordinary"}), input, 1, "test"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal(err)
	}
	ctx := WithPrincipal(t.Context(), domain.Principal{UserID: "administrator", Administrator: true})
	saved, err := s.Save(ctx, input, 1, "enable staff registration")
	if err != nil {
		t.Fatal(err)
	}
	if saved.AppSecret != "stored-secret" || saved.TenantKey != "trusted-tenant" || !saved.Ready || repo.audits != 1 {
		t.Fatal("settings or audit lost")
	}
	if _, err = s.Save(ctx, input, 1, "stale edit"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale write allowed")
	}
	input.AppID = "replacement"
	if _, err = s.Save(ctx, input, 2, "replace application"); err == nil {
		t.Fatal("new app inherited old credentials")
	}
	if gateway.calls != 1 {
		t.Fatal("invalid edit reached provider")
	}
}
func TestRegistrationSynchronizationFailureClosesNewAndPendingAttempts(t *testing.T) {
	s, repo, _, p, _ := registrationFixture()
	a, b := beginRegistration(t, s, domain.RegistrationFeishu)
	p.configureErr = errors.New("identity down")
	ctx := WithPrincipal(t.Context(), domain.Principal{UserID: "admin", Administrator: true})
	_, err := s.Save(ctx, domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: true, AppID: "app"}, 1, "rotate config")
	if err == nil || repo.settings[domain.RegistrationFeishu].Ready {
		t.Fatal("failed sync left method open")
	}
	if _, err = s.BrowserAttempt(t.Context(), a.ID, b); err == nil {
		t.Fatal("old attempt survived settings change")
	}
	p.configureErr = nil
	_, err = s.Save(ctx, domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: true, AppID: "app"}, 2, "retry sync")
	if err != nil {
		t.Fatal(err)
	}
}
func TestRegistrationWeChatFollowProofBrowserBindingAndSingleUseExchange(t *testing.T) {
	s, _, users, _, _ := registrationFixture()
	a, b := beginRegistration(t, s, domain.RegistrationWeChat)
	if _, _, err := s.Complete(t.Context(), a.ID, b); err == nil {
		t.Fatal("completed without follow")
	}
	for _, code := range []string{"", "AW-AAAA-AAAA-AAAA"} {
		if err := s.VerifyWeChat(t.Context(), code, "openid"); err == nil {
			t.Fatal("invalid login code accepted")
		}
	}
	if err := s.VerifyWeChat(t.Context(), a.LoginCode, "openid"); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifyWeChat(t.Context(), a.LoginCode, "openid"); err != nil {
		t.Fatal("repeat callback not idempotent", err)
	}
	if err := s.VerifyWeChat(t.Context(), a.LoginCode, "someone-else"); err == nil {
		t.Fatal("second identity overwrote proof")
	}
	if _, _, err := s.Complete(t.Context(), a.ID, "another-browser"); err == nil {
		t.Fatal("proof moved to another browser")
	}
	issued, code, err := s.Complete(t.Context(), a.ID, b)
	if err != nil {
		t.Fatal(err)
	}
	if users.created != 1 {
		t.Fatal("ordinary user not provisioned")
	}
	if _, _, err = s.Complete(t.Context(), a.ID, b); err == nil {
		t.Fatal("second code issued")
	}
	_, verifier := registrationChallenge()
	for _, bad := range []struct{ provider, redirect, verifier string }{{domain.RegistrationFeishu, issued.RedirectURI, verifier}, {a.Provider, "https://attacker.test", verifier}, {a.Provider, issued.RedirectURI, strings.Repeat("wrong", 10)}} {
		if _, err = s.Exchange(t.Context(), bad.provider, code, bad.redirect, bad.verifier); err == nil {
			t.Fatal("invalid code binding accepted")
		}
	}
	if _, err = s.Exchange(t.Context(), a.Provider, code, issued.RedirectURI, verifier); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Exchange(t.Context(), a.Provider, code, issued.RedirectURI, verifier); err == nil {
		t.Fatal("code replay accepted")
	}
	second, browser := beginRegistration(t, s, a.Provider)
	_ = s.VerifyWeChat(t.Context(), second.LoginCode, "openid")
	if _, _, err = s.Complete(t.Context(), second.ID, browser); err != nil {
		t.Fatal(err)
	}
	if users.created != 1 {
		t.Fatal("returning user duplicated")
	}
}
func TestRegistrationFeishuRejectsWrongBrowserProviderAndEnterprise(t *testing.T) {
	s, _, users, _, g := registrationFixture()
	a, b := beginRegistration(t, s, domain.RegistrationFeishu)
	if err := s.VerifyFeishu(t.Context(), a.ID, "other-browser", "code"); err == nil {
		t.Fatal("wrong browser accepted")
	}
	g.identityErr = domain.ErrUnauthenticated
	if err := s.VerifyFeishu(t.Context(), a.ID, b, "wrong-tenant-code"); err == nil {
		t.Fatal("wrong enterprise accepted")
	}
	if users.created != 0 {
		t.Fatal("wrong enterprise registered")
	}
	g.identityErr = nil
	if err := s.VerifyFeishu(t.Context(), a.ID, b, "code"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Complete(t.Context(), a.ID, b); err != nil {
		t.Fatal(err)
	}
}
func TestRegistrationExpiryDisableAndDisabledUserAreRejected(t *testing.T) {
	for _, mode := range []string{"expired", "disabled", "changed", "disabled-user"} {
		t.Run(mode, func(t *testing.T) {
			s, repo, users, _, _ := registrationFixture()
			a, b := beginRegistration(t, s, domain.RegistrationWeChat)
			_ = s.VerifyWeChat(t.Context(), a.LoginCode, "openid")
			switch mode {
			case "expired":
				a = repo.attempts[a.ID]
				a.ExpiresAt = time.Now().Add(-time.Second)
				repo.attempts[a.ID] = a
			case "disabled":
				settings := repo.settings[a.Provider]
				settings.Enabled = false
				repo.settings[a.Provider] = settings
			case "changed":
				settings := repo.settings[a.Provider]
				settings.Version++
				repo.settings[a.Provider] = settings
			case "disabled-user":
				identity, _ := domain.NewRegistrationIdentity(a.Provider, "app", "openid", "微信用户")
				users.users["kc-"+identity.Subject] = domain.User{ID: "disabled", Enabled: false}
			}
			if _, _, err := s.Complete(t.Context(), a.ID, b); err == nil {
				t.Fatal("closed registration completed")
			}
		})
	}
}
func TestRegistrationConcurrentCompletionIssuesOneCode(t *testing.T) {
	s, _, users, _, _ := registrationFixture()
	a, b := beginRegistration(t, s, domain.RegistrationWeChat)
	if err := s.VerifyWeChat(t.Context(), a.LoginCode, "openid"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := s.Complete(context.Background(), a.ID, b); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 || users.created != 1 {
		t.Fatalf("successes=%d users=%d", success, users.created)
	}
}

func TestWeChatLoginCodeIsBrowserBoundAndDoesNotCallQRProvider(t *testing.T) {
	s, _, _, _, gateway := registrationFixture()
	a, browser := beginRegistration(t, s, domain.RegistrationWeChat)
	if NormalizeRegistrationLoginCode(a.LoginCode) != a.LoginCode || a.LoginCodeHash != RegistrationHash(a.LoginCode) || a.CodeHash != "" || gateway.calls != 0 {
		t.Fatal("invalid or externally generated login challenge")
	}
	if a.QRURL != "https://open.weixin.qq.com/qr/code?username=gh_account" {
		t.Fatal("not an ordinary follow QR")
	}
	second, secondBrowser := beginRegistration(t, s, domain.RegistrationWeChat)
	if a.LoginCode == second.LoginCode {
		t.Fatal("login code reused")
	}
	if err := s.VerifyWeChat(t.Context(), strings.ToLower(strings.ReplaceAll(a.LoginCode, "-", "")), "openid"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Complete(t.Context(), a.ID, secondBrowser); err == nil {
		t.Fatal("another browser completed the code")
	}
	if _, _, err := s.Complete(t.Context(), second.ID, browser); err == nil {
		t.Fatal("identity transferred to another attempt")
	}
	if _, _, err := s.Complete(t.Context(), a.ID, browser); err != nil {
		t.Fatal(err)
	}
}

func TestWeChatLoginCodeRejectsExpiredClosedAndChangedAttempts(t *testing.T) {
	for _, mode := range []string{"expired", "disabled", "changed", "foreign-provider"} {
		t.Run(mode, func(t *testing.T) {
			s, repo, _, _, _ := registrationFixture()
			a, _ := beginRegistration(t, s, domain.RegistrationWeChat)
			switch mode {
			case "expired":
				a.ExpiresAt = time.Now().Add(-time.Second)
				repo.attempts[a.ID] = a
			case "disabled":
				settings := repo.settings[a.Provider]
				settings.Enabled = false
				repo.settings[a.Provider] = settings
			case "changed":
				settings := repo.settings[a.Provider]
				settings.Version++
				repo.settings[a.Provider] = settings
			case "foreign-provider":
				a.Provider = domain.RegistrationFeishu
				repo.attempts[a.ID] = a
			}
			if err := s.VerifyWeChat(t.Context(), a.LoginCode, "openid"); err == nil {
				t.Fatal("unavailable challenge accepted")
			}
			if repo.attempts[a.ID].Identity.Subject != "" {
				t.Fatal("rejected message changed identity")
			}
		})
	}
}

func TestWeChatConcurrentDifferentSendersCannotClaimOneCode(t *testing.T) {
	s, repo, _, _, _ := registrationFixture()
	a, _ := beginRegistration(t, s, domain.RegistrationWeChat)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.VerifyWeChat(context.Background(), a.LoginCode, "openid-"+string(rune('a'+i)))
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 || repo.attempts[a.ID].Status != "verified" {
		t.Fatal("multiple senders claimed one challenge")
	}
}

func TestWeChatConcurrentProviderRetriesAreIdempotent(t *testing.T) {
	s, repo, _, _, _ := registrationFixture()
	a, _ := beginRegistration(t, s, domain.RegistrationWeChat)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.VerifyWeChat(context.Background(), a.LoginCode, "same-openid") }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("provider retry failed", err)
		}
	}
	if repo.attempts[a.ID].Version != 2 {
		t.Fatal("provider retry changed confirmation more than once")
	}
}

func TestRegistrationLoginCodeRejectsNonLoginMessages(t *testing.T) {
	for _, value := range []string{"", "hello", "123456", "AW-AAAA-AAAA-AAA0", "AW-AAAA-AAAA-AAAA<script>", "登录 AW-AAAA-AAAA-AAAA"} {
		if NormalizeRegistrationLoginCode(value) != "" {
			t.Fatalf("ordinary message parsed as code: %q", value)
		}
	}
}
