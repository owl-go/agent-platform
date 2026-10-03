package messagechannel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type WhatsApp struct{ *HTTP }

var graphVersion = regexp.MustCompile(`^v[1-9][0-9]?\.0$`)

func (a *WhatsApp) ValidateAudience(audience domain.ChannelAudience) error {
	return directAudience(audience)
}
func whatsappBase(c application.ChannelCredentials) string {
	return "https://graph.facebook.com/" + c["graph_version"] + "/" + url.PathEscape(c["phone_number_id"])
}
func (a *WhatsApp) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["access_token"] == "" || c["app_secret"] == "" || c["verify_token"] == "" || c["phone_number_id"] == "" || c["business_account_id"] == "" || !graphVersion.MatchString(c["graph_version"]) {
		return application.ChannelIdentity{}, providerError("whatsapp_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodGet, whatsappBase(c)+"?fields=id,verified_name", "Bearer "+c["access_token"], nil)
	if err != nil || status != 200 || rawString(result["id"]) != c["phone_number_id"] {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	// Check that the authenticated phone belongs to the declared WABA.
	phones, status, _, err := a.request(ctx, http.MethodGet, "https://graph.facebook.com/"+c["graph_version"]+"/"+url.PathEscape(c["business_account_id"])+"/phone_numbers?fields=id&limit=100", "Bearer "+c["access_token"], nil)
	var entries []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(phones["data"], &entries)
	found := false
	for _, entry := range entries {
		if entry.ID == c["phone_number_id"] {
			found = true
		}
	}
	if err != nil || status != 200 || !found {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: c["phone_number_id"], TenantID: c["business_account_id"], BindingID: c["phone_number_id"], Name: rawString(result["verified_name"])}, nil
}
func (a *WhatsApp) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}
func (a *WhatsApp) Challenge(_ context.Context, _ application.ChannelStored, c application.ChannelCredentials, query url.Values) (string, error) {
	if query.Get("hub.mode") != "subscribe" || query.Get("hub.challenge") == "" || len(query.Get("hub.challenge")) > 1024 || !hmac.Equal([]byte(query.Get("hub.verify_token")), []byte(c["verify_token"])) {
		return "", providerError("callback_authentication_failed")
	}
	return query.Get("hub.challenge"), nil
}
func (a *WhatsApp) Callback(_ context.Context, s application.ChannelStored, c application.ChannelCredentials, headers http.Header, body []byte) (application.ChannelCallback, error) {
	expected := hmac.New(sha256.New, []byte(c["app_secret"]))
	expected.Write(body)
	signature, err := hex.DecodeString(strings.TrimPrefix(headers.Get("X-Hub-Signature-256"), "sha256="))
	if err != nil || !strings.HasPrefix(headers.Get("X-Hub-Signature-256"), "sha256=") || c["app_secret"] == "" || !hmac.Equal(signature, expected.Sum(nil)) {
		return application.ChannelCallback{}, providerError("callback_authentication_failed")
	}
	var payload struct {
		Object string `json:"object"`
		Entry  []struct {
			ID      string `json:"id"`
			Changes []struct {
				Field string `json:"field"`
				Value struct {
					Product  string `json:"messaging_product"`
					Metadata struct {
						Phone string `json:"phone_number_id"`
					} `json:"metadata"`
					Messages []struct {
						ID        string `json:"id"`
						From      string `json:"from"`
						Type      string `json:"type"`
						Timestamp string `json:"timestamp"`
						Text      struct {
							Body string `json:"body"`
						} `json:"text"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Object != "whatsapp_business_account" {
		return application.ChannelCallback{}, providerError("provider_payload_invalid")
	}
	response := application.ChannelCallback{Response: map[string]bool{"ok": true}}
	for _, entry := range payload.Entry {
		if entry.ID != s.Channel.TenantID {
			continue
		}
		for _, change := range entry.Changes {
			if change.Field != "messages" || change.Value.Product != "whatsapp" || change.Value.Metadata.Phone != s.Channel.AccountID {
				continue
			}
			for _, event := range change.Value.Messages {
				timestamp, err := strconv.ParseInt(event.Timestamp, 10, 64)
				if err != nil || event.Type != "text" || event.From == s.Channel.AccountID {
					continue
				}
				response.Messages = append(response.Messages, domain.ChannelMessage{EventID: event.ID, MessageID: event.ID, SenderID: event.From, ChatID: event.From, Text: event.Text.Body, OccurredAt: time.Unix(timestamp, 0), Reply: map[string]string{"expires_at": strconv.FormatInt(time.Unix(timestamp, 0).Add(24*time.Hour).Unix(), 10)}})
			}
		}
	}
	return response, nil
}
func (a *WhatsApp) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	expires, err := strconv.ParseInt(m.Reply["expires_at"], 10, 64)
	if err != nil || !time.Now().Before(time.Unix(expires, 0)) {
		return application.ChannelSendResult{State: "expired", Code: "reply_window_expired"}
	}
	if m.Group || !graphVersion.MatchString(c["graph_version"]) || c["phone_number_id"] != s.Channel.AccountID {
		return application.ChannelSendResult{State: "failed", Code: "reply_unavailable"}
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, whatsappBase(c)+"/messages", "Bearer "+c["access_token"], map[string]any{"messaging_product": "whatsapp", "recipient_type": "individual", "to": m.SenderID, "type": "text", "text": map[string]any{"body": text, "preview_url": false}, "context": map[string]string{"message_id": m.MessageID}})
	var messages []struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(result["messages"], &messages)
	if err != nil || status != 200 || len(messages) != 1 || messages[0].ID == "" {
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: messages[0].ID}
}
