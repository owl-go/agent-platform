package messagechannel

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type QQBot struct {
	*HTTP
	dial channelSocketDialer
}

func qqKey(secret string) ed25519.PrivateKey {
	if secret == "" {
		return nil
	}
	seed := []byte(secret)
	for len(seed) < ed25519.SeedSize {
		seed = append(seed, seed...)
	}
	return ed25519.NewKeyFromSeed(seed[:ed25519.SeedSize])
}
func (a *QQBot) token(ctx context.Context, c application.ChannelCredentials) (string, error) {
	if c["app_id"] == "" || c["app_secret"] == "" {
		return "", providerError("qqbot_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodPost, "https://bots.qq.com/app/getAppAccessToken", "", map[string]string{"appId": c["app_id"], "clientSecret": c["app_secret"]})
	if err != nil || status != 200 || rawString(result["access_token"]) == "" {
		return "", providerError("provider_identity_failed")
	}
	return rawString(result["access_token"]), nil
}
func (a *QQBot) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	token, err := a.token(ctx, c)
	if err != nil {
		return application.ChannelIdentity{}, err
	}
	result, status, _, err := a.request(ctx, http.MethodGet, "https://api.sgroup.qq.com/users/@me", "QQBot "+token, nil)
	if err != nil || status != 200 || rawString(result["id"]) == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: rawString(result["id"]), BindingID: c["app_id"], Name: rawString(result["username"])}, nil
}
func (a *QQBot) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}
func (a *QQBot) Callback(_ context.Context, s application.ChannelStored, c application.ChannelCredentials, headers http.Header, body []byte) (application.ChannelCallback, error) {
	stamp := headers.Get("X-Signature-Timestamp")
	timestamp, err := strconv.ParseInt(stamp, 10, 64)
	key := qqKey(c["app_secret"])
	signature, sigErr := hex.DecodeString(headers.Get("X-Signature-Ed25519"))
	if err != nil || sigErr != nil || key == nil || time.Since(time.Unix(timestamp, 0)) > 5*time.Minute || time.Until(time.Unix(timestamp, 0)) > 5*time.Minute || !ed25519.Verify(key.Public().(ed25519.PublicKey), append([]byte(stamp), body...), signature) {
		return application.ChannelCallback{}, providerError("callback_authentication_failed")
	}
	var payload struct {
		Op   int             `json:"op"`
		Type string          `json:"t"`
		ID   string          `json:"id"`
		Data json.RawMessage `json:"d"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return application.ChannelCallback{}, providerError("provider_payload_invalid")
	}
	result := application.ChannelCallback{Response: map[string]int{"op": 12, "d": 0}}
	if payload.Op == 13 {
		var challenge struct {
			Token     string `json:"plain_token"`
			Timestamp string `json:"event_ts"`
		}
		if json.Unmarshal(payload.Data, &challenge) != nil || challenge.Token == "" || len(challenge.Token) > 4096 || challenge.Timestamp == "" {
			return result, providerError("provider_payload_invalid")
		}
		signed := ed25519.Sign(key, append([]byte(challenge.Timestamp), []byte(challenge.Token)...))
		result.Response = map[string]string{"plain_token": challenge.Token, "signature": hex.EncodeToString(signed)}
		return result, nil
	}
	if payload.Op == 1 {
		var sequence uint32
		if json.Unmarshal(payload.Data, &sequence) != nil {
			return result, providerError("provider_payload_invalid")
		}
		result.Response = map[string]any{"op": 11, "d": sequence}
		return result, nil
	}
	if payload.Op != 0 {
		return result, nil
	}
	result.Messages, err = normalizeQQ(s, payload.Type, payload.Data)
	return result, err
}

func normalizeQQ(s application.ChannelStored, eventType string, data json.RawMessage) ([]domain.ChannelMessage, error) {
	if eventType != "GROUP_AT_MESSAGE_CREATE" && eventType != "C2C_MESSAGE_CREATE" {
		return nil, nil
	}
	var event struct {
		ID        string    `json:"id"`
		GroupID   string    `json:"group_openid"`
		Content   string    `json:"content"`
		Timestamp time.Time `json:"timestamp"`
		Author    struct {
			UserID   string `json:"user_openid"`
			MemberID string `json:"member_openid"`
			Bot      bool   `json:"bot"`
		} `json:"author"`
	}
	if json.Unmarshal(data, &event) != nil {
		return nil, providerError("provider_payload_invalid")
	}
	group := eventType == "GROUP_AT_MESSAGE_CREATE"
	sender, chat := event.Author.UserID, event.Author.UserID
	if group {
		sender, chat = event.Author.MemberID, event.GroupID
	}
	if event.Author.Bot || sender == s.Channel.AccountID {
		return nil, nil
	}
	text := strings.TrimSpace(event.Content)
	if group {
		text = strings.TrimSpace(strings.ReplaceAll(text, "<@!"+s.Channel.AccountID+">", ""))
	}
	return []domain.ChannelMessage{{EventID: event.ID, MessageID: event.ID, SenderID: sender, ChatID: chat, Group: group, Mentioned: group, Text: text, OccurredAt: event.Timestamp, Reply: map[string]string{"expires_at": strconv.FormatInt(event.Timestamp.Add(5*time.Minute).Unix(), 10)}}}, nil
}
func (a *QQBot) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	expires, err := strconv.ParseInt(m.Reply["expires_at"], 10, 64)
	if err != nil || !time.Now().Before(time.Unix(expires, 0)) {
		return application.ChannelSendResult{State: "expired", Code: "reply_window_expired"}
	}
	token, err := a.token(ctx, c)
	if err != nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_authentication_failed", RetryAfter: time.Minute}
	}
	kind, target := "users", m.SenderID
	if m.Group {
		kind, target = "groups", m.ChatID
		text = "[" + m.SenderID + "] " + text
	}
	chunk, _ := strconv.Atoi(m.Reply["delivery_chunk"])
	if chunk < 1 {
		chunk = 1
	}
	seq := chunk
	if kind := m.Reply["delivery_kind"]; kind == "answer" || kind == "" {
		if chunk > 4 {
			return application.ChannelSendResult{State: "failed", Code: "reply_budget_exhausted"}
		}
		seq++
	} else if chunk > 1 {
		return application.ChannelSendResult{State: "failed", Code: "reply_budget_exhausted"}
	} else if kind == "status" {
		seq = 2 // Terminal rejection/failure never has an answer, so shares its slot.
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, "https://api.sgroup.qq.com/v2/"+kind+"/"+url.PathEscape(target)+"/messages", "QQBot "+token, map[string]any{"content": text, "msg_type": 0, "msg_id": m.MessageID, "msg_seq": seq})
	if err != nil || status != 200 || rawString(result["id"]) == "" {
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: rawString(result["id"])}
}
