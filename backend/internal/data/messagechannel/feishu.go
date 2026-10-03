package messagechannel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type Feishu struct{ *HTTP }

func feishuBase(region string) string {
	if region == "lark" {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}
func (a *Feishu) token(ctx context.Context, c application.ChannelCredentials, region string) (string, error) {
	if c["app_id"] == "" || c["app_secret"] == "" || c["tenant_key"] == "" {
		return "", providerError("feishu_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, feishuBase(region)+"/open-apis/auth/v3/tenant_access_token/internal", "", map[string]string{"app_id": c["app_id"], "app_secret": c["app_secret"]})
	if err != nil || status != 200 || result["code"] == nil || rawNumber(result["code"]) != 0 || rawString(result["tenant_access_token"]) == "" {
		return "", providerError("provider_identity_failed")
	}
	return rawString(result["tenant_access_token"]), nil
}
func (a *Feishu) Identify(ctx context.Context, c application.ChannelCredentials, region string) (application.ChannelIdentity, error) {
	token, err := a.token(ctx, c, region)
	if err != nil {
		return application.ChannelIdentity{}, err
	}
	result, status, _, err := a.request(ctx, http.MethodGet, feishuBase(region)+"/open-apis/bot/v3/info", "Bearer "+token, nil)
	var bot struct {
		OpenID string `json:"open_id"`
		Name   string `json:"app_name"`
		Status int    `json:"activate_status"`
	}
	if err != nil || status != 200 || result["code"] == nil || rawNumber(result["code"]) != 0 || json.Unmarshal(result["bot"], &bot) != nil || bot.OpenID == "" || bot.Status != 2 {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: bot.OpenID, BindingID: feishuBase(region) + ":" + c["app_id"], Name: bot.Name, TenantID: c["tenant_key"]}, nil
}
func (a *Feishu) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	identity, err := a.Identify(ctx, c, s.Channel.Region)
	if err != nil {
		return err
	}
	if identity.ID != s.Channel.AccountID || identity.TenantID != s.Channel.TenantID {
		return providerError("provider_identity_changed")
	}
	return nil
}
func (a *Feishu) Callback(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, h http.Header, b []byte) (application.ChannelCallback, error) {
	return noCallback(ctx, s, c, h, b)
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func normalizeFeishu(s application.ChannelStored, e *larkim.P2MessageReceiveV1) (domain.ChannelMessage, bool) {
	if e == nil || e.Event == nil || e.Event.Message == nil || e.Event.Sender == nil || e.Event.Sender.SenderId == nil || e.EventV2Base == nil || e.EventV2Base.Header == nil {
		return domain.ChannelMessage{}, false
	}
	sender, msg := e.Event.Sender, e.Event.Message
	if value(sender.SenderType) != "user" || value(sender.TenantKey) != s.Channel.TenantID || e.EventV2Base.Header.TenantKey != s.Channel.TenantID || e.EventV2Base.Header.AppID == "" || value(msg.MessageType) != "text" || value(sender.SenderId.OpenId) == s.Channel.AccountID || (value(msg.ChatType) != "group" && value(msg.ChatType) != "p2p") {
		return domain.ChannelMessage{}, false
	}
	var content struct{ Text string }
	if json.Unmarshal([]byte(value(msg.Content)), &content) != nil {
		return domain.ChannelMessage{}, false
	}
	mentioned := false
	for _, mention := range msg.Mentions {
		if mention != nil && mention.Id != nil && value(mention.Id.OpenId) == s.Channel.AccountID {
			mentioned = true
			content.Text = strings.ReplaceAll(content.Text, value(mention.Key), "")
		}
	}
	ms, err := strconv.ParseInt(value(msg.CreateTime), 10, 64)
	if err != nil {
		return domain.ChannelMessage{}, false
	}
	return domain.ChannelMessage{EventID: e.EventV2Base.Header.EventID, MessageID: value(msg.MessageId), SenderID: value(sender.SenderId.OpenId), ChatID: value(msg.ChatId), ThreadID: value(msg.ThreadId), Group: value(msg.ChatType) == "group", Mentioned: mentioned, Text: strings.TrimSpace(content.Text), OccurredAt: time.UnixMilli(ms), Reply: map[string]string{}}, true
}

type silentLogger struct{}

func (silentLogger) Debug(context.Context, ...interface{}) {}
func (silentLogger) Info(context.Context, ...interface{})  {}
func (silentLogger) Warn(context.Context, ...interface{})  {}
func (silentLogger) Error(context.Context, ...interface{}) {}
func (a *Feishu) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, receive func(context.Context, domain.ChannelMessage) error) error {
	handler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(func(callbackCtx context.Context, e *larkim.P2MessageReceiveV1) error {
		if e != nil && e.EventV2Base != nil && e.EventV2Base.Header != nil && e.EventV2Base.Header.AppID != c["app_id"] {
			return nil
		}
		if m, ok := normalizeFeishu(s, e); ok {
			return receive(callbackCtx, m)
		}
		return nil
	})
	handler.Config.Logger = silentLogger{}
	ws := larkws.NewClient(c["app_id"], c["app_secret"], larkws.WithDomain(feishuBase(s.Channel.Region)), larkws.WithLogger(silentLogger{}), larkws.WithEventHandler(handler))
	defer ws.Close()
	if err := ws.Start(ctx); err != nil && ctx.Err() == nil {
		return providerError("provider_connection_failed")
	}
	return ctx.Err()
}
func (a *Feishu) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	token, err := a.token(ctx, c, s.Channel.Region)
	if err != nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_authentication_failed", RetryAfter: time.Minute}
	}
	content, _ := json.Marshal(map[string]string{"text": text})
	body := map[string]any{"msg_type": "text", "content": string(content), "uuid": key}
	if m.ThreadID != "" {
		body["reply_in_thread"] = true
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, feishuBase(s.Channel.Region)+"/open-apis/im/v1/messages/"+url.PathEscape(m.MessageID)+"/reply", "Bearer "+token, body)
	if err != nil || status != 200 || result["code"] == nil || rawNumber(result["code"]) != 0 {
		if status == 200 {
			status = 400
		}
		return failedSend(status, retry, err)
	}
	var data struct {
		ID string `json:"message_id"`
	}
	if json.Unmarshal(result["data"], &data) != nil || data.ID == "" {
		return failedSend(0, 0, nil)
	}
	return application.ChannelSendResult{State: "sent", MessageID: data.ID}
}
