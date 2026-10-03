package messagechannel

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQQQRBindingDecryptsAuthenticatedSecretAndRejectsTampering(t *testing.T) {
	for _, tamper := range []bool{false, true} {
		t.Run(map[bool]string{false: "confirmed", true: "tampered"}[tamper], func(t *testing.T) {
			var key []byte
			adapter := &QQBot{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var body map[string]string
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Fatal("missing request")
				}
				switch r.URL.Path {
				case "/lite/create_bind_task":
					key, _ = base64.StdEncoding.DecodeString(body["key"])
					return jsonResponse(`{"retcode":0,"data":{"task_id":"task"}}`, 200), nil
				case "/lite/poll_bind_result":
					if body["task_id"] != "task" {
						t.Fatal("wrong task")
					}
					block, _ := aes.NewCipher(key)
					gcm, _ := cipher.NewGCM(block)
					nonce := make([]byte, 12)
					sealed := gcm.Seal(nonce, nonce, []byte("secret"), nil)
					if tamper {
						sealed[len(sealed)-1] ^= 1
					}
					payload, _ := json.Marshal(map[string]any{"retcode": 0, "data": map[string]any{"status": 2, "bot_appid": 12345, "bot_encrypt_secret": base64.StdEncoding.EncodeToString(sealed), "user_openid": "user"}})
					return jsonResponse(string(payload), 200), nil
				}
				t.Fatal("unexpected endpoint")
				return nil, nil
			}))}
			login, err := adapter.StartLogin(context.Background())
			if err != nil || len(key) != 32 || !strings.Contains(login.QRContent, "task_id=task") {
				t.Fatal("QR creation failed")
			}
			result, err := adapter.PollLogin(context.Background(), login.State, "")
			if tamper {
				if err == nil {
					t.Fatal("unauthenticated secret accepted")
				}
			} else if err != nil || result.Status != "connected" || result.Credentials["app_id"] != "12345" || result.Credentials["app_secret"] != "secret" || result.SuggestedSenderID != "user" {
				t.Fatal("confirmed credentials lost")
			}
		})
	}
}
func TestWeChatQRRedirectVerificationAndBoundCredentials(t *testing.T) {
	replies := []string{`{"qrcode":"private-ticket","qrcode_img_content":"https://weixin.qq.com/qr"}`, `{"status":"need_verifycode"}`, `{"status":"scaned_but_redirect","redirect_host":"ilink-test.weixin.qq.com"}`, `{"status":"confirmed","baseurl":"https://ilink-test.weixin.qq.com/","bot_token":"secret","ilink_bot_id":"bot","ilink_user_id":"user"}`, `{"ret":0}`}
	requests := 0
	adapter := &WeChat{HTTP: NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if requests == 0 {
			data, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || !strings.Contains(string(data), `"local_token_list":[]`) || r.Header.Get("Authorization") != "" || r.Header.Get("X-WECHAT-UIN") == "" {
				t.Fatal("unsafe QR request")
			}
		} else if requests < 4 {
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > 20*time.Second {
				t.Fatal("QR poll exceeds API deadline")
			}
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-WECHAT-UIN") != "" {
				t.Fatal("bot credentials sent during QR polling")
			}
			if requests == 2 && r.URL.Query().Get("verify_code") != "1234" {
				t.Fatal("verification code lost")
			}
			if requests == 3 && r.URL.Host != "ilink-test.weixin.qq.com" {
				t.Fatal("redirect ignored")
			}
		} else if r.URL.Host != "ilink-test.weixin.qq.com" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("confirmed base URL not used")
		}
		response := jsonResponse(replies[requests], 200)
		requests++
		return response, nil
	}))}
	login, err := adapter.StartLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	login, err = adapter.PollLogin(context.Background(), login.State, "")
	if err != nil || login.Status != "verification_required" {
		t.Fatal("pairing request lost")
	}
	login, err = adapter.PollLogin(context.Background(), login.State, "1234")
	if err != nil || login.Status != "scanned" {
		t.Fatal("redirect lost")
	}
	login, err = adapter.PollLogin(context.Background(), login.State, "")
	if err != nil || login.Status != "connected" {
		t.Fatal("confirmation lost")
	}
	if _, err = adapter.Identify(context.Background(), login.Credentials, ""); err != nil {
		t.Fatal(err)
	}
}
func TestWeChatQRRejectsUntrustedRedirects(t *testing.T) {
	for _, base := range []string{"http://ilinkai.weixin.qq.com", "https://ilinkai.weixin.qq.com.evil.test", "https://ilinkai.weixin.qq.com@evil.test", "https://ilinkai.weixin.qq.com:443", "https://ilinkai.weixin.qq.com/path", "https://127.0.0.1", "https://ilinkai.weixin.qq.com?secret=x"} {
		if _, err := wechatAPIBase(base); err == nil {
			t.Fatalf("untrusted origin accepted: %s", base)
		}
	}
}

func TestWeChatQRLongPollReturnsWaitingBeforeUnaryDeadlineAndHonorsCancellation(t *testing.T) {
	adapter := &WeChat{HTTP: NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	}))}
	state := map[string]string{"qrcode": "private-ticket"}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	result, err := adapter.PollLogin(ctx, state, "")
	if err != nil || result.Status != "waiting" || result.State["qrcode"] != "private-ticket" || ctx.Err() != nil {
		t.Fatal("long poll did not return reusable waiting state before API deadline")
	}
	stopped, stop := context.WithCancel(context.Background())
	stop()
	if _, err = adapter.PollLogin(stopped, state, ""); err == nil {
		t.Fatal("caller cancellation was treated as live waiting")
	}
}
