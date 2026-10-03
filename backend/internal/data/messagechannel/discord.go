package messagechannel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/bwmarrin/discordgo"
)

type Discord struct{ *HTTP }

func (a *Discord) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["bot_token"] == "" {
		return application.ChannelIdentity{}, providerError("bot_token_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodGet, "https://discord.com/api/v10/users/@me", "Bot "+c["bot_token"], nil)
	if err != nil || status != 200 || !rawBool(result["bot"]) || rawString(result["id"]) == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: rawString(result["id"]), BindingID: rawString(result["id"]), Name: rawString(result["username"])}, nil
}
func (a *Discord) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	identity, err := a.Identify(ctx, c, "")
	if err != nil {
		return err
	}
	if identity.ID != s.Channel.AccountID {
		return providerError("provider_identity_changed")
	}
	return nil
}
func (a *Discord) Callback(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, h http.Header, b []byte) (application.ChannelCallback, error) {
	return noCallback(ctx, s, c, h, b)
}

func normalizeDiscord(s application.ChannelStored, event *discordgo.MessageCreate) (domain.ChannelMessage, bool) {
	if event == nil || event.Message == nil || event.Author == nil || event.Author.Bot || event.WebhookID != "" || event.Author.ID == s.Channel.AccountID || event.Content == "" || (event.Type != discordgo.MessageTypeDefault && event.Type != discordgo.MessageTypeReply) {
		return domain.ChannelMessage{}, false
	}
	text := event.Content
	mentioned := false
	for _, user := range event.Mentions {
		if user != nil && user.ID == s.Channel.AccountID {
			mentioned = true
			text = strings.ReplaceAll(strings.ReplaceAll(text, "<@"+user.ID+">", ""), "<@!"+user.ID+">", "")
		}
	}
	// A Discord thread is its own stable channel identity; no name-based merging.
	m := domain.ChannelMessage{EventID: event.ID, MessageID: event.ID, SenderID: event.Author.ID, ChatID: event.ChannelID, Group: event.GuildID != "", Mentioned: mentioned, Text: strings.TrimSpace(text), OccurredAt: event.Timestamp, Reply: map[string]string{}}
	return m, true
}
func (a *Discord) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, receive func(context.Context, domain.ChannelMessage) error) error {
	session, err := discordgo.New("Bot " + c["bot_token"])
	if err != nil {
		return providerError("provider_connection_failed")
	}
	session.Client = a.client
	session.LogLevel = -1
	session.SyncEvents = true
	session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent
	failed := make(chan struct{}, 1)
	// SDK diagnostics may contain user messages. Product health uses safe codes.
	session.AddHandler(func(_ *discordgo.Session, event *discordgo.MessageCreate) {
		if m, ok := normalizeDiscord(s, event); ok {
			if err := receive(ctx, m); err != nil {
				select {
				case failed <- struct{}{}:
				default:
				}
			}
		}
	})
	if err = session.Open(); err != nil {
		return providerError("provider_connection_failed")
	}
	defer session.Close()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-failed:
		return providerError("channel_inbox_unavailable")
	}
}
func (a *Discord) Send(ctx context.Context, _ application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	sum := sha256.Sum256([]byte(key))
	nonce := strconv.FormatUint(binary.BigEndian.Uint64(sum[:8]), 10)
	body := map[string]any{"content": text, "nonce": nonce, "enforce_nonce": true, "allowed_mentions": map[string]any{"parse": []string{}, "replied_user": false}, "message_reference": map[string]any{"message_id": m.MessageID, "channel_id": m.ChatID, "fail_if_not_exists": true}, "flags": 4}
	result, status, retry, err := a.request(ctx, http.MethodPost, "https://discord.com/api/v10/channels/"+m.ChatID+"/messages", "Bot "+c["bot_token"], body)
	if err != nil || status != 200 || rawString(result["id"]) == "" {
		if status == 429 {
			var seconds float64
			_ = json.Unmarshal(result["retry_after"], &seconds)
			retry = time.Duration(seconds * float64(time.Second))
		}
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: rawString(result["id"])}
}
