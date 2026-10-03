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
)

type BlueBubbles struct {
	*HTTP
	cursor application.ChannelReceiveCursor
}

func (a *BlueBubbles) ValidateAudience(audience domain.ChannelAudience) error {
	return directAudience(audience)
}
func (a *BlueBubbles) call(ctx context.Context, c application.ChannelCredentials, method, path string, body any) (map[string]json.RawMessage, int, time.Duration, error) {
	target, err := a.endpoint(c["endpoint"], path)
	if err != nil {
		return nil, 0, 0, err
	}
	target += "?password=" + url.QueryEscape(c["password"])
	return a.request(ctx, method, target, "", body)
}
func (a *BlueBubbles) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["password"] == "" {
		return application.ChannelIdentity{}, providerError("bluebubbles_credentials_invalid")
	}
	result, status, _, err := a.call(ctx, c, http.MethodGet, "/api/v1/server/info", nil)
	var info struct {
		OSVersion     string `json:"os_version"`
		ServerVersion string `json:"server_version"`
		ComputerID    string `json:"computer_id"`
		Account       string `json:"detected_imessage"`
	}
	_ = json.Unmarshal(result["data"], &info)
	if err != nil || status != 200 || rawNumber(result["status"]) != 200 || info.ServerVersion == "" || info.ComputerID == "" || info.Account == "" {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	// The authenticated server reports its active iMessage account and computer ID.
	// Recheck them on reception and sending to prevent a server account switch.
	return application.ChannelIdentity{ID: info.Account, Name: "iMessage", BindingID: info.ComputerID + ":" + info.Account}, nil
}
func (a *BlueBubbles) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}

type blueMessage struct {
	GUID           string `json:"guid"`
	RowID          int64  `json:"originalROWID"`
	Text           string `json:"text"`
	DateCreated    int64  `json:"dateCreated"`
	FromMe         bool   `json:"isFromMe"`
	ItemType       int    `json:"itemType"`
	AssociatedType int    `json:"associatedMessageType"`
	Edited         int64  `json:"dateEdited"`
	Retracted      int64  `json:"dateRetracted"`
	Handle         struct {
		Address string `json:"address"`
	} `json:"handle"`
	Chats []struct {
		GUID         string `json:"guid"`
		Participants []struct {
			Address string `json:"address"`
		} `json:"participants"`
	} `json:"chats"`
}

func normalizeBlueBubbles(e blueMessage) (domain.ChannelMessage, bool) {
	if e.FromMe || e.Text == "" || e.GUID == "" || e.ItemType != 0 || e.AssociatedType != 0 || e.Edited != 0 || e.Retracted != 0 || len(e.Chats) != 1 || len(e.Chats[0].Participants) != 1 || e.Handle.Address == "" || !strings.HasPrefix(e.Chats[0].GUID, "iMessage;") {
		return domain.ChannelMessage{}, false
	}
	return domain.ChannelMessage{EventID: e.GUID, MessageID: e.GUID, SenderID: e.Handle.Address, ChatID: e.Chats[0].GUID, Text: e.Text, OccurredAt: time.UnixMilli(e.DateCreated), Reply: map[string]string{}}, true
}
func (a *BlueBubbles) query(ctx context.Context, c application.ChannelCredentials, body any) ([]blueMessage, error) {
	result, status, _, err := a.call(ctx, c, http.MethodPost, "/api/v1/message/query", body)
	if err != nil || status != 200 || rawNumber(result["status"]) != 200 {
		return nil, providerError("provider_receive_failed")
	}
	var events []blueMessage
	if json.Unmarshal(result["data"], &events) != nil {
		return nil, providerError("provider_response_invalid")
	}
	return events, nil
}
func (a *BlueBubbles) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	return pollMessages(ctx, s, a.cursor, sink, func(ctx context.Context, cursor string) ([]domain.ChannelMessage, string, error) {
		if err := sameIdentity(ctx, a, s, c); err != nil {
			return nil, "", err
		}
		// Fixed SQL and bound numeric arguments use the server's insertion identity,
		// rather than sender/host clocks. No inbound text can become a query.
		latest, err := a.query(ctx, c, map[string]any{"limit": 1, "with": []string{}, "where": []any{map[string]any{"statement": "message.ROWID = (SELECT MAX(ROWID) FROM message)", "args": map[string]any{}}}})
		if err != nil {
			return nil, "", err
		}
		through := int64(0)
		if len(latest) > 1 {
			return nil, "", providerError("provider_response_invalid")
		}
		if len(latest) == 1 {
			through = latest[0].RowID
			if through <= 0 || through > 9007199254740991 {
				return nil, "", providerError("provider_response_invalid")
			}
		}
		if cursor == "" {
			return nil, strconv.FormatInt(through, 10), nil
		}
		after, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || after < 0 || after > 9007199254740991 {
			return nil, "", providerError("channel_cursor_unavailable")
		}
		if through < after {
			return nil, "", providerError("bluebubbles_history_reset")
		}
		if through == after {
			return nil, cursor, nil
		}
		var messages []domain.ChannelMessage
		for offset := 0; offset < 1000; offset += 100 {
			events, err := a.query(ctx, c, map[string]any{"offset": offset, "limit": 100, "sort": "ASC", "with": []string{"chats", "chats.participants"}, "where": []any{map[string]any{"statement": "message.ROWID > :after AND message.ROWID <= :through", "args": map[string]int64{"after": after, "through": through}}}})
			if err != nil {
				return nil, "", err
			}
			for _, event := range events {
				if event.RowID <= after || event.RowID > through {
					return nil, "", providerError("provider_response_invalid")
				}
				if m, ok := normalizeBlueBubbles(event); ok {
					messages = append(messages, m)
				}
			}
			if len(events) < 100 {
				return messages, strconv.FormatInt(through, 10), nil
			}
		}
		return nil, "", providerError("channel_backlog_full")
	})
}
func (a *BlueBubbles) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	if err := sameIdentity(ctx, a, s, c); err != nil {
		return application.ChannelSendResult{State: "failed", Code: "provider_identity_changed"}
	}
	if m.Group {
		return application.ChannelSendResult{State: "failed", Code: "provider_direct_messages_only"}
	}
	result, status, retry, err := a.call(ctx, c, http.MethodPost, "/api/v1/message/text", map[string]any{"chatGuid": m.ChatID, "message": text, "method": "apple-script", "tempGuid": key})
	var message struct {
		GUID string `json:"guid"`
	}
	_ = json.Unmarshal(result["data"], &message)
	if err != nil || status != 200 || rawNumber(result["status"]) != 200 || message.GUID == "" {
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: message.GUID}
}
