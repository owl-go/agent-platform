package messagechannel

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

func TestDingTalkCredentialsDoNotRequireOrTrustCorpID(t *testing.T) {
	calls := 0
	a := &DingTalk{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r.URL.String() != "https://api.dingtalk.com/v1.0/oauth2/accessToken" || r.Method != "POST" || len(body) != 2 || body["appKey"] != "app" || body["appSecret"] != "secret" {
			t.Fatalf("unexpected request: %s %#v", r.URL, body)
		}
		return jsonResponse(`{"accessToken":"protected-token","expireIn":7200}`, 200), nil
	}))}
	for _, corp := range []string{"", "self-reported-enterprise"} {
		c := application.ChannelCredentials{"client_id": "app", "client_secret": "secret", "corp_id": corp}
		identity, err := a.Identify(context.Background(), c, "")
		if err != nil || identity.ID != "app" || identity.BindingID != "app" || identity.TenantID != "" {
			t.Fatalf("identity: %#v %v", identity, err)
		}
		if err := a.Configure(context.Background(), application.ChannelStored{Channel: domain.MessageChannel{AccountID: "app", TenantID: "trusted-corp"}}, c, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Identify(context.Background(), application.ChannelCredentials{"client_id": "app"}, ""); err == nil {
		t.Fatal("missing secret accepted")
	}
	if calls != 4 {
		t.Fatalf("calls %d", calls)
	}
}

func TestDingTalkDiscoversCorpOnlyFromMatchingApplicationCallbacks(t *testing.T) {
	s := application.ChannelStored{Channel: domain.MessageChannel{AccountID: "app"}}
	base := chatbot.BotCallbackDataModel{ChatbotCorpId: "corp", SenderCorpId: "corp", ChatbotUserId: "bot", SenderId: "person", SenderStaffId: "staff", ConversationId: "chat", ConversationType: "1", MsgId: "message", Msgtype: "text", SessionWebhook: "https://oapi.dingtalk.com/robot/sendBySession?session=protected", CreateAt: time.Now().UnixMilli()}
	base.Text.Content = "verify code"
	cases := []struct {
		name   string
		alter  func(*chatbot.BotCallbackDataModel)
		tenant string
		want   bool
	}{
		{"discover", func(*chatbot.BotCallbackDataModel) {}, "", true},
		{"pinned", func(*chatbot.BotCallbackDataModel) {}, "corp", true},
		{"other enterprise", func(*chatbot.BotCallbackDataModel) {}, "other", false},
		{"outside sender", func(e *chatbot.BotCallbackDataModel) { e.SenderCorpId = "other" }, "", false},
		{"missing enterprise", func(e *chatbot.BotCallbackDataModel) { e.ChatbotCorpId = ""; e.SenderCorpId = "" }, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.alter(&e)
			stored := s
			stored.Channel.TenantID = tc.tenant
			m, ok := normalizeDingTalk(stored, &e)
			if ok != tc.want {
				t.Fatalf("accepted %v", ok)
			}
			if ok && (m.TenantID != "corp" || m.SenderID != "staff") {
				t.Fatalf("message %#v", m)
			}
		})
	}
}

func TestDingTalkFrameRejectsOtherApplication(t *testing.T) {
	for _, robot := range []string{"app", "other", ""} {
		event := map[string]any{"robotCode": robot, "chatbotCorpId": "corp", "senderCorpId": "corp", "chatbotUserId": "bot", "senderId": "person", "senderStaffId": "staff", "conversationId": "chat", "conversationType": "1", "msgId": "event", "msgtype": "text", "text": map[string]string{"content": "verify"}, "sessionWebhook": "https://oapi.dingtalk.com/robot/sendBySession?session=protected", "createAt": time.Now().UnixMilli()}
		body, _ := json.Marshal(event)
		frame := &payload.DataFrame{Type: "CALLBACK", Headers: payload.DataFrameHeader{payload.DataFrameHeaderKTopic: payload.BotMessageCallbackTopic, payload.DataFrameHeaderKMessageId: "frame"}, Data: string(body)}
		received := false
		_, _, err := dingTalkFrame(context.Background(), application.ChannelStored{Channel: domain.MessageChannel{AccountID: "app"}}, frame, func(_ context.Context, m domain.ChannelMessage) error {
			received = true
			if m.TenantID != "corp" {
				t.Fatal("missing trusted tenant")
			}
			return nil
		})
		if err != nil || received != (robot == "app") {
			t.Fatalf("robot %q received %v err %v", robot, received, err)
		}
	}
}
