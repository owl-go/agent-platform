package messagechannel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/bwmarrin/discordgo"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func jsonResponse(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func strptr(s string) *string { return &s }

func TestTelegramAuthenticatesAndNormalizesUnicodeMention(t *testing.T) {
	a := &Telegram{NewHTTP(nil)}
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "99", AccountName: "TestBot"}}
	c := application.ChannelCredentials{"callback_secret": "correct-secret"}
	body := []byte(fmt.Sprintf(`{"update_id":42,"message":{"message_id":7,"date":%d,"from":{"id":1,"is_bot":false},"chat":{"id":-10,"type":"supergroup"},"message_thread_id":2,"text":"😀 @TestBot question","entities":[{"type":"mention","offset":3,"length":8}]}}`, time.Now().Unix()))
	h := make(http.Header)
	h.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	if _, err := a.Callback(context.Background(), s, c, h, body); err == nil {
		t.Fatal("wrong secret accepted")
	}
	h.Set("X-Telegram-Bot-Api-Secret-Token", "correct-secret")
	parsed, err := a.Callback(context.Background(), s, c, h, body)
	if err != nil || len(parsed.Messages) != 1 {
		t.Fatalf("parse: %#v %v", parsed, err)
	}
	m := parsed.Messages[0]
	if !m.Group || !m.Mentioned || m.ThreadID != "2" || m.Text != "😀  question" {
		t.Fatalf("mention: %#v", m)
	}
	edited := []byte(`{"update_id":43,"edited_message":{"text":"edit"}}`)
	parsed, err = a.Callback(context.Background(), s, c, h, edited)
	if err != nil || len(parsed.Messages) != 0 {
		t.Fatal("edit created a question")
	}
	body = []byte(strings.ReplaceAll(string(body), `"is_bot":false`, `"is_bot":true`))
	parsed, _ = a.Callback(context.Background(), s, c, h, body)
	if len(parsed.Messages) != 0 {
		t.Fatal("bot echo accepted")
	}
}

func TestDingTalkFramesAckOnlyCommittedMessagesAndBoundGateway(t *testing.T) {
	for _, endpoint := range []string{"ws://wss-open-connection.dingtalk.com/connect", "wss://evil.test/connect", "wss://wss-open-connection.dingtalk.com.evil.test/connect", "wss://user@wss-open-connection.dingtalk.com/connect", "wss://wss-open-connection.dingtalk.com:8443/connect"} {
		if _, err := dingTalkStreamURL(endpoint, "ticket"); err == nil {
			t.Fatalf("accepted endpoint %s", endpoint)
		}
	}
	target, err := dingTalkStreamURL("wss://wss-open-connection.dingtalk.com:443/connect", "ticket&other=value")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(target)
	if u.Query().Get("ticket") != "ticket&other=value" || len(u.Query()) != 1 {
		t.Fatal("ticket not escaped")
	}
	stored := application.ChannelStored{Channel: domain.MessageChannel{TenantID: "corp"}}
	event := chatbot.BotCallbackDataModel{ChatbotUserId: "bot", SenderId: "alice", Msgtype: "text", ChatbotCorpId: "corp", SenderCorpId: "corp", SenderStaffId: "alice", ConversationId: "chat", ConversationType: "1", MsgId: "message", SessionWebhook: "https://oapi.dingtalk.com/robot/sendBySession?session=secret", CreateAt: time.Now().UnixMilli()}
	event.Text.Content = "question"
	body, _ := json.Marshal(event)
	frame := &payload.DataFrame{Type: "CALLBACK", Headers: payload.DataFrameHeader{payload.DataFrameHeaderKTopic: payload.BotMessageCallbackTopic, payload.DataFrameHeaderKMessageId: "frame"}, Data: string(body)}
	for _, fail := range []bool{false, true} {
		committed := false
		ack, disconnect, err := dingTalkFrame(context.Background(), stored, frame, func(ctx context.Context, message domain.ChannelMessage) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("unbounded inbox commit")
			}
			if message.SenderID != "alice" {
				t.Fatal("incorrect sender")
			}
			if fail {
				return errors.New("database failed")
			}
			committed = true
			return nil
		})
		if disconnect || (err != nil) != fail || (ack.Code == 200) != committed {
			t.Fatalf("unsafe ACK: fail=%v committed=%v code=%d error=%v", fail, committed, ack.Code, err)
		}
	}
	frame.Type = "SYSTEM"
	frame.Headers[payload.DataFrameHeaderKTopic] = "disconnect"
	if _, disconnect, err := dingTalkFrame(context.Background(), stored, frame, nil); err != nil || !disconnect {
		t.Fatal("disconnect did not stop transport")
	}
}
func slackHeaders(body []byte, secret string, seconds int64) http.Header {
	timestamp := strconv.FormatInt(seconds, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("v0:" + timestamp + ":"))
	_, _ = mac.Write(body)
	h := make(http.Header)
	h.Set("X-Slack-Request-Timestamp", timestamp)
	h.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	return h
}
func TestSlackRawSignatureReplayTenantAndThread(t *testing.T) {
	a := &Slack{NewHTTP(nil)}
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "U_BOT", TenantID: "T_OWN"}}
	c := application.ChannelCredentials{"signing_secret": "signing-secret"}
	body := []byte(fmt.Sprintf(`{"type":"event_callback","team_id":"T_OWN","event_id":"Ev1","event":{"type":"app_mention","user":"U1","channel":"C1","ts":"%d.001","text":"<@U_BOT> question"}}`, time.Now().Unix()))
	h := slackHeaders(body, c["signing_secret"], time.Now().Unix())
	parsed, err := a.Callback(context.Background(), s, c, h, body)
	if err != nil || len(parsed.Messages) != 1 || parsed.Messages[0].ThreadID == "" || !parsed.Messages[0].Mentioned {
		t.Fatalf("parse: %#v %v", parsed, err)
	}
	if _, err := a.Callback(context.Background(), s, c, h, append(body, ' ')); err == nil {
		t.Fatal("changed raw body accepted")
	}
	if _, err := a.Callback(context.Background(), s, c, slackHeaders(body, c["signing_secret"], time.Now().Add(-10*time.Minute).Unix()), body); err == nil {
		t.Fatal("replay timestamp accepted")
	}
	other := []byte(strings.ReplaceAll(string(body), "T_OWN", "T_OTHER"))
	parsed, err = a.Callback(context.Background(), s, c, slackHeaders(other, c["signing_secret"], time.Now().Unix()), other)
	if err != nil || len(parsed.Messages) != 0 {
		t.Fatal("cross-tenant event accepted")
	}
}
func TestGatewayNormalizersRejectBotsAndTenantDrift(t *testing.T) {
	discord := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot"}}
	e := &discordgo.MessageCreate{Message: &discordgo.Message{ID: "1", ChannelID: "channel", GuildID: "guild", Content: "<@bot> question", Author: &discordgo.User{ID: "sender"}, Mentions: []*discordgo.User{{ID: "bot"}}, Timestamp: time.Now()}}
	m, ok := normalizeDiscord(discord, e)
	if !ok || !m.Group || !m.Mentioned || m.Text != "question" {
		t.Fatal("Discord mention lost")
	}
	e.Author.Bot = true
	if _, ok := normalizeDiscord(discord, e); ok {
		t.Fatal("Discord bot accepted")
	}
	ding := application.ChannelStored{Channel: domain.MessageChannel{TenantID: "corp"}}
	de := &chatbot.BotCallbackDataModel{ChatbotCorpId: "corp", SenderCorpId: "corp", ChatbotUserId: "bot", SenderId: "sender", SenderStaffId: "staff", ConversationId: "group", ConversationType: "2", Msgtype: "text", MsgId: "msg", IsInAtList: true, CreateAt: time.Now().UnixMilli(), SessionWebhook: "https://oapi.dingtalk.com/robot/sendBySession?session=protected", SessionWebhookExpiredTime: time.Now().Add(time.Hour).UnixMilli(), Text: chatbot.BotCallbackDataTextModel{Content: "question"}}
	m, ok = normalizeDingTalk(ding, de)
	if !ok || !m.Mentioned || m.SenderID != "staff" {
		t.Fatal("DingTalk identity lost")
	}
	de.SenderCorpId = "other"
	if _, ok := normalizeDingTalk(ding, de); ok {
		t.Fatal("cross-enterprise DingTalk input accepted")
	}
	de.SenderCorpId = "corp"
	de.SessionWebhook = "https://attacker.test/robot/sendBySession"
	if _, ok := normalizeDingTalk(ding, de); ok {
		t.Fatal("untrusted reply URL accepted")
	}
	fs := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "bot", TenantID: "tenant"}}
	fe := &larkim.P2MessageReceiveV1{EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "event", AppID: "app", TenantKey: "tenant"}}, Event: &larkim.P2MessageReceiveV1Data{Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: strptr("sender")}, SenderType: strptr("user"), TenantKey: strptr("tenant")}, Message: &larkim.EventMessage{MessageId: strptr("message"), ChatId: strptr("group"), ChatType: strptr("group"), MessageType: strptr("text"), CreateTime: strptr(strconv.FormatInt(time.Now().UnixMilli(), 10)), Content: strptr(`{"text":"@_user_1 question"}`), Mentions: []*larkim.MentionEvent{{Key: strptr("@_user_1"), Id: &larkim.UserId{OpenId: strptr("bot")}}}}}}
	m, ok = normalizeFeishu(fs, fe)
	if !ok || !m.Mentioned || m.Text != "question" {
		t.Fatal("Feishu mention lost")
	}
	fe.Event.Sender.TenantKey = strptr("other")
	if _, ok := normalizeFeishu(fs, fe); ok {
		t.Fatal("cross-tenant Feishu event accepted")
	}
	fe.Event.Sender.TenantKey = strptr("tenant")
	fe.EventV2Base.Header.TenantKey = "other"
	if _, ok := normalizeFeishu(fs, fe); ok {
		t.Fatal("cross-tenant Feishu header accepted")
	}
	fs.Channel.TenantID = ""
	fe.EventV2Base.Header.TenantKey = ""
	fe.Event.Sender.TenantKey = strptr("")
	if _, ok := normalizeFeishu(fs, fe); ok {
		t.Fatal("Feishu event without a verified tenant accepted")
	}
}
func TestProviderSendPreservesTargetsAndUsesSafeFailures(t *testing.T) {
	tests := []struct{ provider, reply string }{{"telegram", `{"ok":true,"result":{"message_id":99}}`}, {"slack", `{"ok":true,"ts":"123.1"}`}, {"discord", `{"id":"99"}`}, {"dingtalk", `{"errcode":0}`}, {"feishu", `{"code":0,"data":{"message_id":"99"}}`}}
	for _, test := range tests {
		t.Run(test.provider, func(t *testing.T) {
			var sent map[string]any
			var target *url.URL
			transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if strings.Contains(r.URL.Path, "tenant_access_token") {
					return jsonResponse(`{"code":0,"tenant_access_token":"protected-access-token"}`, 200), nil
				}
				target = r.URL
				data, _ := io.ReadAll(r.Body)
				if json.Unmarshal(data, &sent) != nil {
					t.Fatal("invalid send body")
				}
				return jsonResponse(test.reply, 200), nil
			})
			adapter := NewTransports(transport)[test.provider].Sender
			s := application.ChannelStored{Channel: domain.MessageChannel{Region: "feishu"}}
			c := application.ChannelCredentials{"bot_token": "test-token", "app_id": "app", "app_secret": "secret"}
			m := domain.ChannelMessage{ChatID: "123", MessageID: "456", ThreadID: "789", SenderID: "sender", Reply: map[string]string{"session_webhook": "https://oapi.dingtalk.com/robot/sendBySession?session=protected", "expires_at": strconv.FormatInt(time.Now().Add(time.Hour).UnixMilli(), 10)}}
			result := adapter.Send(context.Background(), s, c, m, "answer", "stable-key")
			if result.State != "sent" {
				t.Fatalf("send: %#v", result)
			}
			if target == nil {
				t.Fatal("no request")
			}
			if test.provider == "telegram" {
				if sent["chat_id"] != "123" || sent["message_thread_id"] != float64(789) {
					t.Fatal("Telegram target changed")
				}
				p := sent["reply_parameters"].(map[string]any)
				if p["message_id"] != float64(456) {
					t.Fatal("Telegram reply identity changed")
				}
			}
			if test.provider == "slack" && sent["thread_ts"] != "789" {
				t.Fatal("Slack thread lost")
			}
			if test.provider == "discord" && (sent["enforce_nonce"] != true || sent["allowed_mentions"] == nil) {
				t.Fatal("Discord duplicate or mention guard lost")
			}
			if test.provider == "feishu" && (sent["uuid"] != "stable-key" || sent["reply_in_thread"] != true) {
				t.Fatal("Feishu reply key lost")
			}
		})
	}
	token := "secret-token-must-not-leak"
	a := &Telegram{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: r.URL.String(), Err: errors.New(token)}
	}))}
	_, err := a.Identify(context.Background(), application.ChannelCredentials{"bot_token": token}, "")
	if err == nil || strings.Contains(err.Error(), token) {
		t.Fatal("provider error exposed token")
	}
	result := a.Send(context.Background(), application.ChannelStored{}, application.ChannelCredentials{"bot_token": token}, domain.ChannelMessage{ChatID: "1", MessageID: "2"}, "answer", "key")
	if result.State != "outcome_unknown" {
		t.Fatal("uncertain network send was retried")
	}
}
func TestOutboundRejectsUntrustedURLAndExpiredReply(t *testing.T) {
	calls := 0
	h := NewHTTP(roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return jsonResponse(`{}`, 200), nil }))
	for _, target := range []string{"http://api.telegram.org/test", "https://api.telegram.org.attacker.test/test", "https://api.telegram.org:443/test", "https://user@api.telegram.org/test", "https://127.0.0.1/test"} {
		if _, _, _, err := h.request(context.Background(), "POST", target, "", nil); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", target)
		}
	}
	if calls != 0 {
		t.Fatal("unsafe request reached transport")
	}
	result := (&DingTalk{h}).Send(context.Background(), application.ChannelStored{}, nil, domain.ChannelMessage{Reply: map[string]string{"expires_at": "0"}}, "answer", "key")
	if result.State != "expired" || calls != 0 {
		t.Fatal("expired reply sent")
	}
}
