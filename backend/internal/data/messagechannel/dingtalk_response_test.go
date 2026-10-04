package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func dingTalkResponseFixture() (application.ChannelStored, application.ChannelCredentials, domain.ChannelMessage) {
	return application.ChannelStored{Channel: domain.MessageChannel{AccountID: "app", TenantID: "corp"}}, application.ChannelCredentials{"client_id": "app", "client_secret": "secret"}, domain.ChannelMessage{TenantID: "corp", SenderID: "staff", ChatID: "chat", Reply: map[string]string{"session_webhook": "https://oapi.dingtalk.com/robot/sendBySession?session=protected", "expires_at": "4102444800000"}}
}

func TestDingTalkResponseReceiptStreamAndFinalTargetSameCard(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "group"}[group], func(t *testing.T) {
			s, c, m := dingTalkResponseFixture()
			m.Group = group
			var cardID string
			calls := 0
			a := &DingTalk{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/v1.0/oauth2/accessToken" {
					return jsonResponse(`{"accessToken":"protected-token"}`, 200), nil
				}
				if r.URL.Hostname() != "api.dingtalk.com" || r.Header.Get("x-acs-dingtalk-access-token") != "protected-token" || r.Header.Get("Authorization") != "" {
					t.Fatal("card request used the wrong token or endpoint")
				}
				var body map[string]json.RawMessage
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Fatal("invalid card request")
				}
				if calls == 0 {
					cardID = rawString(body["cardBizId"])
					if r.Method != "POST" || r.URL.Path != "/v1.0/im/v1.0/robot/interactiveCards/send" || rawString(body["cardTemplateId"]) != "StandardCard" || rawString(body["robotCode"]) != "app" || rawBool(body["pullStrategy"]) {
						t.Fatal("creation did not follow official StandardCard protocol")
					}
					if group {
						if rawString(body["openConversationId"]) != "chat" || body["singleChatReceiver"] != nil {
							t.Fatal("group answer escaped its originating chat")
						}
						var options struct {
							AtAll          bool
							AtUserListJson string
						}
						_ = json.Unmarshal(body["sendOptions"], &options)
						if options.AtAll || options.AtUserListJson != `[{"userId":"staff"}]` {
							t.Fatal("group reply did not identify only its questioner")
						}
					} else if rawString(body["singleChatReceiver"]) != `{"userId":"staff"}` || body["openConversationId"] != nil {
						t.Fatal("direct card did not target the authenticated Staff ID")
					}
				} else if r.Method != "PUT" || r.URL.Path != "/v1.0/im/robots/interactiveCards" || rawString(body["cardBizId"]) != cardID {
					t.Fatal("progress/final update created another card")
				}
				var card struct {
					Header   struct{ Title struct{ Text string } }
					Config   struct{ EnableForward bool }
					Contents []map[string]string
				}
				if json.Unmarshal([]byte(rawString(body["cardData"])), &card) != nil || card.Config.EnableForward {
					t.Fatal("invalid or forwardable reply card")
				}
				if calls == 0 && (card.Header.Title.Text != "⏳ 正在处理" || len(card.Contents) != 1 || !strings.Contains(card.Contents[0]["text"], "已收到")) {
					t.Fatal("receipt did not immediately show processing")
				}
				if calls == 1 && (len(card.Contents) != 3 || !strings.Contains(card.Contents[0]["text"], "3 次工具调用") || card.Contents[2]["type"] != "markdown") {
					t.Fatal("streaming card lost real progress, summary or Markdown")
				}
				if calls == 2 && (card.Header.Title.Text != "回复" || len(card.Contents) != 2 || card.Contents[1]["text"] != "**最终答案**\n\n- 完成") {
					t.Fatal("terminal card retained typing or lost the answer")
				}
				calls++
				return jsonResponse(`{"processQueryKey":"confirmed"}`, 200), nil
			}))}
			ctx := context.Background()
			receipt := a.CreateResponse(ctx, s, c, m, "inbox", application.ChannelResponsePreview{Status: "已收到，正在准备执行"}, false)
			if receipt.State != "sent" || !strings.HasPrefix(receipt.MessageID, cardID+".") {
				t.Fatal(receipt)
			}
			preview := application.ChannelResponsePreview{Answer: "**正在输出**", Summary: "公开摘要", Status: "正在调用工具", ToolsCompleted: 3, ElapsedSeconds: 65}
			if result := a.UpdateResponse(ctx, s, c, domain.ChannelMessage{}, receipt.MessageID, preview, false); result.State != "sent" || result.MessageID != receipt.MessageID {
				t.Fatal(result)
			}
			preview.Answer = "**最终答案**\n\n- 完成"
			if result := a.UpdateResponse(ctx, s, c, domain.ChannelMessage{}, receipt.MessageID, preview, true); result.State != "sent" || calls != 3 {
				t.Fatal(result)
			}
		})
	}
}

func TestDingTalkCardFailuresDistinguishCreationUncertaintyFromSafeUpdateRetry(t *testing.T) {
	for _, test := range []struct {
		name, body, create, update string
		status                     int
		network                    bool
	}{
		{"rejected", `{"code":"Forbidden","message":"protected-secret"}`, "failed", "failed", 403, false},
		{"limited", `{}`, "retry_wait", "retry_wait", 429, false},
		{"server", `{}`, "outcome_unknown", "retry_wait", 500, false},
		{"missing-receipt", `{}`, "outcome_unknown", "retry_wait", 200, false},
		{"malformed", `not-json`, "outcome_unknown", "retry_wait", 200, false},
		{"network", `{}`, "outcome_unknown", "retry_wait", 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := &DingTalk{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/v1.0/oauth2/accessToken" {
					return jsonResponse(`{"accessToken":"protected-token"}`, 200), nil
				}
				if test.network {
					return nil, errors.New("protected-network-detail")
				}
				return jsonResponse(test.body, test.status), nil
			}))}
			s, c, m := dingTalkResponseFixture()
			create := a.CreateResponse(context.Background(), s, c, m, "inbox", application.ChannelResponsePreview{}, false)
			update := a.UpdateResponse(context.Background(), s, c, domain.ChannelMessage{}, strings.Repeat("a", 64)+".4102444800000", application.ChannelResponsePreview{}, false)
			if create.State != test.create || update.State != test.update || strings.Contains(create.Code+update.Code, "protected") || create.MessageID != "" {
				t.Fatalf("unsafe classification: %v %v", create, update)
			}
		})
	}
}

func TestDingTalkCardCannotExtendReplyDeadlineOrChangeIdentity(t *testing.T) {
	a := &DingTalk{NewHTTP(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid reply reached provider")
		return nil, nil
	}))}
	for _, change := range []string{"expired", "tenant", "app", "webhook"} {
		s, c, m := dingTalkResponseFixture()
		switch change {
		case "expired":
			m.Reply["expires_at"] = "1"
		case "tenant":
			m.TenantID = "another-corp"
		case "app":
			c["client_id"] = "another-app"
		case "webhook":
			m.Reply["session_webhook"] = "https://attacker.test/robot/sendBySession"
		}
		if a.CreateResponse(context.Background(), s, c, m, "inbox", application.ChannelResponsePreview{}, false).State == "sent" {
			t.Fatal("unsafe creation", change)
		}
	}
	s, c, _ := dingTalkResponseFixture()
	for _, handle := range []string{"invalid", strings.Repeat("a", 64) + ".1", strings.Repeat("z", 64) + ".4102444800000"} {
		if a.UpdateResponse(context.Background(), s, c, domain.ChannelMessage{}, handle, application.ChannelResponsePreview{}, false).State == "sent" {
			t.Fatal("unsafe update")
		}
	}
	if NewTransports(nil)["dingtalk"].Response == nil || NewTransports(nil)["dingtalk"].ResponseFallback == nil {
		t.Fatal("card/fallback not registered")
	}
}

func TestDingTalkCardBoundsPublicPreviewAndEscapesModelMarkup(t *testing.T) {
	preview := application.ChannelResponsePreview{Answer: "<at user_id=\"all\">all</at> ![image](https://private.test/image)\n```html\n<literal>\n```", Summary: strings.Repeat("公开", 400)}
	card := dingTalkCardPreview(preview, false)
	var rendered struct{ Contents []map[string]string }
	if json.Unmarshal([]byte(card), &rendered) != nil {
		t.Fatal("invalid card")
	}
	answer := rendered.Contents[2]["text"]
	if strings.Contains(answer, "<at") || strings.Contains(answer, "![image]") || !strings.Contains(answer, "&lt;at") || !strings.Contains(rendered.Contents[1]["text"], "思考摘要") || !strings.Contains(answer, "<literal>") {
		t.Fatal("unsafe or flattened Markdown", answer)
	}
	preview.Answer, preview.Summary = strings.Repeat("\u2028", 3000), strings.Repeat("\u2028", 1000)
	body, _ := json.Marshal(map[string]string{"cardData": dingTalkCardPreview(preview, true)})
	if len(body) > 28*1024 {
		t.Fatal("card exceeded conservative payload budget")
	}
}
