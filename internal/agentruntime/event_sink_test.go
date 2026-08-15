package agentruntime_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/internal/agentruntime"
)

func TestContractSinkRejectsSequenceGaps(t *testing.T) {
	downstream := &recordingSink{}
	sink := agentruntime.NewContractSink(context.Background(), "run-1", downstream)

	err := sink.Publish(context.Background(), agentruntime.Event{
		RunID:      "run-1",
		Sequence:   2,
		Kind:       agentruntime.EventRuntimeStarted,
		OccurredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected a sequence validation error")
	}
	if len(downstream.events) != 0 {
		t.Fatal("invalid event reached the downstream sink")
	}
}

func TestContractSinkRejectsEventsAfterTerminalEvent(t *testing.T) {
	downstream := &recordingSink{}
	sink := agentruntime.NewContractSink(context.Background(), "run-1", downstream)
	now := time.Now()

	for _, event := range []agentruntime.Event{
		{RunID: "run-1", Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: now},
		{RunID: "run-1", Sequence: 2, Kind: agentruntime.EventRuntimeCompleted, OccurredAt: now},
	} {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatalf("publish valid event: %v", err)
		}
	}

	err := sink.Publish(context.Background(), agentruntime.Event{
		RunID:      "run-1",
		Sequence:   3,
		Kind:       agentruntime.EventFileChanged,
		OccurredAt: now,
	})
	if err == nil {
		t.Fatal("expected an event-after-terminal validation error")
	}
	if len(downstream.events) != 2 {
		t.Fatal("event after terminal reached the downstream sink")
	}
}

func TestContractSinkRejectsInvalidEventIdentity(t *testing.T) {
	tests := map[string]agentruntime.Event{
		"different run": {
			RunID: "run-2", Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(),
		},
		"missing time": {
			RunID: "run-1", Sequence: 1, Kind: agentruntime.EventRuntimeStarted,
		},
		"unknown kind": {
			RunID: "run-1", Sequence: 1, Kind: agentruntime.EventKind("runtime.mystery"), OccurredAt: time.Now(),
		},
		"invalid payload": {
			RunID: "run-1", Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte("not-json"),
		},
	}

	for name, event := range tests {
		t.Run(name, func(t *testing.T) {
			downstream := &recordingSink{}
			sink := agentruntime.NewContractSink(context.Background(), "run-1", downstream)

			if err := sink.Publish(context.Background(), event); err == nil {
				t.Fatal("expected event identity validation error")
			}
			if len(downstream.events) != 0 {
				t.Fatal("invalid event reached the downstream sink")
			}
		})
	}
}

func TestContractSinkRejectsEventsAfterCancellation(t *testing.T) {
	downstream := &recordingSink{}
	sink := agentruntime.NewContractSink(context.Background(), "run-1", downstream)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := sink.Publish(ctx, agentruntime.Event{
		RunID:      "run-1",
		Sequence:   1,
		Kind:       agentruntime.EventRuntimeStarted,
		OccurredAt: time.Now(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if len(downstream.events) != 0 {
		t.Fatal("event published after cancellation")
	}
}

func TestContractSinkCloseRequiresTerminalEvent(t *testing.T) {
	sink := agentruntime.NewContractSink(context.Background(), "run-1", &recordingSink{})

	if err := sink.Publish(context.Background(), agentruntime.Event{
		RunID:      "run-1",
		Sequence:   1,
		Kind:       agentruntime.EventRuntimeStarted,
		OccurredAt: time.Now(),
	}); err != nil {
		t.Fatalf("publish start event: %v", err)
	}

	if err := sink.Close(); err == nil {
		t.Fatal("expected close to reject a stream without a terminal event")
	}
}

func TestContractSinkCloseAcceptsOneTerminalEvent(t *testing.T) {
	sink := agentruntime.NewContractSink(context.Background(), "run-1", &recordingSink{})
	now := time.Now()

	for _, event := range []agentruntime.Event{
		{RunID: "run-1", Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: now},
		{RunID: "run-1", Sequence: 2, Kind: agentruntime.EventRuntimeFailed, OccurredAt: now},
	} {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatalf("publish event: %v", err)
		}
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close complete stream: %v", err)
	}
}

type recordingSink struct {
	events []agentruntime.Event
}

func (s *recordingSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.events = append(s.events, event)
	return nil
}
