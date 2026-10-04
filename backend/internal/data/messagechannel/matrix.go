package messagechannel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

type Matrix struct {
	*HTTP
	cursor application.ChannelReceiveCursor
}

func (a *Matrix) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	target, err := a.endpoint(c["endpoint"], "/_matrix/client/v3/account/whoami")
	if err != nil || c["access_token"] == "" {
		return application.ChannelIdentity{}, providerError("matrix_credentials_invalid")
	}
	result, status, _, err := a.request(ctx, http.MethodGet, target, "Bearer "+c["access_token"], nil)
	id := rawString(result["user_id"])
	if err != nil || status != 200 || !strings.HasPrefix(id, "@") || rawBool(result["is_guest"]) {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	return application.ChannelIdentity{ID: id, Name: id, BindingID: c["endpoint"] + ":" + id}, nil
}
func (a *Matrix) ValidateAudience(audience domain.ChannelAudience) error {
	// Matrix scopes all messages to rooms, including direct rooms; never infer privacy.
	if audience.AllowDirect || len(audience.GroupIDs) == 0 {
		return providerError("matrix_explicit_rooms_required")
	}
	return nil
}
func (a *Matrix) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	if err := sameIdentity(ctx, a, s, c); err != nil {
		return err
	}
	for _, room := range s.Channel.Audience.GroupIDs {
		if err := a.ensurePlainRoom(ctx, c, room); err != nil {
			return err
		}
	}
	return nil
}
func (a *Matrix) ensurePlainRoom(ctx context.Context, c application.ChannelCredentials, room string) error {
	target, err := a.endpoint(c["endpoint"], "/_matrix/client/v3/rooms/"+url.PathEscape(room)+"/state/m.room.encryption")
	if err != nil {
		return err
	}
	result, status, _, err := a.request(ctx, http.MethodGet, target, "Bearer "+c["access_token"], nil)
	if err != nil || (status != 404 && status != 200) {
		return providerError("matrix_room_unavailable")
	}
	if status == 200 || rawString(result["errcode"]) != "M_NOT_FOUND" {
		return providerError("matrix_encryption_unsupported")
	}
	return nil
}

type matrixEvent struct {
	ID        string `json:"event_id"`
	Sender    string `json:"sender"`
	Type      string `json:"type"`
	Timestamp int64  `json:"origin_server_ts"`
	Content   struct {
		MsgType  string `json:"msgtype"`
		Body     string `json:"body"`
		Mentions struct {
			Users []string `json:"user_ids"`
		} `json:"m.mentions"`
		Relation struct {
			Type    string `json:"rel_type"`
			EventID string `json:"event_id"`
		} `json:"m.relates_to"`
	} `json:"content"`
	Unsigned struct {
		Redacted json.RawMessage `json:"redacted_because"`
	} `json:"unsigned"`
}

func normalizeMatrix(s application.ChannelStored, room string, e matrixEvent) (domain.ChannelMessage, bool) {
	if e.Type != "m.room.message" || e.Content.MsgType != "m.text" || e.Sender == s.Channel.AccountID || len(e.Unsigned.Redacted) > 0 || e.Content.Relation.Type == "m.replace" {
		return domain.ChannelMessage{}, false
	}
	mentioned := false
	for _, id := range e.Content.Mentions.Users {
		if id == s.Channel.AccountID {
			mentioned = true
		}
	}
	thread := ""
	if e.Content.Relation.Type == "m.thread" {
		thread = e.Content.Relation.EventID
	}
	return domain.ChannelMessage{EventID: e.ID, MessageID: e.ID, SenderID: e.Sender, ChatID: room, ThreadID: thread, Group: true, Mentioned: mentioned, Text: e.Content.Body, OccurredAt: time.UnixMilli(e.Timestamp), Reply: map[string]string{}}, true
}
func (a *Matrix) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	return pollMessages(ctx, s, a.cursor, sink, func(ctx context.Context, since string) ([]domain.ChannelMessage, string, error) {
		filter, _ := json.Marshal(map[string]any{"room": map[string]any{"rooms": s.Channel.Audience.GroupIDs, "timeline": map[string]any{"limit": 100}, "state": map[string]any{"types": []string{"m.room.encryption"}}, "ephemeral": map[string]any{"types": []string{}}}})
		query := url.Values{"timeout": {"25000"}, "filter": {string(filter)}, "set_presence": {"offline"}}
		if since != "" {
			query.Set("since", since)
		} else {
			query.Set("timeout", "0")
		}
		target, err := a.endpoint(c["endpoint"], "/_matrix/client/v3/sync?"+query.Encode())
		if err != nil {
			return nil, "", err
		}
		result, status, _, err := a.request(ctx, http.MethodGet, target, "Bearer "+c["access_token"], nil)
		if err != nil || status != 200 {
			return nil, "", providerError("provider_receive_failed")
		}
		next := rawString(result["next_batch"])
		if next == "" {
			return nil, "", providerError("provider_response_invalid")
		}
		var rooms struct {
			Join map[string]struct {
				State struct {
					Events []matrixEvent `json:"events"`
				} `json:"state"`
				Timeline struct {
					Events  []matrixEvent `json:"events"`
					Limited bool          `json:"limited"`
				} `json:"timeline"`
			} `json:"join"`
		}
		if len(result["rooms"]) > 0 && json.Unmarshal(result["rooms"], &rooms) != nil {
			return nil, "", providerError("provider_response_invalid")
		}
		var messages []domain.ChannelMessage
		for room, data := range rooms.Join {
			allowed := false
			for _, id := range s.Channel.Audience.GroupIDs {
				if id == room {
					allowed = true
				}
			}
			if !allowed {
				continue
			}
			for _, events := range [][]matrixEvent{data.State.Events, data.Timeline.Events} {
				for _, event := range events {
					if event.Type == "m.room.encryption" || event.Type == "m.room.encrypted" {
						return nil, "", providerError("matrix_encryption_unsupported")
					}
				}
			}
			if since == "" {
				continue
			} // Initial snapshot is a baseline, not new questions.
			if data.Timeline.Limited {
				return nil, "", providerError("matrix_history_gap")
			}
			for _, event := range data.Timeline.Events {
				if m, ok := normalizeMatrix(s, room, event); ok {
					messages = append(messages, m)
				}
			}
		}
		return messages, next, nil
	})
}
func (a *Matrix) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	if err := a.ensurePlainRoom(ctx, c, m.ChatID); err != nil {
		return application.ChannelSendResult{State: "failed", Code: "matrix_room_unavailable"}
	}
	target, err := a.endpoint(c["endpoint"], "/_matrix/client/v3/rooms/"+url.PathEscape(m.ChatID)+"/send/m.room.message/"+url.PathEscape(key))
	if err != nil {
		return application.ChannelSendResult{State: "failed", Code: "provider_endpoint_not_approved"}
	}
	body := map[string]any{"msgtype": "m.text", "body": text, "m.mentions": map[string]any{"user_ids": []string{m.SenderID}}, "m.relates_to": map[string]any{"m.in_reply_to": map[string]string{"event_id": m.MessageID}}}
	if m.ThreadID != "" {
		body["m.relates_to"] = map[string]any{"rel_type": "m.thread", "event_id": m.ThreadID, "is_falling_back": false, "m.in_reply_to": map[string]string{"event_id": m.MessageID}}
	}
	result, status, retry, err := a.request(ctx, http.MethodPut, target, "Bearer "+c["access_token"], body)
	if err != nil || status != 200 || rawString(result["event_id"]) == "" {
		if ms := rawNumber(result["retry_after_ms"]); ms > 0 {
			retry = time.Duration(ms) * time.Millisecond
		}
		return failedSend(status, retry, err)
	}
	return application.ChannelSendResult{State: "sent", MessageID: rawString(result["event_id"])}
}
