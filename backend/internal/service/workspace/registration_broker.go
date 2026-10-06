package workspace

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	registrationgateway "agent-platform/backend/internal/data/account/registration"
)

type registrationBroker struct {
	app     *accountapplication.Registration
	key     *rsa.PrivateKey
	keyID   string
	mu      sync.Mutex
	started time.Time
	starts  int
}

func NewRegistrationBroker(app *accountapplication.Registration, encodedKey string) (*registrationBroker, error) {
	broker := &registrationBroker{app: app}
	if !app.Available() {
		return broker, nil
	}
	bytes, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("registration signing key must be base64 PKCS8 DER")
	}
	key, err := x509.ParsePKCS8PrivateKey(bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid registration signing key")
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok || rsaKey.N.BitLen() < 2048 || rsaKey.Validate() != nil {
		return nil, fmt.Errorf("registration signing key must be RSA with at least 2048 bits")
	}
	broker.key = rsaKey
	publicKey, _ := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	digest := sha256.Sum256(publicKey)
	broker.keyID = hex.EncodeToString(digest[:16])
	return broker, nil
}
func (b *registrationBroker) allowStart() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if time.Since(b.started) > time.Minute {
		b.started = time.Now()
		b.starts = 0
	}
	if b.starts >= 120 {
		return false
	}
	b.starts++
	return true
}
func registrationPublicRoute(method, path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/registration/"), "/")
	if !strings.HasPrefix(path, "/api/v1/registration/") || len(parts) != 2 || !accountdomain.RegistrationProvider(parts[0]) {
		return false
	}
	switch parts[1] {
	case "authorize", "jwks":
		return method == http.MethodGet
	case "callback":
		return method == http.MethodGet || (parts[0] == accountdomain.RegistrationWeChat && method == http.MethodPost)
	case "token", "complete":
		return method == http.MethodPost
	case "status":
		return method == http.MethodGet
	}
	return false
}
func (service *Service) registrationHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	broker := service.registration
	if broker == nil || !broker.app.Available() {
		writeAuthError(writer, 503, "registration_unavailable")
		return
	}
	parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/v1/registration/"), "/")
	if len(parts) != 2 || !registrationPublicRoute(request.Method, request.URL.Path) {
		writeAuthError(writer, 404, "not_found")
		return
	}
	provider, operation := parts[0], parts[1]
	switch operation {
	case "jwks":
		key := broker.key
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": broker.keyID, "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	case "authorize":
		if !broker.allowStart() {
			writeAuthError(writer, 429, "registration_rate_limited")
			return
		}
		q := request.URL.Query()
		if q.Get("client_id") != "agent-workspace-registration" || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || !hasOpenIDScope(q.Get("scope")) {
			writeAuthError(writer, 400, "invalid_request")
			return
		}
		attempt, browser, destination, err := broker.app.Begin(request.Context(), provider, q.Get("redirect_uri"), q.Get("state"), q.Get("nonce"), q.Get("code_challenge"))
		if err != nil {
			writeAuthError(writer, 400, "registration_start_failed")
			return
		}
		http.SetCookie(writer, registrationCookie(attempt.ID, browser, 300))
		if provider == accountdomain.RegistrationFeishu {
			http.Redirect(writer, request, destination, http.StatusFound)
			return
		}
		http.Redirect(writer, request, broker.app.PublicURL()+"/register/wechat?id="+url.QueryEscape(attempt.ID), http.StatusFound)

	case "status", "complete":
		id := request.URL.Query().Get("id")
		browser := registrationBrowser(request, id)
		if operation == "status" {
			a, err := broker.app.BrowserAttempt(request.Context(), id, browser)
			if err != nil {
				writeAuthError(writer, 410, "registration_expired")
				return
			}
			writer.Header().Set("Content-Type", "application/json")
			if provider == accountdomain.RegistrationWeChat && accountapplication.NormalizeRegistrationLoginCode(a.LoginCode) == "" {
				writeAuthError(writer, 410, "registration_expired")
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]string{"status": a.Status, "qr_url": a.QRURL, "login_code": a.LoginCode, "expires_at": a.ExpiresAt.Format(time.RFC3339)})
			return
		}
		if request.Header.Get("X-Registration-Request") != "1" {
			writeAuthError(writer, 403, "invalid_request")
			return
		}
		if origin := request.Header.Get("Origin"); origin != broker.app.PublicURL() {
			writeAuthError(writer, 403, "invalid_origin")
			return
		}
		attempt, code, err := broker.app.Complete(request.Context(), id, browser)
		if err != nil {
			writeAuthError(writer, 400, "registration_completion_failed")
			return
		}
		http.SetCookie(writer, registrationCookie(id, "", -1))
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"redirect": registrationRedirect(attempt, code)})
	case "callback":
		if provider == accountdomain.RegistrationWeChat {
			broker.wechatCallback(writer, request)
			return
		}
		id := request.URL.Query().Get("state")
		browser := registrationBrowser(request, id)
		if request.URL.Query().Get("error") != "" || broker.app.VerifyFeishu(request.Context(), id, browser, request.URL.Query().Get("code")) != nil {
			http.Redirect(writer, request, broker.app.PublicURL()+"/register/failed", http.StatusFound)
			return
		}
		attempt, code, err := broker.app.Complete(request.Context(), id, browser)
		if err != nil {
			http.Redirect(writer, request, broker.app.PublicURL()+"/register/failed", http.StatusFound)
			return
		}
		http.SetCookie(writer, registrationCookie(id, "", -1))
		http.Redirect(writer, request, registrationRedirect(attempt, code), http.StatusFound)
	case "token":
		broker.token(writer, request, provider)
	}
}
func hasOpenIDScope(scope string) bool {
	for _, part := range strings.Fields(scope) {
		if part == "openid" {
			return true
		}
	}
	return false
}
func registrationCookie(id, value string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: "aw_reg_" + id, Value: value, Path: "/api/v1/registration/", MaxAge: maxAge, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
}
func registrationBrowser(request *http.Request, id string) string {
	if len(id) != 43 {
		return ""
	}
	cookie, err := request.Cookie("aw_reg_" + id)
	if err != nil {
		return ""
	}
	return cookie.Value
}
func registrationRedirect(attempt accountdomain.RegistrationAttempt, code string) string {
	return attempt.RedirectURI + "?" + url.Values{"code": {code}, "state": {attempt.State}}.Encode()
}

func (b *registrationBroker) token(w http.ResponseWriter, r *http.Request, provider string) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if r.ParseForm() != nil {
		writeAuthError(w, 400, "invalid_request")
		return
	}
	client, secret, ok := r.BasicAuth()
	if !ok {
		client = r.PostForm.Get("client_id")
		secret = r.PostForm.Get("client_secret")
	}
	if client != "agent-workspace-registration" || subtle.ConstantTimeCompare([]byte(secret), []byte(b.app.ClientSecret())) != 1 {
		writeAuthError(w, 401, "invalid_client")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		writeAuthError(w, 400, "unsupported_grant_type")
		return
	}
	attempt, err := b.app.Exchange(r.Context(), provider, r.PostForm.Get("code"), r.PostForm.Get("redirect_uri"), r.PostForm.Get("code_verifier"))
	if err != nil {
		writeAuthError(w, 400, "invalid_grant")
		return
	}
	now := time.Now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": b.keyID})
	claims, _ := json.Marshal(map[string]any{"iss": b.app.Issuer(provider), "aud": "agent-workspace-registration", "sub": attempt.Identity.Subject, "preferred_username": attempt.Identity.Username, "name": attempt.Identity.DisplayName, "nonce": attempt.Nonce, "iat": now, "exp": now + 60})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, b.key, crypto.SHA256, digest[:])
	if err != nil {
		writeAuthError(w, 503, "registration_unavailable")
		return
	}
	// The opaque access_token has no product API or provider API authority.
	access, err := accountapplication.RegistrationRandom()
	if err != nil {
		writeAuthError(w, 503, "registration_unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"token_type": "Bearer", "expires_in": 60, "access_token": access, "id_token": unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)})
}
func (b *registrationBroker) wechatCallback(w http.ResponseWriter, r *http.Request) {
	settings, err := b.app.PublicSettings(r.Context(), accountdomain.RegistrationWeChat)
	if err != nil {
		writeAuthError(w, 403, "registration_disabled")
		return
	}
	if r.Method == http.MethodGet {
		echo, err := registrationgateway.VerifyWeChatURL(settings, r.URL.Query(), time.Now())
		if err != nil {
			writeAuthError(w, 403, "invalid_callback")
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, echo)
		return
	}
	encrypted := ""
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		var envelope struct{ Encrypt string }
		if xml.NewDecoder(r.Body).Decode(&envelope) != nil {
			writeAuthError(w, 400, "invalid_callback")
			return
		}
		encrypted = envelope.Encrypt
	}
	plain, err := registrationgateway.DecryptWeChat(settings, r.URL.Query(), encrypted, time.Now())
	if err != nil {
		writeAuthError(w, 403, "invalid_callback")
		return
	}
	message, err := registrationgateway.ParseWeChatMessage(settings, plain, time.Now())
	if err != nil {
		writeAuthError(w, 403, "invalid_callback")
		return
	}
	reply := ""
	if message.MsgType == "event" && message.Event == "subscribe" {
		reply = "请将登录网页上的一次性登录码发送给本公众号，完成登录或注册。只发送你自己登录页面上的码。"
	} else if message.MsgType == "text" {
		code := accountapplication.NormalizeRegistrationLoginCode(message.Content)
		reply = "请将当前登录网页上的完整一次性登录码发送给本公众号。"
		if code != "" {
			if err := b.app.VerifyWeChat(r.Context(), code, message.FromUserName); err == nil {
				reply = "登录已确认，请回到发起登录的网页。"
			} else if errors.Is(err, accountdomain.ErrRegistrationRateLimited) {
				reply = "尝试过于频繁，请稍后再试。"
			} else {
				reply = "登录码无效或已过期，请返回网页重新发起登录。"
			}
		}
	}
	if reply != "" {
		body, err := registrationgateway.WeChatTextReply(settings, message, reply, time.Now())
		if err != nil {
			writeAuthError(w, 503, "registration_unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = w.Write(body)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = io.WriteString(w, "success")
}
