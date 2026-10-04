package domain

import (
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestChannelAudienceAndConversationIsolation(t *testing.T) {
	audience := ChannelAudience{SenderIDs: []string{"alice"}, GroupIDs: []string{"room"}, AllowDirect: true}
	message := ChannelMessage{SenderID: "alice", ChatID: "room", ThreadID: "topic", Group: true, Mentioned: true}
	if !audience.Allows(message) {
		t.Fatal("allowed group mention rejected")
	}
	for _, change := range []func(*ChannelMessage){func(m *ChannelMessage) { m.SenderID = "bob" }, func(m *ChannelMessage) { m.ChatID = "other" }, func(m *ChannelMessage) { m.Mentioned = false }, func(m *ChannelMessage) { m.Bot = true }} {
		invalid := message
		change(&invalid)
		if audience.Allows(invalid) {
			t.Fatal("unauthorized message admitted")
		}
	}
	channel := MessageChannel{ID: "channel", OwnerID: "owner", WorkflowID: "workflow", AccountID: "bot", TenantID: "tenant"}
	key := message.ConversationKey(channel)
	for _, change := range []func(*ChannelMessage){func(m *ChannelMessage) { m.SenderID = "bob" }, func(m *ChannelMessage) { m.ChatID = "other" }, func(m *ChannelMessage) { m.ThreadID = "other" }} {
		other := message
		change(&other)
		if other.ConversationKey(channel) == key {
			t.Fatal("conversation collision")
		}
	}
	other := channel
	other.OwnerID = "other"
	if message.ConversationKey(other) == key {
		t.Fatal("owner collision")
	}
}
func TestChannelInputWindowAndUnicodeChunks(t *testing.T) {
	now := time.Now()
	m := ChannelMessage{EventID: "event", MessageID: "message", SenderID: "sender", ChatID: "chat", Text: "question", OccurredAt: now}
	if m.Validate(now) != nil {
		t.Fatal("valid message rejected")
	}
	m.OccurredAt = now.Add(-25 * time.Hour)
	if m.Validate(now) == nil {
		t.Fatal("historical replay admitted")
	}
	text := strings.Repeat("😀中文", 1700)
	chunks := SplitChannelText(text)
	if strings.Join(chunks, "") != text {
		t.Fatal("Unicode content changed")
	}
	for _, chunk := range chunks {
		if len(utf16.Encode([]rune(chunk))) > 1800 {
			t.Fatal("UTF-16 limit exceeded")
		}
	}
}
