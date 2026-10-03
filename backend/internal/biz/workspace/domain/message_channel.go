package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrChannelCallbackAuthentication = errors.New("message channel callback authentication failed")

// MessageChannel is a Workflow-owned transport, independent of its Runtime.
// Credentials and provider reply capabilities never appear in this projection.
type MessageChannel struct {
	BindingID       string          `json:"-"`
	ID              string          `json:"id"`
	OwnerID         string          `json:"-"`
	WorkflowID      string          `json:"workflow_id"`
	Provider        string          `json:"provider"`
	Name            string          `json:"name"`
	AccountID       string          `json:"account_id"`
	AccountName     string          `json:"account_name"`
	TenantID        string          `json:"tenant_id"`
	Region          string          `json:"region"`
	Audience        ChannelAudience `json:"audience"`
	Enabled         bool            `json:"enabled"`
	Version         int64           `json:"version"`
	ConfigVersion   int64           `json:"config_version"`
	ValidationState string          `json:"validation_state"`
	ValidationCode  string          `json:"validation_code"`
	ValidationUntil *time.Time      `json:"validation_until,omitempty"`
	Health          string          `json:"health"`
	ErrorCode       string          `json:"error_code"`
	CallbackURL     string          `json:"callback_url"`
}

type ChannelAudience struct {
	SenderIDs   []string `json:"sender_ids"`
	GroupIDs    []string `json:"group_ids"`
	AllowDirect bool     `json:"allow_direct"`
}

func (a ChannelAudience) Validate() error {
	if len(a.SenderIDs) == 0 || len(a.SenderIDs) > 100 || len(a.GroupIDs) > 100 || !a.AllowDirect && len(a.GroupIDs) == 0 {
		return fmt.Errorf("%w: choose explicit senders and chat scope", ErrInvalid)
	}
	for _, ids := range [][]string{a.SenderIDs, a.GroupIDs} {
		for _, id := range ids {
			if id == "" || len(id) > 200 || strings.TrimSpace(id) != id || strings.ContainsAny(id, "\r\n\x00") {
				return fmt.Errorf("%w: invalid external identity", ErrInvalid)
			}
		}
	}
	return nil
}

type ChannelMessage struct {
	EventID    string    `json:"event_id"`
	MessageID  string    `json:"message_id"`
	SenderID   string    `json:"sender_id"`
	ChatID     string    `json:"chat_id"`
	ThreadID   string    `json:"thread_id"`
	Group      bool      `json:"group"`
	Mentioned  bool      `json:"mentioned"`
	Bot        bool      `json:"bot"`
	Text       string    `json:"text"`
	OccurredAt time.Time `json:"occurred_at"`
	// Reply is supplied exclusively by the authenticated transport, then encrypted.
	Reply map[string]string `json:"-"`
}

func (a ChannelAudience) Allows(m ChannelMessage) bool {
	return !m.Bot && slices.Contains(a.SenderIDs, m.SenderID) && ((!m.Group && a.AllowDirect) || (m.Group && m.Mentioned && slices.Contains(a.GroupIDs, m.ChatID)))
}

func (m ChannelMessage) Validate(now time.Time) error {
	for _, id := range []string{m.EventID, m.MessageID, m.SenderID, m.ChatID} {
		if id == "" || !utf8.ValidString(id) || len(id) > 256 || strings.ContainsAny(id, "\x00\r\n") {
			return ErrInvalid
		}
	}
	if !utf8.ValidString(m.ThreadID) || strings.ContainsAny(m.ThreadID, "\x00\r\n") || len(m.ThreadID) > 256 || !utf8.ValidString(m.Text) || len(m.Text) > 10_000 || strings.TrimSpace(m.Text) == "" || m.OccurredAt.Before(now.Add(-24*time.Hour)) || m.OccurredAt.After(now.Add(5*time.Minute)) {
		return ErrInvalid
	}
	return nil
}

func (m ChannelMessage) ConversationKey(channel MessageChannel) string {
	data, _ := json.Marshal([]string{channel.OwnerID, channel.WorkflowID, channel.ID, channel.AccountID, channel.TenantID, m.ChatID, m.ThreadID, m.SenderID})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SplitChannelText leaves room for provider UTF-16 limits and JSON escaping.
func SplitChannelText(text string) []string {
	runes := []rune(text)
	if len(runes) > 50_000 {
		runes = append(runes[:49_950], []rune("\n[Answer truncated; view the complete result in Agent Workspace.]")...)
	}
	chunks := []string{}
	for len(runes) > 0 {
		n := min(900, len(runes))
		chunks = append(chunks, string(runes[:n]))
		runes = runes[n:]
	}
	return chunks
}

type ChannelDelivery struct {
	ID                string    `json:"id"`
	ChannelID         string    `json:"channel_id"`
	RunID             string    `json:"run_id"`
	Kind              string    `json:"kind"`
	Chunk             int       `json:"chunk"`
	State             string    `json:"state"`
	Attempts          int       `json:"attempts"`
	ErrorCode         string    `json:"error_code"`
	ProviderMessageID string    `json:"provider_message_id"`
	CreatedAt         time.Time `json:"created_at"`
}
