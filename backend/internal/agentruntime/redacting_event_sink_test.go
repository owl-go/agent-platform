package agentruntime_test

import (
	"context"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/credentials"
)

func TestRedactingEventSinkFiltersPayloadBeforePersistence(t *testing.T) {
	const secret = "model-secret"
	recording := &eventRecordingSink{}
	sink := agentruntime.NewRedactingEventSink(credentials.NewRedactor([]byte(secret)), recording)
	event := agentruntime.Event{Payload: []byte(`{"command":"echo model-secret"}`)}

	if err := sink.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish event: %v", err)
	}
	got := string(recording.events[0].Payload)
	if strings.Contains(got, secret) {
		t.Fatalf("persisted event contains credential: %s", got)
	}
	if got != `{"command":"echo [REDACTED]"}` {
		t.Fatalf("persisted payload = %s", got)
	}
	if string(event.Payload) != `{"command":"echo model-secret"}` {
		t.Fatal("redacting sink mutated the adapter-owned event")
	}
}

type eventRecordingSink struct {
	events []agentruntime.Event
}

func (s *eventRecordingSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.events = append(s.events, event)
	return nil
}
