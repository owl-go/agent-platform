package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestDingTalkContinuationAndFallbackUseNativeMarkdownInOriginalChat(t *testing.T) {
	source := "# 标题\n\n**重点**与 *强调*\n\n- 列表\n\n> 引用\n\n[链接](https://example.com)\n\n```go\nif a < b && ready { return }\n```\n\n<at user_id=\"all\">all</at> ![图片](https://private.test/image)"
	want := strings.ReplaceAll(strings.ReplaceAll(source, "<at", "&lt;at"), "</at>", "&lt;/at>")
	want = strings.ReplaceAll(want, "![图片]", "&#33;[图片]")
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "group"}[group], func(t *testing.T) {
			s, c, m := dingTalkResponseFixture()
			m.Group = group
			calls := 0
			a := &DingTalk{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodPost || r.URL.String() != m.Reply["session_webhook"] || r.Header.Get("Authorization") != "" {
					t.Fatal("reply changed its original session target")
				}
				var body struct {
					Msgtype  string
					Markdown struct{ Title, Text string }
					At       struct {
						AtUserIDs []string
						IsAtAll   bool
					}
				}
				if json.NewDecoder(r.Body).Decode(&body) != nil || body.Msgtype != "markdown" || body.Markdown.Title == "" || body.Markdown.Text != want {
					t.Fatalf("native Markdown lost formatting or code literals: %#v", body.Markdown)
				}
				if body.At.IsAtAll || len(body.At.AtUserIDs) != 1 || body.At.AtUserIDs[0] != m.SenderID {
					t.Fatal("model markup changed the reply audience")
				}
				return jsonResponse(`{"errcode":0}`, 200), nil
			}))}
			for _, sender := range []string{"continuation", "fallback"} {
				if got := a.Send(context.Background(), s, c, m, source, sender); got.State != "sent" {
					t.Fatal(got)
				}
			}
			if calls != 2 {
				t.Fatal("unexpected number of sends", calls)
			}
		})
	}
}

func TestDingTalkMarkdownPreservesReplyDeadlineAndSafeFailureStates(t *testing.T) {
	for _, test := range []struct {
		name, body, state string
		status            int
		invalid, network  bool
	}{
		{name: "expired", state: "expired", invalid: true},
		{name: "wrong-host", state: "failed", invalid: true},
		{name: "business-rejection", body: `{"errcode":400,"errmsg":"protected-detail"}`, status: 200, state: "failed"},
		{name: "rate-limit", body: `{}`, status: 429, state: "retry_wait"},
		{name: "server", body: `{}`, status: 500, state: "outcome_unknown"},
		{name: "network", network: true, state: "outcome_unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, c, m := dingTalkResponseFixture()
			if test.name == "expired" {
				m.Reply["expires_at"] = "1"
			} else if test.name == "wrong-host" {
				m.Reply["session_webhook"] = "https://attacker.test/robot/sendBySession"
			}
			a := &DingTalk{NewHTTP(roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.invalid {
					t.Fatal("invalid reply reached provider")
				}
				if test.network {
					return nil, errors.New("protected-network-detail")
				}
				return jsonResponse(test.body, test.status), nil
			}))}
			got := a.Send(context.Background(), s, c, m, "**回复**", "key")
			if got.State != test.state || strings.Contains(got.Code, "protected") {
				t.Fatal(got)
			}
		})
	}
}
