package registration

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}
func TestFeishuValidatesApplicationEnterpriseBeforeUserRegistration(t *testing.T) {
	for _, tenant := range []string{"enterprise", "different", ""} {
		t.Run(tenant, func(t *testing.T) {
			g := New(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/open-apis/auth/v3/tenant_access_token/internal":
					return response(`{"code":0,"tenant_access_token":"application-token"}`), nil
				case "/open-apis/tenant/v2/tenant/query":
					if r.Header.Get("Authorization") != "Bearer application-token" {
						t.Fatal("wrong token")
					}
					return response(`{"code":0,"data":{"tenant":{"tenant_key":"enterprise"}}}`), nil
				case "/oauth/v3/token":
					if r.URL.Host != "accounts.feishu.cn" {
						t.Fatal("wrong token authority")
					}
					return response(`{"access_token":"user-token"}`), nil
				case "/open-apis/authen/v1/user_info":
					if r.Header.Get("Authorization") != "Bearer user-token" {
						t.Fatal("wrong token")
					}
					return response(fmt.Sprintf(`{"code":0,"data":{"open_id":"openid","name":"张三","tenant_key":%q}}`, tenant)), nil
				}
				t.Fatal("unexpected request", r.URL)
				return nil, nil
			})})
			settings, err := g.VerifySettings(t.Context(), domain.RegistrationSettings{Provider: domain.RegistrationFeishu, AppID: "app", AppSecret: "secret"})
			if err != nil || settings.TenantKey != "enterprise" {
				t.Fatal(err)
			}
			identity, err := g.FeishuIdentity(t.Context(), settings, "https://workspace.test/callback", "code")
			if (err == nil) != (tenant == "enterprise") {
				t.Fatal("enterprise restriction failed")
			}
			if err == nil && identity.Subject == "" {
				t.Fatal("missing stable identity")
			}
		})
	}
}
func TestProviderFailuresNeverExposeSecretsAndMissingCodesFailClosed(t *testing.T) {
	for _, body := range []string{`{"tenant_access_token":"secret"}`, `{"code":null,"tenant_access_token":"secret"}`, `{"code":1,"msg":"secret app password"}`, `not-json`} {
		g := New(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(body), nil })})
		_, err := g.VerifySettings(context.Background(), domain.RegistrationSettings{Provider: domain.RegistrationFeishu, AppID: "app", AppSecret: "secret"})
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe provider error", err)
		}
	}
}
func TestWeChatSettingsUseStableTokenWithoutQRPermission(t *testing.T) {
	calls := 0
	g := New(&http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Path != "/cgi-bin/stable_token" {
			t.Fatal("registration requires an unnecessary provider permission")
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, []byte(`"force_refresh":false`)) {
			t.Fatal("unsafe token refresh")
		}
		return response(`{"access_token":"wx-token"}`), nil
	})})
	_, err := g.VerifySettings(t.Context(), domain.RegistrationSettings{Provider: domain.RegistrationWeChat, AppID: "app", AppSecret: "secret"})
	if err != nil || calls != 1 {
		t.Fatal(err)
	}
}
func encryptCallback(t *testing.T, key []byte, app string, body []byte) string {
	t.Helper()
	plain := make([]byte, 20)
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(body)))
	plain = append(plain, body...)
	plain = append(plain, []byte(app)...)
	padding := 32 - len(plain)%32
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:16]).CryptBlocks(encrypted, plain)
	return base64.StdEncoding.EncodeToString(encrypted)
}
func TestWeChatSafeModeBindsAuthenticatedBodyApplicationAccountAndTime(t *testing.T) {
	now := time.Now()
	key := bytes.Repeat([]byte{1}, 32)
	settings := domain.RegistrationSettings{AppID: "wx-app", OfficialAccountID: "gh_account", VerificationToken: "callback-token", EncodingAESKey: strings.TrimRight(base64.StdEncoding.EncodeToString(key), "=")}
	body := []byte(fmt.Sprintf(`<xml><ToUserName>gh_account</ToUserName><FromUserName>openid</FromUserName><CreateTime>%d</CreateTime><MsgType>event</MsgType><Event>subscribe</Event><EventKey>qrscene_scene</EventKey><Ticket>ticket</Ticket></xml>`, now.Unix()))
	encrypted := encryptCallback(t, key, "wx-app", body)
	query := url.Values{"timestamp": {strconv.FormatInt(now.Unix(), 10)}, "nonce": {"nonce"}, "msg_signature": {WeChatSignature(settings.VerificationToken, strconv.FormatInt(now.Unix(), 10), "nonce", encrypted)}}
	plain, err := DecryptWeChat(settings, query, encrypted, now)
	if err != nil {
		t.Fatal(err)
	}
	event, err := ParseWeChatMessage(settings, plain, now)
	if err != nil || event.FromUserName != "openid" {
		t.Fatal("follow proof lost", err)
	}
	for _, mode := range []string{"signature", "timestamp", "application", "account", "body", "plaintext"} {
		t.Run(mode, func(t *testing.T) {
			s := settings
			q := url.Values{}
			for k, v := range query {
				q[k] = append([]string(nil), v...)
			}
			e := encrypted
			switch mode {
			case "signature":
				q.Set("msg_signature", "forged")
			case "timestamp":
				q.Set("timestamp", strconv.FormatInt(now.Add(-10*time.Minute).Unix(), 10))
			case "application":
				s.AppID = "other-app"
			case "account":
				s.OfficialAccountID = "other-account"
			case "body":
				e = encryptCallback(t, key, "wx-app", []byte("forged-body"))
			case "plaintext":
				e = ""
			}
			p, err := DecryptWeChat(s, q, e, now)
			if err == nil {
				_, err = ParseWeChatMessage(s, p, now)
			}
			if err == nil {
				t.Fatal("unsafe callback accepted")
			}
		})
	}
	scan := bytes.ReplaceAll(body, []byte("subscribe"), []byte("SCAN"))
	scan = bytes.ReplaceAll(scan, []byte("qrscene_scene"), []byte("scene"))
	if event, err = ParseWeChatMessage(settings, scan, now); err != nil || event.Event != "SCAN" {
		t.Fatal("existing follower scan rejected")
	}
}

func TestWeChatURLHandshakeCannotAuthenticateEventBodies(t *testing.T) {
	now := time.Now()
	settings := domain.RegistrationSettings{VerificationToken: "callback-token"}
	query := url.Values{"timestamp": {strconv.FormatInt(now.Unix(), 10)}, "nonce": {"nonce"}, "echostr": {"setup-challenge"}}
	query.Set("signature", WeChatSignature(settings.VerificationToken, query.Get("timestamp"), query.Get("nonce"), ""))
	echo, err := VerifyWeChatURL(settings, query, now)
	if err != nil || echo != "setup-challenge" {
		t.Fatal("URL setup rejected", err)
	}
	if _, err := DecryptWeChat(settings, query, "setup-challenge", now); err == nil {
		t.Fatal("URL handshake authenticated an event")
	}
	for _, mode := range []string{"signature", "expired", "overflow"} {
		t.Run(mode, func(t *testing.T) {
			q := url.Values{}
			for k, v := range query {
				q[k] = append([]string(nil), v...)
			}
			switch mode {
			case "signature":
				q.Set("signature", "forged")
			case "expired":
				q.Set("timestamp", strconv.FormatInt(now.Add(-6*time.Minute).Unix(), 10))
			case "overflow":
				q.Set("timestamp", "-9223372036854775808")
			}
			if mode != "signature" {
				q.Set("signature", WeChatSignature(settings.VerificationToken, q.Get("timestamp"), q.Get("nonce"), ""))
			}
			if _, err := VerifyWeChatURL(settings, q, now); err == nil {
				t.Fatal("invalid URL setup accepted")
			}
		})
	}
}
