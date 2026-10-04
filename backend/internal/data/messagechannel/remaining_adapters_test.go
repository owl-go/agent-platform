package messagechannel

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type memoryCursor struct {
	value     string
	saves     int
	afterSave func()
}

func (c *memoryCursor) Load(context.Context, application.ChannelStored) (string, error) {
	return c.value, nil
}
func (c *memoryCursor) Save(_ context.Context, _ application.ChannelStored, v string) error {
	c.value = v
	c.saves++
	if c.afterSave != nil {
		c.afterSave()
	}
	return nil
}
func TestPollingCommitsWholeBatchBeforeCursor(t *testing.T) {
	for _, fail := range []bool{true, false} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cursor := &memoryCursor{value: "old", afterSave: cancel}
			count := 0
			err := pollMessages(ctx, application.ChannelStored{}, cursor, func(ctx context.Context, m domain.ChannelMessage) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("unbounded commit")
				}
				count++
				if fail && count == 2 {
					return errors.New("inbox failed")
				}
				return nil
			}, func(_ context.Context, v string) ([]domain.ChannelMessage, string, error) {
				if v != "old" {
					t.Fatal("lost cursor")
				}
				return []domain.ChannelMessage{{MessageID: "1"}, {MessageID: "2"}}, "next", nil
			})
			if err == nil || count != 2 || (cursor.saves == 1) == fail {
				t.Fatalf("unsafe cursor: %d %q %v", cursor.saves, cursor.value, err)
			}
		})
	}
}
func TestApprovedEndpointsRejectBypassesAndRedirects(t *testing.T) {
	for _, target := range []string{"http://matrix.test", "https://user@matrix.test", "https://*.test", "https://matrix.test/", "https://matrix.test?token=x", "https://matrix.test/%2e%2e"} {
		if validEndpoint(target) {
			t.Fatalf("approved %s", target)
		}
	}
	for _, test := range []struct {
		ip               string
		private, allowed bool
	}{{"127.0.0.1", false, false}, {"127.0.0.1", true, true}, {"10.0.0.1", true, true}, {"169.254.169.254", true, false}, {"::ffff:169.254.169.254", true, false}, {"fe80::1", true, false}, {"8.8.8.8", false, true}} {
		if approvedAddress(netip.MustParseAddr(test.ip), test.private) != test.allowed {
			t.Fatalf("unsafe address policy: %+v", test)
		}
	}
	calls := 0
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		res := jsonResponse(`{}`, 302)
		res.Header.Set("Location", "https://evil.test/secret")
		return res, nil
	})
	h := NewHTTP(rt)
	h.approve([]ApprovedEndpoint{{URL: "https://matrix.test/base"}}, rt)
	if _, err := h.endpoint("https://matrix.test", "/sync"); err == nil {
		t.Fatal("unapproved base")
	}
	if _, _, _, err := h.request(context.Background(), "GET", "https://matrix.test/else", "Bearer secret", nil); err == nil {
		t.Fatal("path escaped approval")
	}
	_, status, _, err := h.request(context.Background(), "GET", "https://matrix.test/base/sync", "Bearer secret", nil)
	if err != nil || status != 302 || calls != 1 {
		t.Fatalf("followed redirect: %d %d %v", calls, status, err)
	}
}
func TestWhatsAppSignatureTenantChallengeAndReplyWindow(t *testing.T) {
	a := &WhatsApp{NewHTTP(nil)}
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "phone", TenantID: "waba"}}
	c := application.ChannelCredentials{"app_secret": "secret", "verify_token": "verify"}
	q := url.Values{"hub.mode": {"subscribe"}, "hub.verify_token": {"verify"}, "hub.challenge": {"123"}}
	if reply, err := a.Challenge(context.Background(), s, c, q); err != nil || reply != "123" {
		t.Fatal("bad challenge")
	}
	q.Set("hub.verify_token", "bad")
	if _, err := a.Challenge(context.Background(), s, c, q); err == nil {
		t.Fatal("unauthenticated challenge")
	}
	body := []byte(fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":"waba","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"phone_number_id":"phone"},"messages":[{"id":"wamid","from":"alice","timestamp":"%d","type":"text","text":{"body":"question"}}]}}]}]}`, time.Now().Unix()))
	sign := func(data []byte) http.Header {
		m := hmac.New(sha256.New, []byte("secret"))
		m.Write(data)
		return http.Header{"X-Hub-Signature-256": {"sha256=" + hex.EncodeToString(m.Sum(nil))}}
	}
	result, err := a.Callback(context.Background(), s, c, sign(body), body)
	if err != nil || len(result.Messages) != 1 || result.Messages[0].ChatID != "alice" {
		t.Fatalf("wrong routing: %+v %v", result, err)
	}
	if _, err := a.Callback(context.Background(), s, c, sign(body), append(body, ' ')); err == nil {
		t.Fatal("modified body accepted")
	}
	other := []byte(strings.ReplaceAll(string(body), "waba", "other"))
	result, err = a.Callback(context.Background(), s, c, sign(other), other)
	if err != nil || len(result.Messages) != 0 {
		t.Fatal("cross tenant")
	}
	if r := a.Send(context.Background(), s, c, domain.ChannelMessage{Reply: map[string]string{"expires_at": "0"}}, "answer", "key"); r.State != "expired" {
		t.Fatal("sent expired WhatsApp reply")
	}
}
func TestQQSignatureChallengeAndStablePassiveReplySequence(t *testing.T) {
	secret := "qq-secret"
	a := &QQBot{HTTP: NewHTTP(nil)}
	c := application.ChannelCredentials{"app_id": "app", "app_secret": secret}
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	body := []byte(`{"op":13,"d":{"plain_token":"plain","event_ts":"100"}}`)
	seed := []byte(strings.Repeat(secret, 32)[:32])
	private := ed25519.NewKeyFromSeed(seed)
	h := http.Header{"X-Signature-Timestamp": {stamp}, "X-Signature-Ed25519": {hex.EncodeToString(ed25519.Sign(private, append([]byte(stamp), body...)))}}
	result, err := a.Callback(context.Background(), application.ChannelStored{}, c, h, body)
	if err != nil {
		t.Fatal(err)
	}
	response := result.Response.(map[string]string)
	signature, _ := hex.DecodeString(response["signature"])
	if !ed25519.Verify(private.Public().(ed25519.PublicKey), []byte("100plain"), signature) {
		t.Fatal("invalid handshake signature")
	}
	if _, err := a.Callback(context.Background(), application.ChannelStored{}, c, h, append(body, ' ')); err == nil {
		t.Fatal("modified body accepted")
	}
	var seqs []float64
	a.HTTP = NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "bots.qq.com" {
			return jsonResponse(`{"access_token":"access"}`, 200), nil
		}
		var v map[string]any
		_ = json.NewDecoder(r.Body).Decode(&v)
		seqs = append(seqs, v["msg_seq"].(float64))
		if v["msg_id"] != "original" || r.URL.Path != "/v2/groups/group/messages" {
			t.Fatal("changed QQ reply target")
		}
		return jsonResponse(`{"id":"receipt"}`, 200), nil
	}))
	m := domain.ChannelMessage{Group: true, ChatID: "group", SenderID: "member", MessageID: "original", Reply: map[string]string{"expires_at": strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10), "delivery_chunk": "1", "delivery_kind": "waiting"}}
	for _, kind := range []string{"waiting", "answer", "answer"} {
		m.Reply["delivery_kind"] = kind
		if r := a.Send(context.Background(), application.ChannelStored{}, c, m, "answer", "key"); r.State != "sent" {
			t.Fatal(r)
		}
	}
	if fmt.Sprint(seqs) != "[1 2 2]" {
		t.Fatalf("unstable reply sequence %v", seqs)
	}
	m.Reply["delivery_chunk"] = "5"
	if r := a.Send(context.Background(), application.ChannelStored{}, c, m, "answer", "key"); r.Code != "reply_budget_exhausted" || len(seqs) != 3 {
		t.Fatal("passive quota bypass")
	}
}
func TestMatrixSendRechecksEncryptionAndPreservesThread(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(strconv.FormatBool(encrypted), func(t *testing.T) {
			sent := false
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "m.room.encryption") {
					if encrypted {
						return jsonResponse(`{"algorithm":"m.megolm.v1.aes-sha2"}`, 200), nil
					}
					return jsonResponse(`{"errcode":"M_NOT_FOUND"}`, 404), nil
				}
				sent = true
				var v map[string]any
				_ = json.NewDecoder(r.Body).Decode(&v)
				rel := v["m.relates_to"].(map[string]any)
				if rel["event_id"] != "root" || rel["m.in_reply_to"].(map[string]any)["event_id"] != "original" || !strings.HasSuffix(r.URL.Path, "/stable-key") {
					t.Fatal("Matrix thread or key lost")
				}
				return jsonResponse(`{"event_id":"receipt"}`, 200), nil
			})
			a := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://matrix.test"}}})["matrix"].Sender
			r := a.Send(context.Background(), application.ChannelStored{}, application.ChannelCredentials{"endpoint": "https://matrix.test", "access_token": "token"}, domain.ChannelMessage{ChatID: "!room:test", MessageID: "original", SenderID: "@alice:test", ThreadID: "root"}, "answer", "stable-key")
			if sent == encrypted || (r.State == "sent") == encrypted {
				t.Fatalf("unsafe encrypted send: %+v", r)
			}
		})
	}
}
func TestNewHTTPTransportsKeepOriginalReplyTargets(t *testing.T) {
	for _, provider := range []string{"whatsapp", "signal", "bluebubbles", "wechat"} {
		t.Run(provider, func(t *testing.T) {
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				var v map[string]any
				_ = json.NewDecoder(r.Body).Decode(&v)
				switch provider {
				case "whatsapp":
					if v["to"] != "alice" || v["context"].(map[string]any)["message_id"] != "original" || r.URL.Path != "/v23.0/phone/messages" {
						t.Fatal("WhatsApp target")
					}
					return jsonResponse(`{"messages":[{"id":"receipt"}]}`, 200), nil
				case "signal":
					if v["method"] == "listAccounts" {
						return jsonResponse(fmt.Sprintf(`{"id":%q,"result":[{"number":"bot","aci":"phone"}]}`, v["id"]), 200), nil
					}
					p := v["params"].(map[string]any)
					if p["recipient"].([]any)[0] != "alice" || v["id"] != "stable-key" || r.Header.Get("Authorization") != "Bearer bridge" {
						t.Fatal("Signal target/auth")
					}
					return jsonResponse(`{"jsonrpc":"2.0","id":"stable-key","result":{"timestamp":123}}`, 200), nil
				case "bluebubbles":
					if strings.HasSuffix(r.URL.Path, "/server/info") {
						return jsonResponse(`{"status":200,"data":{"server_version":"1.9","computer_id":"mac","detected_imessage":"phone"}}`, 200), nil
					}
					if v["chatGuid"] != "chat" || v["tempGuid"] != "stable-key" || r.URL.Query().Get("password") != "secret&password" {
						t.Fatal("BlueBubbles target/auth")
					}
					return jsonResponse(`{"status":200,"data":{"guid":"receipt"}}`, 200), nil
				case "wechat":
					p := v["msg"].(map[string]any)
					if p["to_user_id"] != "alice" || p["context_token"] != "protected-context" || p["client_id"] != "stable-key" || r.Header.Get("AuthorizationType") != "ilink_bot_token" {
						t.Fatal("iLink context or target")
					}
					return jsonResponse(`{"ret":0}`, 200), nil
				}
				return nil, errors.New("unexpected request")
			})
			a := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://bridge.test"}}})[provider].Sender
			c := application.ChannelCredentials{"endpoint": "https://bridge.test", "bridge_token": "bridge", "account_id": "bot", "password": "secret&password", "bot_token": "token", "access_token": "token", "phone_number_id": "phone", "graph_version": "v23.0"}
			m := domain.ChannelMessage{SenderID: "alice", ChatID: "chat", MessageID: "original", Reply: map[string]string{"context_token": "protected-context", "expires_at": strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)}}
			binding := "mac:phone"
			if provider == "signal" {
				binding = "https://bridge.test:phone"
			}
			r := a.Send(context.Background(), application.ChannelStored{Channel: domain.MessageChannel{AccountID: "phone", BindingID: binding}}, c, m, "answer", "stable-key")
			if r.State != "sent" {
				t.Fatal(r)
			}
		})
	}
}
func TestSignalSSECommitsBeforeReplayCursor(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(strconv.FormatBool(fail), func(t *testing.T) {
			cursor := &memoryCursor{value: "prior"}
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method == "POST" {
					var v map[string]any
					_ = json.NewDecoder(r.Body).Decode(&v)
					return jsonResponse(fmt.Sprintf(`{"id":%q,"result":[{"number":"bot","aci":"bot"}]}`, v["id"]), 200), nil
				}
				if r.Header.Get("Last-Event-ID") != "prior" {
					t.Fatal("lost SSE replay cursor")
				}
				res := jsonResponse("id: next\ndata: {\"method\":\"receive\",\"params\":{\"account\":\"bot\",\"envelope\":{\"sourceUuid\":\"alice\",\"dataMessage\":{\"timestamp\":123,\"message\":\"question\"}}}}\n\n", 200)
				res.Header.Set("Content-Type", "text/event-stream")
				return res, nil
			})
			a := NewTransports(rt, TransportOptions{Cursor: cursor, ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://bridge.test"}}})["signal"].StreamReceiver
			calls := 0
			err := a.Connect(context.Background(), application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot", BindingID: "https://bridge.test:bot"}}, application.ChannelCredentials{"endpoint": "https://bridge.test", "account_id": "bot", "bridge_token": "secret"}, func(ctx context.Context, m domain.ChannelMessage) error {
				calls++
				if m.SenderID != "alice" || m.ChatID != "alice" {
					t.Fatal("Signal identity")
				}
				if fail {
					return errors.New("inbox failed")
				}
				return nil
			})
			if err == nil || calls != 1 || (cursor.saves == 1) == fail {
				t.Fatal("unsafe SSE cursor advancement")
			}
		})
	}
}
func TestWeChatPollRestoresCursorAndRejectsWrongBot(t *testing.T) {
	cursor := &memoryCursor{value: "prior"}
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var v map[string]any
		_ = json.NewDecoder(r.Body).Decode(&v)
		if v["get_updates_buf"] != "prior" {
			t.Fatal("lost iLink cursor")
		}
		return jsonResponse(`{"ret":0,"get_updates_buf":"next","msgs":[{"message_id":1,"from_user_id":"alice","to_user_id":"bot","message_type":1,"message_state":2,"create_time_ms":123,"context_token":"protected","item_list":[{"type":1,"text_item":{"text":"question"}}]},{"message_id":2,"from_user_id":"alice","to_user_id":"other","message_type":1,"message_state":2,"context_token":"protected","item_list":[{"type":1,"text_item":{"text":"question"}}]}]}`, 200), nil
	})
	a := NewTransports(rt, TransportOptions{Cursor: cursor})["wechat"].StreamReceiver
	calls := 0
	err := a.Connect(context.Background(), application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}, application.ChannelCredentials{"bot_token": "token"}, func(ctx context.Context, m domain.ChannelMessage) error {
		calls++
		if m.MessageID != "1" || m.Reply["context_token"] != "protected" {
			t.Fatal("lost iLink context")
		}
		return errors.New("inbox failure")
	})
	if err == nil || calls != 1 || cursor.saves != 0 {
		t.Fatal("advanced failed iLink batch")
	}
}
func TestNormalizersRejectEchoAndUntrustedMentions(t *testing.T) {
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}
	var matrix matrixEvent
	_ = json.Unmarshal([]byte(`{"event_id":"event","sender":"alice","type":"m.room.message","content":{"msgtype":"m.text","body":"@bot question","m.mentions":{"user_ids":["other"]}}}`), &matrix)
	m, ok := normalizeMatrix(s, "room", matrix)
	if !ok || m.Mentioned {
		t.Fatal("inferred Matrix mention from text")
	}
	matrix.Sender = "bot"
	if _, ok := normalizeMatrix(s, "room", matrix); ok {
		t.Fatal("Matrix echo")
	}
	frame := wecomFrame{Cmd: "aibot_msg_callback", Body: json.RawMessage(`{"aibotid":"other","msgid":"id","msgtype":"text","chattype":"group","chatid":"group","from":{"userid":"alice"},"text":{"content":"question"}}`)}
	if _, ok := normalizeWeCom(s, frame); ok {
		t.Fatal("wrong WeCom bot")
	}
	frame.Body = json.RawMessage(strings.ReplaceAll(string(frame.Body), "other", "bot"))
	m, ok = normalizeWeCom(s, frame)
	if !ok || !m.Group || !m.Mentioned || m.ChatID != "group" {
		t.Fatal("WeCom native routing")
	}
	var bb blueMessage
	_ = json.Unmarshal([]byte(`{"guid":"id","text":"question","handle":{"address":"alice"},"chats":[{"guid":"chat","participants":[{"address":"alice"},{"address":"bob"}]}]}`), &bb)
	if _, ok := normalizeBlueBubbles(bb); ok {
		t.Fatal("accepted unsupported iMessage group")
	}
	e := yuanbaoInbound{Command: "Group.CallbackAfterSendMsg", ID: "id", From: "alice", Group: "group", Body: []yuanbaoElement{{Type: "TIMTextElem"}}}
	e.Body[0].Content.Text = "@bot question"
	m, ok = normalizeYuanbao(s, e)
	if !ok || m.Mentioned {
		t.Fatal("inferred Yuanbao mention")
	}
	e.Body = append(e.Body, yuanbaoElement{Type: "TIMCustomElem"})
	e.Body[1].Content.Data = `{"elem_type":1002,"user_id":"bot"}`
	m, ok = normalizeYuanbao(s, e)
	if !ok || !m.Mentioned {
		t.Fatal("lost native Yuanbao mention")
	}
}
func TestYuanbaoPublishedWireFrameAndSigning(t *testing.T) {
	// ConnMsg: head(1), data(2). Head: cmd_type(1), cmd(2), msg_id(4), module(5), need_ack(6).
	fixture, _ := hex.DecodeString("0a15080212047075736822026d312a0570726f7879300112020800")
	f, err := decodeYuanbaoFrame(fixture)
	if err != nil || f.kind != 2 || f.cmd != "push" || f.id != "m1" || f.module != "proxy" || !f.ack {
		t.Fatalf("published frame mismatch: %+v %v", f, err)
	}
	for _, bad := range [][]byte{{0x0a, 0xff}, make([]byte, 65537), {0x00}} {
		if _, err := decodeYuanbaoFrame(bad); err == nil {
			t.Fatal("malformed protobuf accepted")
		}
	}
	a := &Yuanbao{HTTP: NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(r.Body)
		var v map[string]string
		_ = json.Unmarshal(data, &v)
		mac := hmac.New(sha256.New, []byte("secret"))
		mac.Write([]byte(v["nonce"] + v["timestamp"] + "keysecret"))
		if v["signature"] != hex.EncodeToString(mac.Sum(nil)) || v["app_key"] != "key" || len(v["nonce"]) != 32 {
			t.Fatal("invalid Yuanbao signature")
		}
		return jsonResponse(`{"code":0,"data":{"bot_id":"bot","token":"access","source":"bot","duration":3600}}`, 200), nil
	}))}
	id, err := a.Identify(context.Background(), application.ChannelCredentials{"app_key": "key", "app_secret": "secret"}, "")
	if err != nil || id.ID != "bot" {
		t.Fatal("Yuanbao identity")
	}
}

func TestMatrixInitialSnapshotAndHistoryGapNeverExecuteOldQuestions(t *testing.T) {
	for _, since := range []string{"", "prior"} {
		t.Run(map[bool]string{true: "baseline", false: "gap"}[since == ""], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cursor := &memoryCursor{value: since, afterSave: cancel}
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("since") != since {
					t.Error("lost Matrix cursor")
				}
				return jsonResponse(`{"next_batch":"next","rooms":{"join":{"room":{"timeline":{"limited":true,"events":[{"event_id":"old","sender":"alice","type":"m.room.message","content":{"msgtype":"m.text","body":"old question"}}]}}}}}`, 200), nil
			})
			a := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://matrix.test"}}, Cursor: cursor})["matrix"].StreamReceiver
			err := a.Connect(ctx, application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot", Audience: domain.ChannelAudience{GroupIDs: []string{"room"}}}}, application.ChannelCredentials{"endpoint": "https://matrix.test", "access_token": "access"}, func(context.Context, domain.ChannelMessage) error { t.Error("old question executed"); return nil })
			if err == nil || (cursor.saves == 1) != (since == "") {
				t.Fatal("unsafe baseline or history gap advancement")
			}
		})
	}
}
func TestBlueBubblesPagesSameTimestampAndFencesAccountChanges(t *testing.T) {
	pages := 0
	sameTime := time.Now().Add(-time.Minute).UnixMilli()
	cursor := &memoryCursor{value: "100"}
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "account", BindingID: "mac:account"}}
	c := application.ChannelCredentials{"endpoint": "https://bridge.test", "password": "secret"}
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/server/info") {
			return jsonResponse(`{"status":200,"data":{"server_version":"1.9","computer_id":"mac","detected_imessage":"account"}}`, 200), nil
		}
		var v map[string]any
		_ = json.NewDecoder(r.Body).Decode(&v)
		where := v["where"].([]any)[0].(map[string]any)
		if strings.Contains(where["statement"].(string), "MAX(ROWID)") {
			return jsonResponse(`{"status":200,"data":[{"originalROWID":201}]}`, 200), nil
		}
		args := where["args"].(map[string]any)
		if v["offset"] != float64(pages*100) || args["after"] != float64(100) || args["through"] != float64(201) || v["sort"] != "ASC" {
			t.Error("unsafe pagination")
		}
		pages++
		count := 100
		if pages == 2 {
			count = 1
		}
		data := make([]map[string]any, count)
		for i := range data {
			data[i] = map[string]any{"guid": fmt.Sprintf("msg-%d-%d", pages, i), "text": "question", "dateCreated": sameTime, "originalROWID": 100 + (pages-1)*100 + i + 1, "handle": map[string]string{"address": "alice"}, "chats": []any{map[string]any{"guid": "iMessage;-;alice", "participants": []any{map[string]string{"address": "alice"}}}}}
		}
		b, _ := json.Marshal(map[string]any{"status": 200, "data": data})
		return jsonResponse(string(b), 200), nil
	})
	a := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://bridge.test"}}, Cursor: cursor})["bluebubbles"].StreamReceiver
	calls := 0
	err := a.Connect(context.Background(), s, c, func(context.Context, domain.ChannelMessage) error {
		calls++
		if calls == 101 {
			return errors.New("last Inbox failed")
		}
		return nil
	})
	if err == nil || calls != 101 || pages != 2 || cursor.saves != 0 {
		t.Fatal("dropped same-timestamp page or advanced failed batch")
	}
	rt = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"status":200,"data":{"server_version":"1.9","computer_id":"mac","detected_imessage":"other-account"}}`, 200), nil
	})
	sender := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://bridge.test"}}})["bluebubbles"].Sender
	if r := sender.Send(context.Background(), s, c, domain.ChannelMessage{ChatID: "chat"}, "answer", "key"); r.State != "failed" || r.Code != "provider_identity_changed" {
		t.Fatal("sent to changed iMessage account")
	}
}

func TestLongPollEmptyResponsesAdvanceCursorWithoutQuestions(t *testing.T) {
	for _, provider := range []string{"matrix", "wechat"} {
		t.Run(provider, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cursor := &memoryCursor{value: "prior", afterSave: cancel}
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				if provider == "matrix" {
					return jsonResponse(`{"next_batch":"next"}`, 200), nil
				}
				return jsonResponse(`{"ret":0,"get_updates_buf":"next"}`, 200), nil
			})
			a := NewTransports(rt, TransportOptions{ApprovedEndpoints: []ApprovedEndpoint{{URL: "https://matrix.test"}}, Cursor: cursor})[provider].StreamReceiver
			err := a.Connect(ctx, application.ChannelStored{}, application.ChannelCredentials{"endpoint": "https://matrix.test", "bot_token": "token"}, func(context.Context, domain.ChannelMessage) error {
				t.Fatal("empty response created question")
				return nil
			})
			if !errors.Is(err, context.Canceled) || cursor.value != "next" || cursor.saves != 1 {
				t.Fatalf("idle poll mistaken for error: %q %v", cursor.value, err)
			}
		})
	}
}
