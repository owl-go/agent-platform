package messagechannel

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

type Signal struct {
	*HTTP
	cursor application.ChannelReceiveCursor
}

func (a *Signal) rpc(ctx context.Context, c application.ChannelCredentials, method string, params any, key string) (json.RawMessage, int, time.Duration, error) {
	target, err := a.endpoint(c["endpoint"], "/api/v1/rpc")
	if err != nil {
		return nil, 0, 0, err
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, target, "Bearer "+c["bridge_token"], map[string]any{"jsonrpc": "2.0", "id": key, "method": method, "params": params})
	if err != nil || status != 200 || result["error"] != nil || rawString(result["id"]) != key || result["result"] == nil {
		return nil, status, retry, providerError("provider_rpc_failed")
	}
	return result["result"], status, retry, nil
}
func (a *Signal) Identify(ctx context.Context, c application.ChannelCredentials, _ string) (application.ChannelIdentity, error) {
	if c["bridge_token"] == "" || c["account_id"] == "" {
		return application.ChannelIdentity{}, providerError("signal_credentials_invalid")
	}
	data, _, _, err := a.rpc(ctx, c, "listAccounts", map[string]any{}, uuid.NewString())
	var accounts []struct {
		Number string `json:"number"`
		ACI    string `json:"aci"`
	}
	if err != nil || json.Unmarshal(data, &accounts) != nil {
		return application.ChannelIdentity{}, providerError("provider_identity_failed")
	}
	for _, account := range accounts {
		if account.ACI != "" && (account.ACI == c["account_id"] || account.Number == c["account_id"]) {
			return application.ChannelIdentity{ID: account.ACI, Name: account.ACI, BindingID: c["endpoint"] + ":" + account.ACI}, nil
		}
	}
	return application.ChannelIdentity{}, providerError("provider_identity_failed")
}
func (a *Signal) Configure(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ string) error {
	return sameIdentity(ctx, a, s, c)
}
func normalizeSignal(s application.ChannelStored, c application.ChannelCredentials, data []byte) (domain.ChannelMessage, bool) {
	var payload struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(data, &payload) != nil || payload.Method != "receive" {
		return domain.ChannelMessage{}, false
	}
	var outer struct {
		Result json.RawMessage `json:"result"`
	}
	_ = json.Unmarshal(payload.Params, &outer)
	body := payload.Params
	if len(outer.Result) > 0 {
		body = outer.Result
	}
	var event struct {
		Account  string `json:"account"`
		Envelope struct {
			Source    string `json:"sourceUuid"`
			Number    string `json:"sourceNumber"`
			Timestamp int64  `json:"timestamp"`
			Message   *struct {
				Timestamp int64  `json:"timestamp"`
				Text      string `json:"message"`
				ViewOnce  bool   `json:"viewOnce"`
				Expires   int    `json:"expiresInSeconds"`
				Mentions  []struct {
					UUID string `json:"uuid"`
				} `json:"mentions"`
				Group *struct {
					ID   string `json:"groupId"`
					Type string `json:"type"`
				} `json:"groupInfo"`
			} `json:"dataMessage"`
		} `json:"envelope"`
	}
	if json.Unmarshal(body, &event) != nil || (event.Account != c["account_id"] && event.Account != s.Channel.AccountID) || event.Envelope.Message == nil {
		return domain.ChannelMessage{}, false
	}
	e, msg := event.Envelope, event.Envelope.Message
	if e.Source == "" || e.Source == s.Channel.AccountID || msg.ViewOnce || msg.Expires > 0 || msg.Text == "" {
		return domain.ChannelMessage{}, false
	}
	group, chat := false, e.Source
	if msg.Group != nil {
		if msg.Group.Type != "DELIVER" || msg.Group.ID == "" {
			return domain.ChannelMessage{}, false
		}
		group, chat = true, msg.Group.ID
	}
	mentioned := false
	for _, mention := range msg.Mentions {
		if mention.UUID == s.Channel.AccountID {
			mentioned = true
		}
	}
	id := e.Source + ":" + strconv.FormatInt(msg.Timestamp, 10)
	return domain.ChannelMessage{EventID: id, MessageID: id, SenderID: e.Source, ChatID: chat, Group: group, Mentioned: mentioned, Text: msg.Text, OccurredAt: time.UnixMilli(msg.Timestamp), Reply: map[string]string{}}, true
}
func (a *Signal) Connect(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, sink application.ChannelMessageSink) error {
	if err := sameIdentity(ctx, a, s, c); err != nil {
		return err
	}
	if a.cursor == nil {
		return providerError("channel_cursor_unavailable")
	}
	last, err := a.cursor.Load(ctx, s)
	if err != nil {
		return err
	}
	target, err := a.endpoint(c["endpoint"], "/api/v1/events")
	if err != nil {
		return err
	}
	client := *a.approved[c["endpoint"]]
	client.Timeout = 0
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return providerError("provider_request_invalid")
	}
	req.Header.Set("Authorization", "Bearer "+c["bridge_token"])
	req.Header.Set("Accept", "text/event-stream")
	if last != "" {
		req.Header.Set("Last-Event-ID", last)
	}
	response, err := client.Do(req)
	if err != nil {
		return providerError("provider_receive_failed")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return providerError("provider_response_invalid")
	}
	reader := bufio.NewScanner(response.Body)
	reader.Buffer(make([]byte, 4096), 65536)
	id := ""
	data := ""
	commit := func() error {
		if data == "" {
			return nil
		}
		if m, ok := normalizeSignal(s, c, []byte(data)); ok {
			commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := sink(commitCtx, m)
			cancel()
			if err != nil {
				return err
			}
		}
		if id != "" {
			if err := a.cursor.Save(ctx, s, id); err != nil {
				return err
			}
		}
		return nil
	}
	for reader.Scan() {
		line := reader.Text()
		if line == "" {
			if err := commit(); err != nil {
				return err
			}
			id, data = "", ""
			continue
		}
		if strings.HasPrefix(line, "id:") {
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
		if strings.HasPrefix(line, "data:") {
			if data != "" {
				data += "\n"
			}
			data += strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if len(data) > 65536 {
				return providerError("provider_payload_invalid")
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return providerError("provider_connection_closed")
}
func (a *Signal) Send(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string) application.ChannelSendResult {
	if err := sameIdentity(ctx, a, s, c); err != nil {
		return application.ChannelSendResult{State: "failed", Code: "provider_identity_changed"}
	}
	params := map[string]any{"account": c["account_id"], "message": text}
	if m.Group {
		params["groupId"] = m.ChatID
		params["message"] = "[" + m.SenderID + "] " + text
	} else {
		params["recipient"] = []string{m.SenderID}
	}
	data, status, retry, err := a.rpc(ctx, c, "send", params, key)
	if err != nil {
		return failedSend(status, retry, err)
	}
	var result struct {
		Timestamp int64 `json:"timestamp"`
	}
	if json.Unmarshal(data, &result) != nil || result.Timestamp <= 0 {
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	return application.ChannelSendResult{State: "sent", MessageID: strconv.FormatInt(result.Timestamp, 10)}
}
