package messagechannel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

const wechatProtocolVersion = "2.4.8"

type WeChat struct {
	*HTTP
	cursor application.ChannelReceiveCursor
}

func (a *WeChat) ValidateAudience(audience domain.ChannelAudience) error {
	return directAudience(audience)
}
func (a *WeChat) call(ctx context.Context, c application.ChannelCredentials, path string, body map[string]any) (map[string]json.RawMessage, int, time.Duration, error) {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return nil, 0, 0, providerError("provider_request_invalid")
	}
	body["base_info"] = map[string]string{"channel_version": wechatProtocolVersion, "bot_agent": "AgentWorkspace"}
	headers := http.Header{"AuthorizationType": {"ilink_bot_token"}, "X-WECHAT-UIN": {base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(binary.BigEndian.Uint32(random)), 10)))}, "iLink-App-Id": {"bot"}, "iLink-App-ClientVersion": {"132104"}}
	return a.requestHeaders(ctx, http.MethodPost, "https://ilinkai.weixin.qq.com"+path, "Bearer "+c["bot_token"], body, headers)
}
func (a *WeChat) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["bot_token"] == "" || c["account_id"] == "" || c["user_id"] == "" {
		return application.ChannelIdentity{}, providerError("wechat_credentials_invalid")
	}
	result, status, _, err := a.call(ctx, c, "/ilink/bot/getconfig", map[string]any{"ilink_user_id": c["user_id"]})
	if err != nil || status != 200 || result["ret"] == nil || rawNumber(result["ret"]) != 0 {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	// iLink has no whoami endpoint. The QR-issued bot ID is confirmed on inbound to_user_id.
	return application.ChannelIdentity{ID: c["account_id"], Name: c["account_id"], BindingID: c["account_id"]}, nil
}
func (a *WeChat) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}

type wechatMessage struct {
	ID      json.Number `json:"message_id"`
	From    string      `json:"from_user_id"`
	To      string      `json:"to_user_id"`
	GroupID string      `json:"group_id"`
	Type    int         `json:"message_type"`
	State   int         `json:"message_state"`
	Created int64       `json:"create_time_ms"`
	Updated int64       `json:"update_time_ms"`
	Deleted int64       `json:"delete_time_ms"`
	Token   string      `json:"context_token"`
	Items   []struct {
		Type int `json:"type"`
		Text struct {
			Text string `json:"text"`
		} `json:"text_item"`
	} `json:"item_list"`
}

func normalizeWeChat(s application.ChannelStored, e wechatMessage) (domain.ChannelMessage, bool) {
	if e.Type != 1 || e.State != 2 || e.From == s.Channel.AccountID || e.To != s.Channel.AccountID || e.GroupID != "" || e.Deleted != 0 || e.Token == "" || len(e.Token) > 4096 {
		return domain.ChannelMessage{}, false
	}
	text := ""
	for _, item := range e.Items {
		if item.Type == 1 {
			text += item.Text.Text
		}
	}
	if text == "" {
		return domain.ChannelMessage{}, false
	}
	return domain.ChannelMessage{EventID: e.ID.String(), MessageID: e.ID.String(), SenderID: e.From, ChatID: e.From, Text: text, OccurredAt: time.UnixMilli(e.Created), Reply: map[string]string{"context_token": e.Token}}, true
}
func (a *WeChat) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	return pollMessages(ctx, s, a.cursor, sink, func(ctx context.Context, cursor string) ([]domain.ChannelMessage, string, error) {
		result, status, _, err := a.call(ctx, c, "/ilink/bot/getupdates", map[string]any{"get_updates_buf": cursor})
		if err != nil || status != 200 || result["ret"] == nil || rawNumber(result["ret"]) != 0 || rawNumber(result["errcode"]) != 0 {
			return nil, "", providerError("provider_receive_failed")
		}
		var events []wechatMessage
		if len(result["msgs"]) > 0 && json.Unmarshal(result["msgs"], &events) != nil {
			return nil, "", providerError("provider_response_invalid")
		}
		var messages []domain.ChannelMessage
		for _, event := range events {
			if m, ok := normalizeWeChat(s, event); ok {
				messages = append(messages, m)
			}
		}
		return messages, rawString(result["get_updates_buf"]), nil
	})
}
func (a *WeChat) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	if m.Group || m.Reply["context_token"] == "" {
		return application.ChannelSendResult{State: "failed", Code: "reply_unavailable"}
	}
	body := map[string]any{"msg": map[string]any{"from_user_id": "", "to_user_id": m.SenderID, "client_id": key, "message_type": 2, "message_state": 2, "context_token": m.Reply["context_token"], "item_list": []any{map[string]any{"type": 1, "text_item": map[string]string{"text": text}}}}}
	result, status, retry, err := a.call(ctx, c, "/ilink/bot/sendmessage", body)
	if err != nil || status != 200 || result["ret"] == nil || rawNumber(result["ret"]) != 0 {
		if status == 200 {
			status = 400
		}
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: key}
}
