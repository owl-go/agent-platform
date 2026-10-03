package messagechannel

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type Telegram struct{ *HTTP }

func telegramURL(c application.ChannelCredentials, method string) string {
	return "https://api.telegram.org/bot" + c["bot_token"] + "/" + method
}
func (a *Telegram) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["bot_token"] == "" || strings.ContainsAny(c["bot_token"], "/?#% ") {
		return application.ChannelIdentity{}, providerError("bot_token_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, telegramURL(c, "getMe"), "", map[string]any{})
	if err != nil || status != 200 || !rawBool(result["ok"]) {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	var bot struct {
		ID       int64
		IsBot    bool `json:"is_bot"`
		Username string
	}
	if json.Unmarshal(result["result"], &bot) != nil || bot.ID <= 0 || !bot.IsBot || bot.Username == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_invalid")
	}
	return application.ChannelIdentity{ID: strconv.FormatInt(bot.ID, 10), BindingID: strconv.FormatInt(bot.ID, 10), Name: bot.Username}, nil
}
func (a *Telegram) Configure(ctx context.Context, stored application.ChannelStored, c application.ChannelCredentials, callback string) error {
	identity, err := a.Identify(ctx, c, "")
	if err != nil || identity.ID != stored.Channel.AccountID {
		return providerError("provider_identity_changed")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, telegramURL(c, "getWebhookInfo"), "", map[string]any{})
	if err != nil || status != 200 || !rawBool(result["ok"]) {
		return providerError("provider_configuration_failed")
	}
	var info struct{ URL string }
	if json.Unmarshal(result["result"], &info) != nil || info.URL != "" && info.URL != callback {
		return providerError("bot_webhook_already_bound")
	}
	result, status, _, err = a.request(ctx, http.MethodPost, telegramURL(c, "setWebhook"), "", map[string]any{"url": callback, "secret_token": c["callback_secret"], "allowed_updates": []string{"message"}, "drop_pending_updates": false})
	if err != nil || status != 200 || !rawBool(result["ok"]) {
		return providerError("provider_configuration_failed")
	}
	return nil
}

type telegramUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		ID       int64 `json:"message_id"`
		ThreadID int64 `json:"message_thread_id"`
		Date     int64
		Text     string
		From     *struct {
			ID  int64
			Bot bool `json:"is_bot"`
		}
		Chat struct {
			ID   int64
			Type string
		}
		Entities []struct {
			Type           string
			Offset, Length int
		}
	} `json:"message"`
}

func (a *Telegram) Callback(_ context.Context, stored application.ChannelStored, c application.ChannelCredentials, headers http.Header, body []byte) (application.ChannelCallback, error) {
	if c["callback_secret"] == "" || subtle.ConstantTimeCompare([]byte(headers.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(c["callback_secret"])) != 1 {
		return application.ChannelCallback{}, providerError("callback_authentication_failed")
	}
	var update telegramUpdate
	if json.Unmarshal(body, &update) != nil || update.UpdateID < 0 {
		return application.ChannelCallback{}, providerError("callback_payload_invalid")
	}
	result := application.ChannelCallback{Response: map[string]bool{"ok": true}}
	m := update.Message
	if m == nil || m.From == nil || m.From.Bot || m.Text == "" || (m.Chat.Type != "private" && m.Chat.Type != "group" && m.Chat.Type != "supergroup") {
		return result, nil
	}
	text := m.Text
	mentioned := false
	units := utf16.Encode([]rune(text))
	target := "@" + stored.Channel.AccountName
	for _, entity := range m.Entities {
		if entity.Type == "mention" && entity.Offset >= 0 && entity.Length > 0 && entity.Offset+entity.Length <= len(units) {
			value := string(utf16.Decode(units[entity.Offset : entity.Offset+entity.Length]))
			if strings.EqualFold(value, target) {
				mentioned = true
				text = strings.ReplaceAll(text, value, "")
			}
		}
	}
	message := domain.ChannelMessage{EventID: strconv.FormatInt(update.UpdateID, 10), MessageID: strconv.FormatInt(m.ID, 10), SenderID: strconv.FormatInt(m.From.ID, 10), ChatID: strconv.FormatInt(m.Chat.ID, 10), Group: m.Chat.Type != "private", Mentioned: mentioned, Text: strings.TrimSpace(text), OccurredAt: time.Unix(m.Date, 0), Reply: map[string]string{}}
	if m.ThreadID != 0 {
		message.ThreadID = strconv.FormatInt(m.ThreadID, 10)
	}
	result.Messages = []domain.ChannelMessage{message}
	return result, nil
}
func (a *Telegram) Send(ctx context.Context, _ application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	body := map[string]any{"chat_id": m.ChatID, "text": text, "reply_parameters": map[string]any{"message_id": rawTelegramID(m.MessageID), "allow_sending_without_reply": false}, "link_preview_options": map[string]any{"is_disabled": true}}
	if m.ThreadID != "" {
		body["message_thread_id"] = rawTelegramID(m.ThreadID)
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, telegramURL(c, "sendMessage"), "", body)
	if err != nil || status != 200 || !rawBool(result["ok"]) {
		var parameters struct {
			RetryAfter int `json:"retry_after"`
		}
		_ = json.Unmarshal(result["parameters"], &parameters)
		if parameters.RetryAfter > 0 {
			status = 429
			retry = time.Duration(parameters.RetryAfter) * time.Second
		}
		return failedSend(status, retry, err)
	}
	var message struct {
		ID int64 `json:"message_id"`
	}
	if json.Unmarshal(result["result"], &message) != nil || message.ID <= 0 {
		return failedSend(0, 0, nil)
	}
	return application.ChannelSendResult{State: "sent", MessageID: strconv.FormatInt(message.ID, 10)}
}

func rawTelegramID(id string) int64 { value, _ := strconv.ParseInt(id, 10, 64); return value }
