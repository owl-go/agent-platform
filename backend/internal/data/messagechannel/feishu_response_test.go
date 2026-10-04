package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestFeishuReactionAndSingleMutableCard(t *testing.T) {
	var operations []string
	adapter := &Feishu{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "auth/v3") {
			return jsonResponse(`{"code":0,"tenant_access_token":"token"}`, 200), nil
		}
		operations = append(operations, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing authentication")
		}
		payload, _ := io.ReadAll(r.Body)
		switch r.Method {
		case http.MethodDelete:
			return jsonResponse(`{"code":0}`, 200), nil
		case http.MethodPost:
			if strings.HasSuffix(r.URL.Path, "reactions") {
				if !strings.Contains(string(payload), `"emoji_type":"Typing"`) {
					t.Fatal("not a Typing reaction")
				}
				return jsonResponse(`{"code":0,"data":{"reaction_id":"reaction"}}`, 200), nil
			}
			var body map[string]any
			_ = json.Unmarshal(payload, &body)
			if !strings.Contains(body["content"].(string), `"schema":"2.0"`) || !strings.Contains(body["content"].(string), `"tag":"markdown"`) {
				t.Fatal("reply is not Markdown", body["content"])
			}
			if body["uuid"] != "stable-inbox" || body["msg_type"] != "interactive" || body["reply_in_thread"] != true {
				t.Fatalf("unexpected card request: %s", payload)
			}
			return jsonResponse(`{"code":0,"data":{"message_id":"card"}}`, 200), nil
		case http.MethodPatch:
			var body map[string]string
			if json.Unmarshal(payload, &body) != nil || !strings.Contains(body["content"], `"update_multi":true`) || !strings.Contains(body["content"], `"schema":"2.0"`) || !strings.Contains(body["content"], `"tag":"markdown"`) || !strings.Contains(body["content"], "answer") {
				t.Fatal("invalid card update")
			}
			return jsonResponse(`{"code":0}`, 200), nil
		}
		return nil, errors.New("unexpected request")
	}))}
	ctx := context.Background()
	s := application.ChannelStored{}
	c := application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}
	m := domain.ChannelMessage{MessageID: "original", ThreadID: "thread"}
	reaction, err := adapter.React(ctx, s, c, m)
	if err != nil || reaction != "reaction" {
		t.Fatal(reaction, err)
	}
	if result := adapter.CreateResponse(ctx, s, c, m, "stable-inbox", application.ChannelResponsePreview{Status: "正在准备执行环境"}, false); result.State != "sent" || result.MessageID != "card" {
		t.Fatal(result)
	}
	for _, final := range []bool{false, true} {
		if result := adapter.UpdateResponse(ctx, s, c, domain.ChannelMessage{}, "card", application.ChannelResponsePreview{Answer: "answer", Summary: "公开摘要", Status: "正在调用工具", ToolsCompleted: 2, ElapsedSeconds: 65}, final); result.State != "sent" || result.MessageID != "card" {
			t.Fatal(result)
		}
	}
	if adapter.ClearReaction(ctx, s, c, m, reaction) != nil {
		t.Fatal("cleanup failed")
	}
	if len(operations) != 5 || operations[0] != "POST /open-apis/im/v1/messages/original/reactions" || operations[2] != "PATCH /open-apis/im/v1/messages/card" || operations[4] != "DELETE /open-apis/im/v1/messages/original/reactions/reaction" {
		t.Fatal(operations)
	}
}
func TestFeishuCardUncertaintyAndSize(t *testing.T) {
	adapter := &Feishu{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "auth/v3") {
			return jsonResponse(`{"code":0,"tenant_access_token":"token"}`, 200), nil
		}
		return nil, errors.New("connection reset after send")
	}))}
	c := application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}
	if r := adapter.CreateResponse(context.Background(), application.ChannelStored{}, c, domain.ChannelMessage{MessageID: "source"}, "uuid", application.ChannelResponsePreview{}, false); r.State != "outcome_unknown" {
		t.Fatal(r)
	}
	if r := adapter.UpdateResponse(context.Background(), application.ChannelStored{}, c, domain.ChannelMessage{}, "card", application.ChannelResponsePreview{Answer: "answer"}, true); r.State != "retry_wait" {
		t.Fatal(r)
	}
	for _, text := range []string{strings.Repeat("中文🙂", 10000), strings.Repeat("&", 10000), strings.Repeat("![", 10000), strings.Repeat("\x00\"\\<at>\n", 10000)} {
		card := feishuCardPreview(application.ChannelResponsePreview{Answer: text, Summary: text, Status: "步骤 2/2 · 正在调用工具", ToolsCompleted: 23, ElapsedSeconds: 125}, false)
		request, _ := json.Marshal(map[string]any{"content": card, "msg_type": "interactive", "uuid": "uuid"})
		if len(request) > 30*1024 || !utf8.ValidString(card) || !json.Valid([]byte(card)) {
			t.Fatal("invalid or oversized card", len(request))
		}
	}
	if NewTransports(nil)["feishu"].Response == nil {
		t.Fatal("response capability not registered")
	}
}

func TestFeishuCardKeepsPublicSummarySeparateFromAnswer(t *testing.T) {
	preview := application.ChannelResponsePreview{Answer: "最终回答", Summary: "先检查配置，再验证连接", Status: "步骤 1/2 · 正在调用工具", ToolsCompleted: 3, ElapsedSeconds: 65}
	for _, final := range []bool{false, true} {
		var card struct {
			Header struct{ Title struct{ Content string } }
			Body   struct {
				Elements []struct {
					Tag     string
					Content string
				}
			}
		}
		if json.Unmarshal([]byte(feishuCardPreview(preview, final)), &card) != nil {
			t.Fatal("invalid card")
		}
		elements := card.Body.Elements
		if len(elements) < 2 || elements[len(elements)-2].Content != "**思考摘要**\n\n先检查配置，再验证连接" || elements[len(elements)-1].Content != "**回答**\n\n最终回答" {
			t.Fatal(elements)
		}
		for _, element := range elements {
			if element.Tag != "markdown" {
				t.Fatal(element)
			}
		}
		if !final && (len(elements) != 3 || !strings.Contains(elements[0].Content, "3 次工具调用") || !strings.Contains(elements[0].Content, "1 分 5 秒") || !strings.Contains(card.Header.Title.Content, "步骤 1/2")) {
			t.Fatal(card)
		}
		if final && (len(elements) != 2 || card.Header.Title.Content != "回复") {
			t.Fatal(card)
		}
	}
}

func TestFeishuAnswerUsesMarkdownComponent(t *testing.T) {
	var card struct {
		Schema string
		Body   struct {
			Elements []struct {
				Tag     string
				Content string
			}
		}
	}
	if err := json.Unmarshal([]byte(feishuCard("**重点**", true)), &card); err != nil {
		t.Fatal(err)
	}
	if card.Schema != "2.0" || len(card.Body.Elements) != 1 || card.Body.Elements[0].Tag != "markdown" || card.Body.Elements[0].Content != "**重点**" {
		t.Fatal("reply uses plain text instead of a Markdown-capable card", card)
	}
}
