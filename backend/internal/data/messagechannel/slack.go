package messagechannel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type Slack struct{ *HTTP }

func (a *Slack) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["bot_token"] == "" || len(c["signing_secret"]) < 16 {
		return application.ChannelIdentity{}, providerError("slack_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, "https://slack.com/api/auth.test", "Bearer "+c["bot_token"], map[string]any{})
	if err != nil || status != 200 || !rawBool(result["ok"]) || rawString(result["user_id"]) == "" || rawString(result["team_id"]) == "" || rawString(result["bot_id"]) == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: rawString(result["user_id"]), BindingID: rawString(result["team_id"]) + ":" + rawString(result["user_id"]), Name: rawString(result["user"]), TenantID: rawString(result["team_id"])}, nil
}
func (a *Slack) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	identity, err := a.Identify(ctx, c, "")
	if err != nil {
		return err
	}
	if identity.ID != s.Channel.AccountID || identity.TenantID != s.Channel.TenantID {
		return providerError("provider_identity_changed")
	}
	return nil
}
func (a *Slack) Callback(_ context.Context, s application.ChannelStored, c application.ChannelCredentials, h http.Header, body []byte) (application.ChannelCallback, error) {
	timestamp := h.Get("X-Slack-Request-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || seconds < time.Now().Add(-5*time.Minute).Unix() || seconds > time.Now().Add(5*time.Minute).Unix() {
		return application.ChannelCallback{}, providerError("callback_authentication_failed")
	}
	mac := hmac.New(sha256.New, []byte(c["signing_secret"]))
	_, _ = mac.Write([]byte("v0:" + timestamp + ":"))
	_, _ = mac.Write(body)
	if c["signing_secret"] == "" || !hmac.Equal([]byte(h.Get("X-Slack-Signature")), []byte("v0="+hex.EncodeToString(mac.Sum(nil)))) {
		return application.ChannelCallback{}, providerError("callback_authentication_failed")
	}
	var payload struct {
		Type, Challenge string
		TeamID          string `json:"team_id"`
		EventID         string `json:"event_id"`
		Event           struct {
			Type, Subtype, User, Text, Channel, TS string
			ThreadTS                               string `json:"thread_ts"`
			ChannelType                            string `json:"channel_type"`
			BotID                                  string `json:"bot_id"`
		}
	}
	if json.Unmarshal(body, &payload) != nil {
		return application.ChannelCallback{}, providerError("callback_payload_invalid")
	}
	if payload.Type == "url_verification" {
		if len(payload.Challenge) > 256 {
			return application.ChannelCallback{}, providerError("callback_payload_invalid")
		}
		return application.ChannelCallback{Response: map[string]string{"challenge": payload.Challenge}}, nil
	}
	result := application.ChannelCallback{Response: map[string]bool{"ok": true}}
	e := payload.Event
	if payload.Type != "event_callback" || payload.TeamID != s.Channel.TenantID || e.BotID != "" || e.Subtype != "" || e.User == s.Channel.AccountID || (e.Type != "app_mention" && !(e.Type == "message" && e.ChannelType == "im")) {
		return result, nil
	}
	ts, err := strconv.ParseFloat(e.TS, 64)
	if err != nil {
		return result, nil
	}
	mention := "<@" + s.Channel.AccountID + ">"
	mentioned := strings.Contains(e.Text, mention)
	m := domain.ChannelMessage{EventID: payload.EventID, MessageID: e.TS, SenderID: e.User, ChatID: e.Channel, ThreadID: e.ThreadTS, Group: e.ChannelType != "im", Mentioned: mentioned, Text: strings.TrimSpace(strings.ReplaceAll(e.Text, mention, "")), OccurredAt: time.UnixMilli(int64(ts * 1000)), Reply: map[string]string{}}
	if m.Group && m.ThreadID == "" {
		m.ThreadID = e.TS
	}
	result.Messages = []domain.ChannelMessage{m}
	return result, nil
}
func (a *Slack) Send(ctx context.Context, _ application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	body := map[string]any{"channel": m.ChatID, "text": text, "client_msg_id": key, "unfurl_links": false, "unfurl_media": false, "parse": "none", "mrkdwn": false}
	if m.ThreadID != "" {
		body["thread_ts"] = m.ThreadID
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, "https://slack.com/api/chat.postMessage", "Bearer "+c["bot_token"], body)
	if err != nil || status != 200 || !rawBool(result["ok"]) {
		if status == 200 {
			status = 400
			if rawString(result["error"]) == "ratelimited" {
				status = 429
			}
		}
		return failedSend(status, retry, err)
	}
	id := rawString(result["ts"])
	if id == "" {
		return failedSend(0, 0, nil)
	}
	return application.ChannelSendResult{State: "sent", MessageID: id}
}
