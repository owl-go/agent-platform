package messagechannel

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

type DingTalk struct{ *HTTP }

func (a *DingTalk) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["client_id"] == "" || c["client_secret"] == "" || c["corp_id"] == "" {
		return application.ChannelIdentity{}, providerError("dingtalk_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, "https://api.dingtalk.com/v1.0/oauth2/accessToken", "", map[string]string{"appKey": c["client_id"], "appSecret": c["client_secret"]})
	if err != nil || status != 200 || rawString(result["accessToken"]) == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: c["client_id"], BindingID: c["client_id"], Name: c["client_id"], TenantID: c["corp_id"]}, nil
}
func (a *DingTalk) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	identity, err := a.Identify(ctx, c, "")
	if err != nil {
		return err
	}
	if identity.ID != s.Channel.AccountID || identity.TenantID != s.Channel.TenantID {
		return providerError("provider_identity_changed")
	}
	return nil
}
func (a *DingTalk) Callback(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, h http.Header, b []byte) (application.ChannelCallback, error) {
	return noCallback(ctx, s, c, h, b)
}
func validDingTalkReply(target string) bool {
	u, err := url.Parse(target)
	return err == nil && u.Scheme == "https" && u.Host == "oapi.dingtalk.com" && u.User == nil && u.Path == "/robot/sendBySession" && u.Fragment == "" && len(target) <= 4096
}
func normalizeDingTalk(s application.ChannelStored, e *chatbot.BotCallbackDataModel) (domain.ChannelMessage, bool) {
	if e == nil || e.Msgtype != "text" || e.ChatbotCorpId != s.Channel.TenantID || e.SenderCorpId != s.Channel.TenantID || e.ChatbotUserId == e.SenderId || !validDingTalkReply(e.SessionWebhook) || (e.ConversationType != "1" && e.ConversationType != "2") {
		return domain.ChannelMessage{}, false
	}
	// Staff IDs are scoped to this authenticated enterprise, never display names.
	return domain.ChannelMessage{EventID: e.MsgId, MessageID: e.MsgId, SenderID: e.SenderStaffId, ChatID: e.ConversationId, Group: e.ConversationType == "2", Mentioned: e.IsInAtList, Text: strings.TrimSpace(e.Text.Content), OccurredAt: time.UnixMilli(e.CreateAt), Reply: map[string]string{"session_webhook": e.SessionWebhook, "expires_at": strconv.FormatInt(e.SessionWebhookExpiredTime, 10)}}, true
}

// The SDK's StreamClient reconnects with context.Background, even after Close.
// Keep the official frame types, but own the connection and its cancellation.
func (a *DingTalk) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, receive func(context.Context, domain.ChannelMessage) error) error {
	result, status, _, err := a.request(ctx, http.MethodPost, "https://api.dingtalk.com/v1.0/gateway/connections/open", "", map[string]any{
		"clientId": c["client_id"], "clientSecret": c["client_secret"], "ua": "agent-workspace/1.0",
		"subscriptions": []map[string]string{{"type": "CALLBACK", "topic": payload.BotMessageCallbackTopic}},
	})
	if err != nil || status != 200 {
		return providerError("provider_connection_failed")
	}
	target, err := dingTalkStreamURL(rawString(result["endpoint"]), rawString(result["ticket"]))
	if err != nil {
		return err
	}
	dialer := websocket.Dialer{NetDialContext: dialPublicProvider, HandshakeTimeout: 10 * time.Second}
	conn, response, err := dialer.DialContext(ctx, target, nil)
	if response != nil && err != nil {
		response.Body.Close()
	}
	if err != nil {
		return providerError("provider_connection_failed")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetReadLimit(64 * 1024)
	for {
		conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return providerError("provider_connection_failed")
		}
		frame, err := payload.DecodeDataFrame(data)
		if err != nil || frame.Headers == nil || frame.GetMessageId() == "" {
			return providerError("provider_payload_invalid")
		}
		ack, disconnect, err := dingTalkFrame(ctx, s, frame, receive)
		ack.SetHeader(payload.DataFrameHeaderKMessageId, frame.GetMessageId())
		ack.SetHeader(payload.DataFrameHeaderKContentType, payload.DataFrameContentTypeKJson)
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if writeErr := conn.WriteJSON(ack); writeErr != nil {
			return providerError("provider_connection_failed")
		}
		if err != nil {
			return err
		}
		if disconnect {
			return providerError("provider_connection_closed")
		}
	}
}
func dingTalkStreamURL(endpoint, ticket string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "wss" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Path != "/connect" ||
		(u.Hostname() != "wss-open-connection.dingtalk.com" && u.Hostname() != "wss-open-connection-union.dingtalk.com") ||
		(u.Port() != "" && u.Port() != "443") || ticket == "" || len(ticket) > 4096 {
		return "", providerError("provider_endpoint_invalid")
	}
	query := url.Values{"ticket": []string{ticket}}
	u.RawQuery = query.Encode()
	return u.String(), nil
}
func dingTalkFrame(ctx context.Context, s application.ChannelStored, frame *payload.DataFrame, receive func(context.Context, domain.ChannelMessage) error) (*payload.DataFrameResponse, bool, error) {
	ack := payload.NewSuccessDataFrameResponse()
	if frame.Type == "SYSTEM" {
		switch frame.GetTopic() {
		case "ping":
			ack = payload.NewDataFrameAckPong(frame.GetMessageId())
			ack.Data = frame.Data
		case "disconnect":
			return ack, true, nil
		default:
			ack = payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound)
		}
		return ack, false, nil
	}
	if frame.Type != "CALLBACK" || frame.GetTopic() != payload.BotMessageCallbackTopic {
		return payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound), false, nil
	}
	var event chatbot.BotCallbackDataModel
	if json.Unmarshal([]byte(frame.Data), &event) != nil {
		return payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKInternalError), false, providerError("provider_payload_invalid")
	}
	if message, ok := normalizeDingTalk(s, &event); ok {
		commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := receive(commitCtx, message); err != nil {
			return payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKInternalError), false, providerError("channel_inbox_unavailable")
		}
	}
	return ack, false, nil
}
func (a *DingTalk) Send(ctx context.Context, _ application.ChannelStored, _ application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	target := m.Reply["session_webhook"]
	expires, err := strconv.ParseInt(m.Reply["expires_at"], 10, 64)
	if err != nil || !time.Now().Before(time.UnixMilli(expires)) {
		return application.ChannelSendResult{State: "expired", Code: "provider_reply_expired"}
	}
	if !validDingTalkReply(target) {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, target, "", map[string]any{"msgtype": "text", "text": map[string]string{"content": text}, "at": map[string]any{"atUserIds": []string{m.SenderID}, "isAtAll": false}})
	if err != nil || status != 200 || result["errcode"] == nil || rawNumber(result["errcode"]) != 0 {
		if status == 200 {
			status = 400
		}
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent"}
}
