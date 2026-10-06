package workspace

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	registrationgateway "agent-platform/backend/internal/data/account/registration"
)

func encryptedLoginMessage(t *testing.T, settings accountdomain.RegistrationSettings, body string) (*http.Request, time.Time) {
	t.Helper()
	now := time.Now()
	key, err := base64.StdEncoding.DecodeString(settings.EncodingAESKey + "=")
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, 20)
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(body)))
	plain = append(plain, body...)
	plain = append(plain, settings.AppID...)
	padding := 32 - len(plain)%32
	plain = append(plain, bytes.Repeat([]byte{byte(padding)}, padding)...)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(encrypted, plain)
	text := base64.StdEncoding.EncodeToString(encrypted)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	query := url.Values{"timestamp": {timestamp}, "nonce": {"callback-fixture"}}
	query.Set("msg_signature", registrationgateway.WeChatSignature(settings.VerificationToken, timestamp, query.Get("nonce"), text))
	return httptest.NewRequest(http.MethodPost, "/api/v1/registration/wechat_official/callback?"+query.Encode(), strings.NewReader("<xml><Encrypt>"+text+"</Encrypt></xml>")), now
}

func decryptLoginReply(t *testing.T, settings accountdomain.RegistrationSettings, response *httptest.ResponseRecorder) string {
	t.Helper()
	var envelope struct {
		Encrypt, MsgSignature, Nonce string
		TimeStamp                    int64
	}
	if err := xml.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"timestamp": {strconv.FormatInt(envelope.TimeStamp, 10)}, "nonce": {envelope.Nonce}, "msg_signature": {envelope.MsgSignature}}
	plain, err := registrationgateway.DecryptWeChat(settings, query, envelope.Encrypt, time.Now())
	if err != nil {
		t.Fatal("unsafe passive reply", err)
	}
	var message registrationgateway.WeChatMessage
	if err = xml.Unmarshal(plain, &message); err != nil {
		t.Fatal(err)
	}
	if message.ToUserName != "openid" || message.FromUserName != settings.OfficialAccountID || message.MsgType != "text" {
		t.Fatal("reply sent to wrong account")
	}
	return message.Content
}

func TestWeChatEncryptedTextConfirmsOnlyItsLoginAttempt(t *testing.T) {
	settings := accountdomain.RegistrationSettings{Provider: accountdomain.RegistrationWeChat, Enabled: true, Ready: true, AppID: "wx-fixture", OfficialAccountID: "gh_fixture", VerificationToken: "fixture-token", EncodingAESKey: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), Version: 1}
	store := &brokerTestStore{settings: settings, attempts: map[string]accountdomain.RegistrationAttempt{}}
	// No Gateway: starting this flow cannot call either token or QR creation APIs.
	app := accountapplication.NewRegistration(nil, store, nil, nil, "https://workspace.test", "https://identity.test/realms/workspace", strings.Repeat("s", 32))
	challenge := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	attempt, browser, _, err := app.Begin(t.Context(), settings.Provider, app.RedirectURI(settings.Provider), "state", "nonce", challenge)
	if err != nil {
		t.Fatal(err)
	}
	unknown := "9999"
	if attempt.LoginCode == unknown {
		unknown = "0000"
	}
	broker := &registrationBroker{app: app}
	service := &Service{registration: broker}

	for _, scenario := range []struct {
		name, kind, content string
		confirms            bool
		reply               string
	}{
		{"follow-only", "event", "<Event>subscribe</Event>", false, "一次性登录码"},
		{"old-scene", "event", "<Event>SCAN</Event><EventKey>old-attempt</EventKey><Ticket>old-ticket</Ticket>", false, ""},
		{"ordinary-text", "text", "<Content>hello</Content>", false, "完整一次性登录码"},
		{"unknown-code", "text", "<Content>" + unknown + "</Content>", false, "无效或已过期"},
		{"valid-code", "text", "<Content>" + attempt.LoginCode + "</Content><MsgId>10001</MsgId>", true, "登录已确认"},
		{"provider-retry", "text", "<Content>" + attempt.LoginCode + "</Content><MsgId>10001</MsgId>", true, "登录已确认"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			body := fmt.Sprintf("<xml><ToUserName>%s</ToUserName><FromUserName>openid</FromUserName><CreateTime>%d</CreateTime><MsgType>%s</MsgType>%s</xml>", settings.OfficialAccountID, time.Now().Unix(), scenario.kind, scenario.content)
			request, _ := encryptedLoginMessage(t, settings, body)
			response := httptest.NewRecorder()
			broker.wechatCallback(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("callback=%d %s", response.Code, response.Body.String())
			}
			if scenario.reply == "" {
				if response.Body.String() != "success" {
					t.Fatal("ignored event returned a login reply")
				}
			} else if reply := decryptLoginReply(t, settings, response); !strings.Contains(reply, scenario.reply) {
				t.Fatal("missing passive reply")
			}
			confirmed := store.attempts[attempt.ID].Identity.Subject != ""
			if confirmed != scenario.confirms {
				t.Fatal("message confirmed the wrong attempt")
			}
		})
	}
	for _, wrongBrowser := range []bool{true, false} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/registration/wechat_official/status?id="+attempt.ID, nil)
		cookie := browser
		if wrongBrowser {
			cookie = "another-browser"
		}
		request.AddCookie(registrationCookie(attempt.ID, cookie, 300))
		response := httptest.NewRecorder()
		service.registrationHTTP(response, request)
		if wrongBrowser {
			if response.Code != 410 || strings.Contains(response.Body.String(), attempt.LoginCode) {
				t.Fatal("challenge exposed to another browser")
			}
			continue
		}
		var status map[string]string
		if err = json.Unmarshal(response.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || status["login_code"] != attempt.LoginCode || status["status"] != "verified" || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("challenge not bound to initiating browser")
		}
	}
	// Authentication failures must never confirm a fresh attempt.
	fresh, _, _, err := app.Begin(t.Context(), settings.Provider, app.RedirectURI(settings.Provider), "state", "nonce", challenge)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("<xml><ToUserName>%s</ToUserName><FromUserName>openid</FromUserName><CreateTime>%d</CreateTime><MsgType>text</MsgType><Content>%s</Content></xml>", settings.OfficialAccountID, time.Now().Unix(), fresh.LoginCode)
	request, _ := encryptedLoginMessage(t, settings, body)
	query := request.URL.Query()
	query.Set("msg_signature", "forged")
	request.URL.RawQuery = query.Encode()
	response := httptest.NewRecorder()
	broker.wechatCallback(response, request)
	if response.Code != 403 || store.attempts[fresh.ID].Identity.Subject != "" {
		t.Fatal("forged callback confirmed login")
	}
	store.reserveErr = accountdomain.ErrRegistrationRateLimited
	request, _ = encryptedLoginMessage(t, settings, body)
	response = httptest.NewRecorder()
	broker.wechatCallback(response, request)
	if response.Code != 200 || !strings.Contains(decryptLoginReply(t, settings, response), "稍后再试") || store.attempts[fresh.ID].Status != "waiting" {
		t.Fatal("limited message confirmed identity or omitted encrypted reply")
	}
	// A pre-release long challenge is invalidated instead of exposed to the new UI.
	fresh2, browser2, _, err := app.Begin(t.Context(), settings.Provider, app.RedirectURI(settings.Provider), "state", "nonce", challenge)
	if err != nil {
		t.Fatal(err)
	}
	legacy := store.attempts[fresh2.ID]
	legacy.LoginCode = "AW-ABCD-EFGH-JKLM"
	store.attempts[fresh2.ID] = legacy
	request = httptest.NewRequest(http.MethodGet, "/api/v1/registration/wechat_official/status?id="+fresh2.ID, nil)
	request.AddCookie(registrationCookie(fresh2.ID, browser2, 300))
	response = httptest.NewRecorder()
	service.registrationHTTP(response, request)
	if response.Code != 410 || strings.Contains(response.Body.String(), legacy.LoginCode) {
		t.Fatal("legacy challenge was exposed")
	}
}
