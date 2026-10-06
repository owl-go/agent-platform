package workspace

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/data/account/keycloak"
	registrationgateway "agent-platform/backend/internal/data/account/registration"
	"agent-platform/backend/internal/data/account/tokenverifier"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/productanalytics"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
)

type brokerTestStore struct {
	accountapplication.RegistrationRepository
	settings   accountdomain.RegistrationSettings
	attempts   map[string]accountdomain.RegistrationAttempt
	reserveErr error
}

func (s *brokerTestStore) ReserveWeChatVerification(context.Context, string) error {
	return s.reserveErr
}
func (s *brokerTestStore) Settings(context.Context, string) (accountdomain.RegistrationSettings, error) {
	return s.settings, nil
}
func (s *brokerTestStore) CreateAttempt(_ context.Context, a accountdomain.RegistrationAttempt) error {
	for _, old := range s.attempts {
		if a.LoginCodeHash != "" && old.LoginCodeHash == a.LoginCodeHash {
			return accountdomain.ErrLoginCodeConflict
		}
	}
	s.attempts[a.ID] = a
	return nil
}
func (s *brokerTestStore) Attempt(_ context.Context, id string) (accountdomain.RegistrationAttempt, error) {
	a, ok := s.attempts[id]
	if !ok {
		return a, accountdomain.ErrNotFound
	}
	return a, nil
}
func (s *brokerTestStore) AttemptByCode(_ context.Context, hash string) (accountdomain.RegistrationAttempt, error) {
	for _, a := range s.attempts {
		if a.CodeHash == hash {
			return a, nil
		}
	}
	return accountdomain.RegistrationAttempt{}, accountdomain.ErrNotFound
}
func (s *brokerTestStore) AttemptByLoginCode(_ context.Context, hash string) (accountdomain.RegistrationAttempt, error) {
	for _, a := range s.attempts {
		if a.LoginCodeHash == hash {
			return a, nil
		}
	}
	return accountdomain.RegistrationAttempt{}, accountdomain.ErrNotFound
}
func (s *brokerTestStore) TransitionAttempt(_ context.Context, a accountdomain.RegistrationAttempt, from string) error {
	old := s.attempts[a.ID]
	if old.Status != from || old.Version != a.Version {
		return accountdomain.ErrConflict
	}
	a.Version++
	s.attempts[a.ID] = a
	return nil
}

type brokerTestAccounts struct {
	accountapplication.Repository
	users map[string]accountdomain.User
}

func (s *brokerTestAccounts) FindPrincipal(_ context.Context, subject string) (accountdomain.Principal, error) {
	u, ok := s.users[subject]
	if !ok {
		return accountdomain.Principal{}, accountdomain.ErrNotFound
	}
	return accountdomain.Principal{UserID: u.ID, Disabled: !u.Enabled}, nil
}
func (s *brokerTestAccounts) CreateUser(_ context.Context, u accountdomain.User) (accountdomain.User, error) {
	u.ID = "local-projection"
	s.users[u.OIDCSubject] = u
	return u, nil
}

type brokerTestGateway struct {
	accountapplication.RegistrationGateway
}

func (g brokerTestGateway) FeishuURL(_ accountdomain.RegistrationSettings, callback, state string) string {
	return callback + "?" + url.Values{"state": {state}, "code": {"fixture-code"}}.Encode()
}
func (g brokerTestGateway) FeishuIdentity(_ context.Context, s accountdomain.RegistrationSettings, _ string, _ string) (accountdomain.RegistrationIdentity, error) {
	return accountdomain.NewRegistrationIdentity(s.Provider, s.AppID, "fixture-person", "扫码用户")
}
func brokerSigningKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(encoded)
}
func TestRegistrationBypassIsLimitedToExactPublicProtocols(t *testing.T) {
	filter, err := NewAuthenticationFilter(&accountapplication.Service{}, &workspaceapplication.Service{}, productanalytics.Nop{})
	if err != nil {
		t.Fatal(err)
	}
	handler := filter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		method, path string
		status       int
	}{{"GET", "/api/v1/registration/feishu/authorize", 204}, {"POST", "/api/v1/registration/feishu/token", 204}, {"GET", "/api/v1/registration/wechat_official/callback", 204}, {"POST", "/api/v1/registration/wechat_official/callback", 204}, {"GET", "/api/v1/admin/registration-methods", 401}, {"PUT", "/api/v1/admin/registration-methods/feishu", 401}, {"GET", "/api/v1/registration/unknown/jwks", 401}, {"PUT", "/api/v1/registration/feishu/authorize", 401}, {"GET", "/api/v1/registration/feishu/authorize/extra", 401}, {"GET", "/api/v1/registration/feishu/token", 401}, {"POST", "/api/v1/registration/feishu/callback", 401}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(test.method, test.path, nil))
		if w.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, w.Code)
		}
	}
}
func TestBrokerClientAuthenticationAndCompletionCSRF(t *testing.T) {
	app := accountapplication.NewRegistration(nil, nil, nil, nil, "https://workspace.test", "https://identity.test/realms/workspace", strings.Repeat("s", 32))
	broker, err := NewRegistrationBroker(app, brokerSigningKey(t))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{registration: broker}
	for _, test := range []struct {
		path    string
		headers map[string]string
		status  int
	}{{"/api/v1/registration/feishu/token", nil, 401}, {"/api/v1/registration/wechat_official/complete", nil, 403}, {"/api/v1/registration/wechat_official/complete", map[string]string{"X-Registration-Request": "1", "Origin": "https://attacker.test"}, 403}} {
		r := httptest.NewRequest("POST", test.path, strings.NewReader("grant_type=authorization_code"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range test.headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		service.registrationHTTP(w, r)
		if w.Code != test.status {
			t.Fatal("unsafe request accepted", w.Code)
		}
	}
	cookie := registrationCookie("id", "browser", 300)
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/v1/registration/" {
		t.Fatal("unsafe browser binding cookie")
	}
}

// Uses a disposable local Keycloak realm; the upstream Feishu identity is a
// fake, while broker PKCE, RS256/JWKS, federation, and final OIDC are real.
func TestRegistrationKeycloakOIDCHandoffIntegration(t *testing.T) {
	base := os.Getenv("WORKSPACE_TEST_KEYCLOAK_URL")
	if base == "" {
		t.Skip("WORKSPACE_TEST_KEYCLOAK_URL is not set")
	}
	for _, provider := range []string{accountdomain.RegistrationFeishu, accountdomain.RegistrationWeChat} {
		t.Run(provider, func(t *testing.T) { registrationOIDCHandoff(t, base, provider) })
	}
}

func registrationOIDCHandoff(t *testing.T, base, providerName string) {
	issuer := strings.TrimRight(base, "/") + "/realms/scan-fixture"
	identity, err := keycloak.New(platformconfig.AccountsConfig{KeycloakBaseURL: base, Realm: "scan-fixture", AdminClientID: "scan-admin", AdminClientSecret: "scan-fixture", BootstrapSubject: "admin", BootstrapUsername: "admin", BootstrapEmail: "admin@example.test", BootstrapDisplayName: "Admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	localUsers := &brokerTestAccounts{users: map[string]accountdomain.User{}}
	accounts, err := accountapplication.New(tokenverifier.Rejecting{}, localUsers, identity)
	if err != nil {
		t.Fatal(err)
	}
	store := &brokerTestStore{settings: accountdomain.RegistrationSettings{Provider: providerName, Enabled: true, Ready: true, AppID: "fixture-app-" + uuid.NewString(), OfficialAccountID: "gh_fixture", VerificationToken: "callback-token", EncodingAESKey: base64.RawStdEncoding.EncodeToString(make([]byte, 32)), Version: 1}, attempts: map[string]accountdomain.RegistrationAttempt{}}
	service := &Service{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(service.registrationHTTP))
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	publicURL := "http://host.docker.internal:" + strconv.Itoa(port)
	app := accountapplication.NewRegistration(accounts, store, identity, brokerTestGateway{}, publicURL, issuer, strings.Repeat("b", 32))
	if err = service.EnableRegistration(app, brokerSigningKey(t)); err != nil {
		t.Fatal(err)
	}
	if err = identity.ConfigureRegistration(t.Context(), store.settings, app.Issuer(store.settings.Provider), app.ClientSecret()); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, p, e := net.SplitHostPort(address)
		if e == nil && host == "host.docker.internal" {
			address = net.JoinHostPort("127.0.0.1", p)
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	verifier := strings.Repeat("p", 43)
	hash := sha256.Sum256([]byte(verifier))
	callback := "http://127.0.0.1/callback"
	destination := issuer + "/protocol/openid-connect/auth?" + url.Values{"client_id": {"scan-web"}, "response_type": {"code"}, "scope": {"openid"}, "redirect_uri": {callback}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}, "state": {"product-state"}, "nonce": {"product-nonce"}, "kc_idp_hint": {accountdomain.RegistrationAlias(store.settings.Provider)}}.Encode()
	fixtureCookies := map[string]map[string]*http.Cookie{}
	for hop := 0; hop < 12; hop++ {
		r, err := http.NewRequestWithContext(t.Context(), "GET", destination, nil)
		if err != nil {
			t.Fatal(err)
		}
		// The disposable HTTP fixture carries Secure cookies manually between
		// its own endpoints. Production configuration requires HTTPS.
		for _, c := range fixtureCookies[r.URL.Host] {
			r.AddCookie(c)
		}
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		if fixtureCookies[r.URL.Host] == nil {
			fixtureCookies[r.URL.Host] = map[string]*http.Cookie{}
		}
		for _, c := range resp.Cookies() {
			if c.MaxAge < 0 {
				delete(fixtureCookies[r.URL.Host], c.Name)
			} else {
				fixtureCookies[r.URL.Host][c.Name] = c
			}
		}
		if resp.StatusCode != 302 && resp.StatusCode != 303 {
			if i := strings.Index(string(body), `id="kc-page-title"`); i >= 0 {
				body = body[i:]
			}
			t.Fatalf("handoff stopped at hop %d, HTTP %d: %.6000s", hop, resp.StatusCode, body)
		}
		next, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		prior, _ := url.Parse(destination)
		destination = prior.ResolveReference(next).String()
		if providerName == accountdomain.RegistrationWeChat && strings.HasPrefix(destination, publicURL+"/register/wechat?") {
			page, _ := url.Parse(destination)
			attemptID := page.Query().Get("id")
			attempt := store.attempts[attemptID]
			message := fmt.Sprintf("<xml><ToUserName>%s</ToUserName><FromUserName>openid</FromUserName><CreateTime>%d</CreateTime><MsgType>text</MsgType><Content>%s</Content><MsgId>10001</MsgId></xml>", store.settings.OfficialAccountID, time.Now().Unix(), attempt.LoginCode)
			phone, _ := encryptedLoginMessage(t, store.settings, message)
			phone.URL, _ = url.Parse(publicURL + phone.URL.String())
			phone.RequestURI = ""
			phoneResponse, err := client.Do(phone)
			if err != nil {
				t.Fatal(err)
			}
			phoneReply, _ := io.ReadAll(phoneResponse.Body)
			phoneResponse.Body.Close()
			if phoneResponse.StatusCode != 200 || !strings.Contains(string(phoneReply), "<Encrypt>") {
				t.Fatal("encrypted phone confirmation failed")
			}
			completion, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, publicURL+"/api/v1/registration/wechat_official/complete?id="+attemptID, nil)
			completion.Header.Set("Origin", publicURL)
			completion.Header.Set("X-Registration-Request", "1")
			for _, cookie := range fixtureCookies[completion.URL.Host] {
				completion.AddCookie(cookie)
			}
			completed, err := client.Do(completion)
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Redirect string `json:"redirect"`
			}
			err = json.NewDecoder(completed.Body).Decode(&result)
			completed.Body.Close()
			if err != nil || completed.StatusCode != 200 || result.Redirect == "" {
				t.Fatal("browser completion failed", err)
			}
			destination = result.Redirect
		}
		if strings.HasPrefix(destination, callback+"?") {
			break
		}
	}
	result, err := url.Parse(destination)
	if err != nil || !strings.HasPrefix(destination, callback+"?") || result.Query().Get("state") != "product-state" || result.Query().Get("code") == "" {
		t.Fatal("Keycloak did not issue a product code")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {"scan-web"}, "code": {result.Query().Get("code")}, "redirect_uri": {callback}, "code_verifier": {verifier}}
	r, _ := http.NewRequestWithContext(t.Context(), "POST", issuer+"/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("product code exchange failed", resp.StatusCode)
	}
	var tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(resp.Body).Decode(&tokens) != nil || tokens.AccessToken == "" {
		t.Fatal("product token missing")
	}
	provider, err := oidc.NewProvider(t.Context(), issuer)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := provider.Verifier(&oidc.Config{ClientID: "scan-web"}).Verify(t.Context(), tokens.IDToken)
	if err != nil {
		t.Fatal("invalid final product identity", err)
	}
	if _, ok := localUsers.users[verified.Subject]; !ok || len(localUsers.users) != 1 {
		t.Fatal("product identity lacks one local projection")
	}
}

func TestWeChatSetupHandshakeIsGETOnlyAndDoesNotCreateIdentity(t *testing.T) {
	settings := accountdomain.RegistrationSettings{Provider: accountdomain.RegistrationWeChat, Enabled: true, Ready: true, VerificationToken: "setup-token"}
	store := &brokerTestStore{settings: settings}
	app := accountapplication.NewRegistration(nil, store, nil, nil, "https://workspace.example", "https://identity.example/realms/workspace", "client-secret")
	broker := &registrationBroker{app: app}
	query := url.Values{"timestamp": {strconv.FormatInt(time.Now().Unix(), 10)}, "nonce": {"nonce"}, "echostr": {"setup-echo"}}
	query.Set("signature", registrationgateway.WeChatSignature(settings.VerificationToken, query.Get("timestamp"), "nonce", ""))
	get := httptest.NewRecorder()
	broker.wechatCallback(get, httptest.NewRequest("GET", "/api/v1/registration/wechat_official/callback?"+query.Encode(), nil))
	if get.Code != 200 || get.Body.String() != "setup-echo" {
		t.Fatal("setup handshake failed", get.Code)
	}
	post := httptest.NewRecorder()
	broker.wechatCallback(post, httptest.NewRequest("POST", "/api/v1/registration/wechat_official/callback?"+query.Encode(), strings.NewReader("<xml><Event>subscribe</Event></xml>")))
	if post.Code != 403 {
		t.Fatal("plaintext setup signature authenticated an event", post.Code)
	}
	if len(store.attempts) != 0 {
		t.Fatal("setup handshake created an attempt")
	}
}
