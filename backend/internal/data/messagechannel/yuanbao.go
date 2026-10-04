package messagechannel

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Yuanbao struct {
	*HTTP
	dial     channelSocketDialer
	bindings socketBindings
	sequence atomic.Uint32
}
type yuanbaoToken struct {
	ID       string `json:"bot_id"`
	Token    string `json:"token"`
	Source   string `json:"source"`
	Duration int64  `json:"duration"`
}

func (a *Yuanbao) token(ctx context.Context, c application.ChannelCredentials) (yuanbaoToken, error) {
	if c["app_key"] == "" || c["app_secret"] == "" {
		return yuanbaoToken{}, providerError("yuanbao_credentials_invalid")
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return yuanbaoToken{}, providerError("provider_request_invalid")
	}
	nonce := hex.EncodeToString(random)
	timestamp := time.Now().In(time.FixedZone("UTC+8", 8*3600)).Format("2006-01-02T15:04:05-07:00")
	mac := hmac.New(sha256.New, []byte(c["app_secret"]))
	mac.Write([]byte(nonce + timestamp + c["app_key"] + c["app_secret"]))
	result, status, _, err := a.requestHeaders(ctx, http.MethodPost, "https://bot.yuanbao.tencent.com/api/v5/robotLogic/sign-token", "", map[string]string{"app_key": c["app_key"], "nonce": nonce, "signature": hex.EncodeToString(mac.Sum(nil)), "timestamp": timestamp}, http.Header{"X-AppVersion": {"1.0.0"}, "X-OperationSystem": {"linux"}, "X-Instance-Id": {"16"}, "X-Bot-Version": {"AgentWorkspace"}})
	var token yuanbaoToken
	if err != nil || status != 200 || result["code"] == nil || rawNumber(result["code"]) != 0 || json.Unmarshal(result["data"], &token) != nil || token.ID == "" || token.Token == "" || token.Duration <= 0 || token.Duration > 86400*30 {
		return token, providerError("provider_identity_failed")
	}
	if token.Source == "" {
		token.Source = "bot"
	}
	return token, nil
}
func (a *Yuanbao) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	token, err := a.token(ctx, c)
	if err != nil {
		return application.ChannelIdentity{}, err
	}
	return application.ChannelIdentity{ID: token.ID, Name: token.ID, BindingID: token.ID}, nil
}
func (a *Yuanbao) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}
func normalizeYuanbao(s application.ChannelStored, e yuanbaoInbound) (domain.ChannelMessage, bool) {
	group := e.Command == "Group.CallbackAfterSendMsg"
	if (!group && e.Command != "C2C.CallbackAfterSendMsg") || e.ID == "" || e.From == s.Channel.AccountID || e.From == "" || (!group && e.To != s.Channel.AccountID) || (group && e.Group == "") {
		return domain.ChannelMessage{}, false
	}
	text := ""
	mentioned := false
	for _, element := range e.Body {
		if element.Type == "TIMTextElem" {
			text += element.Content.Text
		}
		if element.Type == "TIMCustomElem" {
			var mention struct {
				Type   int    `json:"elem_type"`
				UserID string `json:"user_id"`
			}
			if json.Unmarshal([]byte(element.Content.Data), &mention) == nil && mention.Type == 1002 && mention.UserID == s.Channel.AccountID {
				mentioned = true
			}
		}
	}
	chat := e.From
	if group {
		chat = e.Group
	}
	return domain.ChannelMessage{EventID: e.ID, MessageID: e.ID, SenderID: e.From, ChatID: chat, Group: group, Mentioned: mentioned, Text: text, OccurredAt: time.Unix(e.Timestamp, 0), Reply: map[string]string{}}, true
}
func (a *Yuanbao) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	token, err := a.token(ctx, c)
	if err != nil {
		return err
	}
	if token.ID != s.Channel.AccountID {
		return providerError("provider_identity_changed")
	}
	socket, err := a.dial(ctx, "wss://bot-wss.yuanbao.tencent.com/wss/connection", nil)
	if err != nil {
		return err
	}
	lifetime := time.Duration(token.Duration) * time.Second
	if lifetime > time.Minute {
		lifetime -= 30 * time.Second
	}
	child, cancel := context.WithTimeout(ctx, lifetime)
	defer cancel()
	defer socket.Close()
	stop := context.AfterFunc(child, func() { socket.Close() })
	defer stop()
	socket.SetReadLimit(65536)
	key := strings.ReplaceAll(uuid.NewString(), "-", "")
	auth := wireText(nil, 1, "ybBot")
	info := wireText(wireText(wireText(nil, 1, token.ID), 2, token.Source), 3, token.Token)
	auth = wireBytes(auth, 2, info)
	device := wireText(wireText(wireText(nil, 1, "1.0.0"), 2, "linux"), 10, "16")
	auth = wireBytes(auth, 3, device)
	auth = wireNumber(auth, 6, 1) // Reject an existing login; never force takeover.
	session := newSocketSession(child, socket)
	if err := session.write(websocket.BinaryMessage, encodeYuanbaoFrame(yuanbaoFrame{sequence: uint64(a.sequence.Add(1)), cmd: "auth-bind", module: "conn_access", id: key, body: auth})); err != nil {
		return err
	}
	_ = socket.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, data, err := socket.ReadMessage()
	if err != nil {
		return providerError("provider_identity_failed")
	}
	frame, err := decodeYuanbaoFrame(data)
	response, parseErr := readWire(frame.body)
	if err != nil || parseErr != nil || frame.kind != 1 || frame.cmd != "auth-bind" || frame.id != key || frame.status != 0 || response.numbers[1] != 0 {
		return providerError("provider_identity_failed")
	}
	remove := a.bindings.add(s, session)
	defer remove()
	go session.heartbeat(websocket.BinaryMessage, 5*time.Second, func() []byte {
		return encodeYuanbaoFrame(yuanbaoFrame{sequence: uint64(a.sequence.Add(1)), cmd: "ping", module: "conn_access", id: strings.ReplaceAll(uuid.NewString(), "-", "")})
	})
	for child.Err() == nil {
		_ = socket.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, data, err := socket.ReadMessage()
		if err != nil {
			return providerError("provider_connection_closed")
		}
		frame, err := decodeYuanbaoFrame(data)
		if err != nil {
			return err
		}
		if frame.kind == 1 {
			session.respond(frame.id, data)
			continue
		}
		if frame.kind != 2 {
			continue
		}
		if frame.cmd == "kickout" {
			return providerError("provider_connection_closed")
		}
		payload, err := yuanbaoPushPayload(frame)
		if err != nil {
			return err
		}
		inbound, err := decodeYuanbaoInbound(payload)
		if err != nil {
			return err
		}
		if message, ok := normalizeYuanbao(s, inbound); ok {
			commitCtx, cancel := context.WithTimeout(child, 10*time.Second)
			err := sink(commitCtx, message)
			cancel()
			if err != nil {
				return err
			}
		}
		if frame.ack {
			frame.kind, frame.ack, frame.body = 3, false, nil
			frame.sequence = uint64(a.sequence.Add(1))
			if err := session.write(websocket.BinaryMessage, encodeYuanbaoFrame(frame)); err != nil {
				return err
			}
		}
	}
	return child.Err()
}
func (a *Yuanbao) Send(ctx context.Context, s application.ChannelStored, _ application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	session := a.bindings.get(s)
	if session == nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_disconnected", RetryAfter: 30 * time.Second}
	}
	id := strings.ReplaceAll(key, "-", "")
	hash := sha256.Sum256([]byte(key))
	random := uint64(binary.BigEndian.Uint32(hash[:4]))
	element := wireText(nil, 1, "TIMTextElem")
	element = wireBytes(element, 2, wireText(nil, 1, text))
	command := "send_c2c_message"
	body := wireText(wireText(wireText(nil, 1, id), 2, m.SenderID), 3, s.Channel.AccountID)
	body = wireNumber(body, 4, random)
	body = wireBytes(body, 5, element)
	if m.Group {
		command = "send_group_message"
		body = wireText(wireText(wireText(wireText(nil, 1, id), 2, m.ChatID), 3, s.Channel.AccountID), 4, m.SenderID)
		body = wireText(body, 5, strconv.FormatUint(random, 10))
		body = wireBytes(body, 6, element)
		body = wireText(body, 7, m.MessageID)
	}
	data, err := session.request(ctx, id, websocket.BinaryMessage, encodeYuanbaoFrame(yuanbaoFrame{sequence: uint64(a.sequence.Add(1)), cmd: command, module: "yuanbao_openclaw_proxy", id: id, body: body}))
	if err != nil {
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	frame, err := decodeYuanbaoFrame(data)
	result, parseErr := readWire(frame.body)
	if err != nil || parseErr != nil || frame.cmd != command || frame.kind != 1 {
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	if frame.status == 50503 {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_rate_limited", RetryAfter: 30 * time.Second}
	}
	if frame.status != 0 || result.numbers[1] != 0 {
		return application.ChannelSendResult{State: "failed", Code: "provider_rejected"}
	}
	return application.ChannelSendResult{State: "sent", MessageID: id}
}
