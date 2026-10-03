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

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
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

// The live iLink confirmation succeeded, while getconfig returned ret=-4.
// getconfig is conversation configuration, not QR account identity verification.
type qrIdentityRepository struct {
	application.MessageChannelRepository
	stored application.ChannelStored
}

func (r *qrIdentityRepository) ListMessageChannels(context.Context, string, string) ([]domain.MessageChannel, error) {
	return nil, nil
}
func (r *qrIdentityRepository) GetMessageChannel(context.Context, string, string, string) (application.ChannelStored, error) {
	return r.stored, nil
}
func (r *qrIdentityRepository) SaveMessageChannel(_ context.Context, c application.ChannelStored, _ int64) (application.ChannelStored, error) {
	r.stored = c
	return c, nil
}
func TestWeChatConfirmedQRDoesNotRequireConversationConfig(t *testing.T) {
	ctx := context.Background()
	configs := 0
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			return jsonResponse(`{"qrcode":"ticket","qrcode_img_content":"https://weixin.qq.com/qr"}`, 200), nil
		case "/ilink/bot/get_qrcode_status":
			return jsonResponse(`{"ret":0,"status":"confirmed","baseurl":"https://ilinkai.weixin.qq.com","bot_token":"secret","ilink_bot_id":"bot","ilink_user_id":"scanner"}`, 200), nil
		case "/ilink/bot/getconfig":
			configs++
			return jsonResponse(`{"ret":-4}`, 200), nil
		default:
			t.Fatal("unexpected endpoint")
			return nil, nil
		}
	})
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := &qrIdentityRepository{}
	transports := NewTransports(rt)
	app := application.NewMessageChannels(repo, box, transports, true, "https://workspace.test")
	login, err := app.StartLogin(ctx, "owner", "workflow", "wechat", "", "qr", "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.CancelLogin(ctx, "owner", "workflow", login.ID) })
	connected, err := app.PollLogin(ctx, "owner", "workflow", login.ID, "")
	if err != nil || connected.Status != "connected" {
		t.Fatalf("official confirmation was rejected: %v", err)
	}
	input := domain.MessageChannel{Provider: "wechat", Name: "WeChat", Audience: domain.ChannelAudience{SenderIDs: []string{"scanner"}, AllowDirect: true}}
	saved, err := app.SaveWithLogin(ctx, "owner", "workflow", "", 0, input, login.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Enabled || saved.ValidationState != "unverified" || saved.AccountID != "bot" {
		t.Fatal("QR authorization enabled or misidentified a channel")
	}
	raw, err := box.Decrypt(repo.stored.Ciphertext, application.ChannelCredentialAAD(saved))
	if err != nil {
		t.Fatal(err)
	}
	var credentials application.ChannelCredentials
	if json.Unmarshal(raw, &credentials) != nil {
		t.Fatal("invalid credentials")
	}
	if strings.Contains(string(repo.stored.Ciphertext), "secret") {
		t.Fatal("credentials not encrypted")
	}
	receiver := transports["wechat"].StreamReceiver
	if err = receiver.Configure(ctx, repo.stored, credentials, ""); err != nil {
		t.Fatal("receive validation still requires conversation config", err)
	}
	saved, err = app.Save(ctx, "owner", "workflow", saved.ID, saved.Version, input, nil)
	if err != nil {
		t.Fatal("editing retained account failed", err)
	}
	if configs != 0 {
		t.Fatal("QR flow called optional conversation config")
	}
	// Server-created authorization metadata cannot be supplied as raw credentials.
	if _, err = app.StartLogin(ctx, "owner", "workflow", "wechat", "", "credentials", "", 0, credentials); err == nil {
		t.Fatal("client forged a QR-confirmed account")
	}
	if _, err = app.Save(ctx, "owner", "workflow", "", 0, input, credentials); err == nil {
		t.Fatal("raw credentials forged a QR-confirmed account")
	}
	if _, err = app.Save(ctx, "owner", "workflow", saved.ID, saved.Version, input, credentials); err == nil {
		t.Fatal("editing accepted client authorization metadata")
	}
	credentials["account_id"] = "other"
	if _, err = transports["wechat"].Account.Identify(ctx, credentials, ""); err == nil {
		t.Fatal("QR binding accepted another account")
	}
}

func TestWeChatQRRejectsFailedConfirmationAndLegacyIdentity(t *testing.T) {
	adapter := &WeChat{HTTP: NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/ilink/bot/get_qrcode_status" {
			return jsonResponse(`{"ret":-4,"status":"confirmed","bot_token":"secret","ilink_bot_id":"bot","ilink_user_id":"scanner"}`, 200), nil
		}
		return jsonResponse(`{"ret":-4}`, 200), nil
	}))}
	if _, err := adapter.PollLogin(context.Background(), map[string]string{"qrcode": "ticket"}, ""); err == nil {
		t.Fatal("provider rejected QR confirmation was accepted")
	}
	c := application.ChannelCredentials{"bot_token": "secret", "account_id": "bot", "user_id": "scanner"}
	if _, err := adapter.Identify(context.Background(), c, ""); err == nil {
		t.Fatal("unattested raw credentials bypassed legacy identity validation")
	}
	c[wechatQRAccountIdentityKey] = "bot"
	c["baseurl"] = "https://evil.test"
	if _, err := adapter.Identify(context.Background(), c, ""); err == nil {
		t.Fatal("attestation bypassed trusted API host validation")
	}
	c["baseurl"] = "https://ilinkai.weixin.qq.com"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := adapter.Identify(ctx, c, ""); err == nil {
		t.Fatal("cancelled identity check succeeded")
	}
}
