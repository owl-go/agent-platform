package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func wecomResponseFixture() (application.ChannelStored, application.ChannelCredentials, domain.ChannelMessage) {
	return application.ChannelStored{Channel: domain.MessageChannel{ID: "channel", AccountID: "bot", ConfigVersion: 1}}, application.ChannelCredentials{"bot_id": "bot", "bot_secret": "test-secret"}, domain.ChannelMessage{MessageID: "message", SenderID: "alice", ChatID: "alice", Reply: map[string]string{"req_id": "callback-capability", "received_at": strconv.FormatInt(time.Now().UnixMilli(), 10)}}
}

func TestWeComReceiptInsideSinkStreamsMarkdownWithoutBlockingReader(t *testing.T) {
	socket := newFakeSocket()
	s, c, _ := wecomResponseFixture()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	var streamID string
	updates := 0
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		if frame.Cmd == "aibot_subscribe" {
			socket.incoming <- []byte(`{"headers":{"req_id":"` + frame.Headers.ID + `"},"errcode":0}`)
			socket.incoming <- []byte(`{"cmd":"aibot_msg_callback","headers":{"req_id":"original-callback"},"body":{"msgid":"message","aibotid":"bot","chattype":"single","msgtype":"text","from":{"userid":"alice"},"text":{"content":"question"}}}`)
			return
		}
		if frame.Cmd != "aibot_respond_msg" || frame.Headers.ID != "original-callback" {
			t.Error("stream did not use the original callback", frame.Cmd, frame.Headers.ID)
		}
		var body struct {
			Type   string `json:"msgtype"`
			Stream struct {
				ID      string `json:"id"`
				Finish  bool   `json:"finish"`
				Content string `json:"content"`
			} `json:"stream"`
		}
		_ = json.Unmarshal(frame.Body, &body)
		if streamID == "" {
			streamID = body.Stream.ID
		}
		if body.Type != "stream" || streamID != body.Stream.ID || body.Stream.Finish != (updates == 2) {
			t.Error("reply was duplicated or not finalized", body)
		}
		if updates == 0 && !strings.Contains(body.Stream.Content, "正在分析") {
			t.Error("no immediate analysis receipt")
		}
		if updates > 0 && (!strings.Contains(body.Stream.Content, "**加粗**") || !strings.Contains(body.Stream.Content, "思考摘要") || strings.Contains(body.Stream.Content, "<at")) {
			t.Error("lost Markdown or allowed an active mention", body.Stream.Content)
		}
		updates++
		socket.incoming <- []byte(`{"headers":{"req_id":"unrelated"},"errcode":400}`)
		socket.incoming <- []byte(`{"headers":{"req_id":"` + frame.Headers.ID + `"},"errcode":0}`)
	}
	err := a.Connect(ctx, s, c, func(commit context.Context, m domain.ChannelMessage) error {
		created := a.CreateResponse(commit, s, c, m, "inbox", application.ChannelResponsePreview{}, false)
		if created.State != "sent" || strings.Contains(created.MessageID, m.Reply["req_id"]) {
			t.Error("receipt deadlocked or persisted callback capability", created)
			return errors.New("receipt failed")
		}
		preview := application.ChannelResponsePreview{Status: "正在调用工具", ToolsCompleted: 2, ElapsedSeconds: 65, Summary: "公开摘要", Answer: "# 标题\n\n**加粗**\n\n```go\nfmt.Println(1)\n```\n<at user_id=\"all\">all</at>"}
		for _, final := range []bool{false, true} {
			if result := a.UpdateResponse(commit, s, c, m, created.MessageID, preview, final); result.State != "sent" || result.MessageID != created.MessageID {
				t.Error("failed cumulative update", result)
			}
		}
		cancel()
		return nil
	})
	if err == nil || updates != 3 || a.bindings.get(s) != nil {
		t.Fatal("stream or cancellation failed", err, updates)
	}
	transport := NewTransports(nil)["wecom"]
	if transport.Response == nil || transport.ResponseFallback == nil {
		t.Fatal("stream transport not registered")
	}
	if _, ok := transport.Response.(application.ChannelReactionSender); ok {
		t.Fatal("invented unsupported Typing reaction")
	}
}

func TestWeComStreamRejectsInvalidIdentityExpiryAndForeignHandleBeforeWriting(t *testing.T) {
	for _, mode := range []string{"expired", "foreign-bot", "missing-callback", "private-chat-changed", "foreign-handle", "future-clock", "oversized-final"} {
		t.Run(mode, func(t *testing.T) {
			s, c, m := wecomResponseFixture()
			id, deadline, _ := wecomStreamIdentity(s, c, m)
			handle := id + "." + strconv.FormatInt(deadline, 10)
			preview := application.ChannelResponsePreview{Answer: "answer"}
			switch mode {
			case "expired":
				m.Reply["received_at"] = strconv.FormatInt(time.Now().Add(-10*time.Minute).UnixMilli(), 10)
				id, deadline, _ = wecomStreamIdentity(s, c, m)
				handle = id + "." + strconv.FormatInt(deadline, 10)
			case "foreign-bot":
				c["bot_id"] = "other"
			case "missing-callback":
				delete(m.Reply, "req_id")
			case "private-chat-changed":
				m.ChatID = "other"
			case "foreign-handle":
				handle = strings.Repeat("a", 64) + "." + strconv.FormatInt(deadline, 10)
			case "future-clock":
				m.Reply["received_at"] = strconv.FormatInt(time.Now().Add(time.Hour).UnixMilli(), 10)
			case "oversized-final":
				preview.Answer = strings.Repeat("字", 1801)
			}
			result := (&WeCom{}).UpdateResponse(context.Background(), s, c, m, handle, preview, true)
			if mode == "expired" {
				if result.State != "expired" || result.Code != "provider_stream_expired" {
					t.Fatal(result)
				}
			} else if result.State != "failed" || result.Code != "provider_reply_invalid" {
				t.Fatal(result)
			}
		})
	}
}

func TestWeComUnknownStreamACKClosesConnectionAndNeverConfirmsFinal(t *testing.T) {
	s, c, m := wecomResponseFixture()
	socket := newFakeSocket()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := newSocketSession(ctx, socket)
	a := &WeCom{}
	defer a.bindings.add(s, session)()
	requestCtx, stop := context.WithTimeout(ctx, 10*time.Millisecond)
	defer stop()
	result := a.CreateResponse(requestCtx, s, c, m, "inbox", application.ChannelResponsePreview{}, false)
	if result.State != "outcome_unknown" {
		t.Fatal(result)
	}
	select {
	case <-socket.closed:
	default:
		t.Fatal("late ACK remained able to confirm another frame")
	}
	if len(session.pending) != 0 {
		t.Fatal("pending request leaked")
	}
}

func TestWeComStreamMarkdownBudgetAndCodeLiterals(t *testing.T) {
	for _, text := range []string{strings.Repeat("字", 1800), strings.Repeat("<&", 900), strings.Repeat("\x00", 1800), "```\n<at> ![image](https://example.com)\n```"} {
		content := wecomStreamPreview(application.ChannelResponsePreview{Answer: text, Summary: strings.Repeat("<&", 300), Status: strings.Repeat("<&", 60), ToolsCompleted: 2, ElapsedSeconds: 65}, false)
		if len(content) > 20480 || !utf8.ValidString(content) {
			t.Fatal("stream exceeds official byte limit")
		}
		if strings.HasPrefix(text, "```") && !strings.Contains(content, text) {
			t.Fatal("code literal changed")
		}
	}
}

func TestWeComStreamInitialReplyDeadlineDoesNotShortenExistingStream(t *testing.T) {
	s, c, m := wecomResponseFixture()
	m.Reply["received_at"] = strconv.FormatInt(time.Now().Add(-10*time.Second).UnixMilli(), 10)
	a := &WeCom{}
	result := a.CreateResponse(context.Background(), s, c, m, "inbox", application.ChannelResponsePreview{}, false)
	if result.State != "expired" || result.Code != "provider_stream_expired" {
		t.Fatal("late callback started a stream", result)
	}
	id, deadline, _ := wecomStreamIdentity(s, c, m)
	result = a.UpdateResponse(context.Background(), s, c, m, id+"."+strconv.FormatInt(deadline, 10), application.ChannelResponsePreview{}, true)
	if result.Code != "provider_disconnected" {
		t.Fatal("existing stream incorrectly used initial reply deadline", result)
	}
}

func TestWeComStreamACKFailuresNeverPretendSuccess(t *testing.T) {
	for _, test := range []struct {
		code   *int
		state  string
		closed bool
	}{
		{nil, "outcome_unknown", true},
		{func() *int { v := 45009; return &v }(), "retry_wait", false},
		{func() *int { v := 400001; return &v }(), "failed", false},
	} {
		s, c, m := wecomResponseFixture()
		socket := newFakeSocket()
		session := newSocketSession(context.Background(), socket)
		socket.onWrite = func(data []byte) {
			var frame wecomFrame
			_ = json.Unmarshal(data, &frame)
			receipt := map[string]any{"headers": map[string]string{"req_id": frame.Headers.ID}}
			if test.code != nil {
				receipt["errcode"] = *test.code
			}
			encoded, _ := json.Marshal(receipt)
			session.respond(frame.Headers.ID, encoded)
		}
		a := &WeCom{}
		remove := a.bindings.add(s, session)
		result := a.CreateResponse(context.Background(), s, c, m, "inbox", application.ChannelResponsePreview{}, false)
		remove()
		if result.State != test.state {
			t.Fatal(result)
		}
		select {
		case <-socket.closed:
			if !test.closed {
				t.Fatal("definite rejection unnecessarily closed socket")
			}
		default:
			if test.closed {
				t.Fatal("invalid receipt allowed subsequent ACK reuse")
			}
		}
	}
}

func TestWeComStreamWaitingForPriorACKCanBeCancelledWithoutSending(t *testing.T) {
	s, c, m := wecomResponseFixture()
	socket := newFakeSocket()
	session := newSocketSession(context.Background(), socket)
	session.replyGate <- struct{}{}
	a := &WeCom{}
	defer a.bindings.add(s, session)()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := a.CreateResponse(ctx, s, c, m, "inbox", application.ChannelResponsePreview{}, false)
	if result.State != "retry_wait" || len(socket.outgoing) != 0 {
		t.Fatal("cancelled lock wait wrote or claimed unknown delivery", result)
	}
}

func TestWeComCallbackBackpressureCancelsSinkAndReleasesReader(t *testing.T) {
	socket := newFakeSocket()
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		socket.incoming <- []byte(`{"headers":{"req_id":"` + frame.Headers.ID + `"},"errcode":0}`)
	}
	s, c, _ := wecomResponseFixture()
	a := &WeCom{dial: func(context.Context, string, http.Header) (channelSocket, error) { return socket, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- a.ConnectWithHealth(ctx, s, c, func(commit context.Context, _ domain.ChannelMessage) error { <-commit.Done(); return commit.Err() }, func(context.Context, string) error { close(ready); return nil })
	}()
	<-ready
	go func() {
		for i := 0; i < 20; i++ {
			select {
			case socket.incoming <- []byte(`{"cmd":"aibot_msg_callback","body":{"msgid":"m","aibotid":"bot","chattype":"single","msgtype":"text","from":{"userid":"alice"},"text":{"content":"q"}}}`):
			case <-socket.closed:
				return
			}
		}
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("overload was hidden")
		}
	case <-ctx.Done():
		t.Fatal("overload did not cancel blocked sink")
	}
	if a.bindings.get(s) != nil {
		t.Fatal("overloaded receiver left sender bound")
	}
}

func TestWeComOrdinaryMarkdownPreservesFormattingAndNeutralizesMentions(t *testing.T) {
	s, _, m := wecomResponseFixture()
	socket := newFakeSocket()
	session := newSocketSession(context.Background(), socket)
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		var body struct {
			Type     string `json:"msgtype"`
			ChatID   string `json:"chatid"`
			Markdown struct {
				Content string `json:"content"`
			} `json:"markdown"`
		}
		_ = json.Unmarshal(frame.Body, &body)
		if frame.Cmd != "aibot_send_msg" || body.Type != "markdown" || body.ChatID != "alice" || !strings.Contains(body.Markdown.Content, "**加粗**") || strings.Contains(body.Markdown.Content, "<at") || !strings.Contains(body.Markdown.Content, "`<code>`") {
			t.Error("unsafe or flattened Markdown", string(frame.Body))
		}
		session.respond(frame.Headers.ID, []byte(`{"errcode":0}`))
	}
	a := &WeCom{}
	defer a.bindings.add(s, session)()
	if result := a.Send(context.Background(), s, nil, m, "# 标题\n\n**加粗** `<code>` <at>all</at>", "delivery"); result.State != "sent" {
		t.Fatal(result)
	}
}

func TestWeComGroupStreamUsesOriginalCallbackAndLabelsQuestioner(t *testing.T) {
	s, c, m := wecomResponseFixture()
	m.Group = true
	m.ChatID = "original-group"
	socket := newFakeSocket()
	session := newSocketSession(context.Background(), socket)
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		var body map[string]json.RawMessage
		_ = json.Unmarshal(frame.Body, &body)
		var stream struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(body["stream"], &stream)
		if frame.Headers.ID != m.Reply["req_id"] || len(body["chatid"]) != 0 || !strings.Contains(stream.Content, "[alice]") {
			t.Error("group stream changed target or lost questioner", string(frame.Body))
		}
		session.respond(frame.Headers.ID, []byte(`{"errcode":0}`))
	}
	a := &WeCom{}
	defer a.bindings.add(s, session)()
	if result := a.CreateResponse(context.Background(), s, c, m, "inbox", application.ChannelResponsePreview{Answer: "answer"}, true); result.State != "sent" {
		t.Fatal(result)
	}
}

func TestWeComLongRunClosesTypingBeforeExpiryAndKeepsFinalFallback(t *testing.T) {
	s, c, m := wecomResponseFixture()
	m.Reply["received_at"] = strconv.FormatInt(time.Now().Add(-326*time.Second).UnixMilli(), 10)
	id, deadline, _ := wecomStreamIdentity(s, c, m)
	handle := id + "." + strconv.FormatInt(deadline, 10)
	socket := newFakeSocket()
	session := newSocketSession(context.Background(), socket)
	socket.onWrite = func(data []byte) {
		var frame wecomFrame
		_ = json.Unmarshal(data, &frame)
		var body struct {
			Stream struct {
				Finish  bool   `json:"finish"`
				Content string `json:"content"`
			} `json:"stream"`
		}
		_ = json.Unmarshal(frame.Body, &body)
		if !body.Stream.Finish || !strings.Contains(body.Stream.Content, "结果将另行发送") || strings.Contains(body.Stream.Content, "provisional-answer") {
			t.Error("long-running stream left typing or finalized provisional answer", string(frame.Body))
		}
		session.respond(frame.Headers.ID, []byte(`{"errcode":0}`))
	}
	a := &WeCom{}
	defer a.bindings.add(s, session)()
	result := a.UpdateResponse(context.Background(), s, c, m, handle, application.ChannelResponsePreview{Answer: "provisional-answer", Status: "正在调用工具"}, false)
	if result.State != "sent" || result.MessageID != handle+".closed" {
		t.Fatal("stream closure not checkpointable", result)
	}
	result = a.UpdateResponse(context.Background(), s, c, m, result.MessageID, application.ChannelResponsePreview{Answer: "final"}, true)
	if result.State != "expired" || result.Code != "provider_stream_expired" || len(socket.outgoing) != 1 {
		t.Fatal("closed stream reopened instead of allowing terminal fallback", result)
	}
}
