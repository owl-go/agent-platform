package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type WeCom struct {
	dial     channelSocketDialer
	bindings socketBindings
}
type wecomFrame struct {
	Cmd     string `json:"cmd,omitempty"`
	Headers struct {
		ID string `json:"req_id"`
	} `json:"headers"`
	Body json.RawMessage `json:"body,omitempty"`
	Code *int            `json:"errcode,omitempty"`
}

func wecomRequest(cmd, key string, body any) []byte {
	data, _ := json.Marshal(map[string]any{"cmd": cmd, "headers": map[string]string{"req_id": key}, "body": body})
	return data
}
func wecomAccountFailure(code string, providerCode int) error {
	return &application.ChannelAccountFailure{Code: code, ProviderCode: providerCode}
}
func wecomConnectionFailure(ctx context.Context, err error) error {
	var networkError net.Error
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return wecomAccountFailure("wecom_authentication_timeout", 0)
	}
	return wecomAccountFailure("wecom_connection_failed", 0)
}
func (a *WeCom) open(ctx context.Context, c application.ChannelCredentials) (channelSocket, error) {
	if c["bot_id"] == "" || c["bot_secret"] == "" {
		return nil, wecomAccountFailure("wecom_credentials_invalid", 0)
	}
	socket, err := a.dial(ctx, "wss://openws.work.weixin.qq.com", http.Header{})
	if err != nil {
		return nil, wecomConnectionFailure(ctx, err)
	}
	authenticated := false
	defer func() {
		if !authenticated {
			socket.Close()
		}
	}()
	socket.SetReadLimit(65536)
	stop := context.AfterFunc(ctx, func() { socket.Close() })
	defer stop()
	deadline := time.Now().Add(15 * time.Second)
	if parent, ok := ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	if err := socket.SetReadDeadline(deadline); err != nil {
		return nil, wecomConnectionFailure(ctx, err)
	}
	if err := socket.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, wecomConnectionFailure(ctx, err)
	}
	key := uuid.NewString()
	if err := socket.WriteMessage(websocket.TextMessage, wecomRequest("aibot_subscribe", key, map[string]string{"bot_id": c["bot_id"], "secret": c["bot_secret"]})); err != nil {
		return nil, wecomConnectionFailure(ctx, err)
	}
	// Callbacks and unrelated receipts are not authentication results. Bound both
	// time and frame count while waiting for our own subscription receipt.
	for attempt := 0; attempt < 32; attempt++ {
		_, data, err := socket.ReadMessage()
		if err != nil {
			return nil, wecomConnectionFailure(ctx, err)
		}
		var frame wecomFrame
		if json.Unmarshal(data, &frame) != nil {
			return nil, wecomAccountFailure("wecom_authentication_invalid", 0)
		}
		if frame.Cmd != "" || frame.Headers.ID != key {
			continue
		}
		if frame.Code == nil {
			return nil, wecomAccountFailure("wecom_authentication_invalid", 0)
		}
		if *frame.Code != 0 {
			return nil, wecomAccountFailure("wecom_authentication_rejected", *frame.Code)
		}
		authenticated = true
		return socket, nil
	}
	return nil, wecomAccountFailure("wecom_authentication_invalid", 0)
}
func (a *WeCom) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	socket, err := a.open(ctx, c)
	if err != nil {
		return application.ChannelIdentity{}, err
	}
	defer socket.Close()
	return application.ChannelIdentity{ID: c["bot_id"], Name: c["bot_id"], BindingID: c["bot_id"]}, nil
}
func (a *WeCom) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}
func normalizeWeCom(s application.ChannelStored, frame wecomFrame) (domain.ChannelMessage, bool) {
	if frame.Cmd != "aibot_msg_callback" {
		return domain.ChannelMessage{}, false
	}
	var event struct {
		ID        string `json:"msgid"`
		BotID     string `json:"aibotid"`
		ChatID    string `json:"chatid"`
		ChatType  string `json:"chattype"`
		Type      string `json:"msgtype"`
		Timestamp int64  `json:"create_time"`
		From      struct {
			ID string `json:"userid"`
		} `json:"from"`
		Text struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	if json.Unmarshal(frame.Body, &event) != nil || event.BotID != s.Channel.AccountID || event.Type != "text" || event.From.ID == event.BotID || (event.ChatType != "single" && event.ChatType != "group") {
		return domain.ChannelMessage{}, false
	}
	group := event.ChatType == "group"
	chat := event.ChatID
	if !group {
		chat = event.From.ID
	}
	timestamp := time.Unix(event.Timestamp, 0)
	if event.Timestamp == 0 {
		timestamp = time.Now().UTC()
	}
	// The authenticated smart-bot group callback is delivered only on bot mentions.
	return domain.ChannelMessage{EventID: event.ID, MessageID: event.ID, SenderID: event.From.ID, ChatID: chat, Group: group, Mentioned: group, Text: event.Text.Content, OccurredAt: timestamp, Reply: map[string]string{}}, true
}
func (a *WeCom) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	socket, err := a.open(ctx, c)
	if err != nil {
		return err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	defer socket.Close()
	stop := context.AfterFunc(child, func() { socket.Close() })
	defer stop()
	session := newSocketSession(child, socket)
	remove := a.bindings.add(s, session)
	defer remove()
	go session.heartbeat(websocket.TextMessage, 25*time.Second, func() []byte { return wecomRequest("ping", uuid.NewString(), nil) })
	for child.Err() == nil {
		_ = socket.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, data, err := socket.ReadMessage()
		if err != nil {
			return providerError("provider_connection_closed")
		}
		var frame wecomFrame
		if json.Unmarshal(data, &frame) != nil {
			return providerError("provider_payload_invalid")
		}
		if frame.Cmd == "aibot_event_callback" {
			var event struct {
				Event struct {
					Type string `json:"eventtype"`
				} `json:"event"`
			}
			_ = json.Unmarshal(frame.Body, &event)
			if event.Event.Type == "disconnected_event" {
				return providerError("provider_connection_closed")
			}
			continue
		}
		if frame.Cmd == "aibot_msg_callback" {
			if m, ok := normalizeWeCom(s, frame); ok {
				commitCtx, cancel := context.WithTimeout(child, 10*time.Second)
				err := sink(commitCtx, m)
				cancel()
				if err != nil {
					return err
				}
			}
			// Smart-bot callbacks do not expose a separate durable ACK. A saved Inbox is
			// the platform recovery boundary; no receipt or successful answer is fabricated.
			continue
		}
		session.respond(frame.Headers.ID, data)
	}
	return child.Err()
}
func (a *WeCom) Send(ctx context.Context, s application.ChannelStored, _ application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	session := a.bindings.get(s)
	if session == nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_disconnected", RetryAfter: 30 * time.Second}
	}
	if m.Group {
		text = "[" + m.SenderID + "] " + text
	}
	data, err := session.request(ctx, key, websocket.TextMessage, wecomRequest("aibot_send_msg", key, map[string]any{"chatid": m.ChatID, "msgtype": "markdown", "markdown": map[string]string{"content": text}}))
	if err != nil {
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	var frame wecomFrame
	if json.Unmarshal(data, &frame) != nil || frame.Code == nil {
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	if *frame.Code == 45009 {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_rate_limited", RetryAfter: time.Minute}
	}
	if *frame.Code != 0 {
		return application.ChannelSendResult{State: "failed", Code: "provider_rejected"}
	}
	return application.ChannelSendResult{State: "sent", MessageID: key}
}
