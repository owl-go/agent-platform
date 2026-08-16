package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

type ContractSink struct {
	mu         sync.Mutex
	lifecycle  context.Context
	runID      string
	next       int64
	terminal   bool
	closed     bool
	downstream EventSink
}

func NewContractSink(lifecycle context.Context, runID string, downstream EventSink) *ContractSink {
	return &ContractSink{
		lifecycle:  lifecycle,
		runID:      runID,
		next:       1,
		downstream: downstream,
	}
}

func (s *ContractSink) Publish(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.lifecycle.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return fmt.Errorf("runtime event stream is closed")
	}
	if s.terminal {
		return fmt.Errorf("runtime event stream already reached a terminal event")
	}
	if event.RunID != s.runID {
		return fmt.Errorf("runtime event run: got %q, want %q", event.RunID, s.runID)
	}
	if event.OccurredAt.IsZero() {
		return fmt.Errorf("runtime event time is required")
	}
	if !knownEventKind(event.Kind) {
		return fmt.Errorf("runtime event kind %q is unknown", event.Kind)
	}
	if len(event.Payload) > 0 && !json.Valid(event.Payload) {
		return fmt.Errorf("runtime event payload must be valid JSON")
	}
	if event.Sequence != s.next {
		return fmt.Errorf("runtime event sequence: got %d, want %d", event.Sequence, s.next)
	}
	if err := s.downstream.Publish(ctx, event); err != nil {
		return &Error{
			Code:    ErrorEventDeliveryFailed,
			Message: "publish runtime event",
			Cause:   err,
		}
	}
	s.next++
	s.terminal = event.Kind == EventRuntimeCompleted || event.Kind == EventRuntimeFailed
	return nil
}

func (s *ContractSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	if !s.terminal {
		return fmt.Errorf("runtime event stream closed without a terminal event")
	}
	return nil
}

func knownEventKind(kind EventKind) bool {
	switch kind {
	case EventRuntimeStarted,
		EventMessageDelta,
		EventMessageCompleted,
		EventCommandRequested,
		EventCommandCompleted,
		EventFileChanged,
		EventApprovalRequested,
		EventUsageUpdated,
		EventCheckpointSaved,
		EventRuntimeCompleted,
		EventRuntimeFailed:
		return true
	default:
		return false
	}
}
