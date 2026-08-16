package runworker_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/runtimefake"
	"agent-platform/backend/internal/runworker"
)

func TestRunnerExecutesThroughRuntimeAdapter(t *testing.T) {
	adapter := &runtimefake.Adapter{
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			now := time.Now()
			for _, event := range []agentruntime.Event{
				{RunID: request.RunID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: now},
				{RunID: request.RunID, Sequence: 2, Kind: agentruntime.EventRuntimeCompleted, OccurredAt: now},
			} {
				if err := events.Publish(ctx, event); err != nil {
					return agentruntime.Result{}, err
				}
			}
			return agentruntime.Result{FinalMessage: "done", ExitCode: 0}, nil
		},
	}
	recorded := &recordingSink{}
	runner := runworker.New(adapter)

	result, err := runner.Execute(context.Background(), validRequest(), recorded)
	if err != nil {
		t.Fatalf("execute run: %v", err)
	}
	if result.FinalMessage != "done" {
		t.Fatalf("final message: got %q, want done", result.FinalMessage)
	}
	if len(recorded.events) != 2 {
		t.Fatalf("event count: got %d, want 2", len(recorded.events))
	}
}

func TestRunnerClassifiesEventSinkFailure(t *testing.T) {
	sinkErr := errors.New("event store unavailable")
	adapter := &runtimefake.Adapter{
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			err := events.Publish(ctx, agentruntime.Event{
				RunID:      request.RunID,
				Sequence:   1,
				Kind:       agentruntime.EventRuntimeStarted,
				OccurredAt: time.Now(),
			})
			return agentruntime.Result{}, err
		},
	}
	runner := runworker.New(adapter)

	_, err := runner.Execute(context.Background(), validRequest(), failingSink{err: sinkErr})
	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorEventDeliveryFailed {
		t.Fatalf("error code: got %q, want %q", got, agentruntime.ErrorEventDeliveryFailed)
	}
	if !errors.Is(err, sinkErr) {
		t.Fatal("event delivery error did not preserve its cause")
	}
}

func TestRunnerRejectsEventsAfterRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	adapter := &runtimefake.Adapter{
		ExecuteFunc: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			err := events.Publish(context.Background(), agentruntime.Event{
				RunID:      request.RunID,
				Sequence:   1,
				Kind:       agentruntime.EventRuntimeStarted,
				OccurredAt: time.Now(),
			})
			return agentruntime.Result{}, err
		},
	}
	recorded := &recordingSink{}
	runner := runworker.New(adapter)

	_, err := runner.Execute(ctx, validRequest(), recorded)
	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorInterrupted {
		t.Fatalf("error code: got %q, want %q", got, agentruntime.ErrorInterrupted)
	}
	if len(recorded.events) != 0 {
		t.Fatal("adapter published an event after run cancellation")
	}
}

func TestRunnerPreservesStreamingRuntimeFailure(t *testing.T) {
	modelErr := errors.New("model endpoint failed")
	adapter := &runtimefake.Adapter{
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			now := time.Now()
			for _, event := range []agentruntime.Event{
				{RunID: request.RunID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: now},
				{RunID: request.RunID, Sequence: 2, Kind: agentruntime.EventMessageDelta, OccurredAt: now, Payload: []byte(`{"delta":"working"}`)},
				{RunID: request.RunID, Sequence: 3, Kind: agentruntime.EventRuntimeFailed, OccurredAt: now},
			} {
				if err := events.Publish(ctx, event); err != nil {
					return agentruntime.Result{}, err
				}
			}
			return agentruntime.Result{}, &agentruntime.Error{
				Code:    agentruntime.ErrorModelFailed,
				Message: "execute model turn",
				Cause:   modelErr,
			}
		},
	}
	recorded := &recordingSink{}
	runner := runworker.New(adapter)

	_, err := runner.Execute(context.Background(), validRequest(), recorded)
	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorModelFailed {
		t.Fatalf("error code: got %q, want %q", got, agentruntime.ErrorModelFailed)
	}
	if !errors.Is(err, modelErr) {
		t.Fatal("runtime failure did not preserve its cause")
	}
	if len(recorded.events) != 3 || recorded.events[1].Kind != agentruntime.EventMessageDelta {
		t.Fatal("streaming events were not preserved before runtime failure")
	}
}

func TestRunnerRejectsInvalidRequestBeforeCallingAdapter(t *testing.T) {
	adapter := &runtimefake.Adapter{
		ExecuteFunc: func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error) {
			return agentruntime.Result{}, nil
		},
	}
	runner := runworker.New(adapter)

	_, err := runner.Execute(context.Background(), agentruntime.ExecuteRequest{}, &recordingSink{})
	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorInvalidConfiguration {
		t.Fatalf("error code: got %q, want %q", got, agentruntime.ErrorInvalidConfiguration)
	}
	if len(adapter.Requests()) != 0 {
		t.Fatal("invalid request reached the runtime adapter")
	}
}

type recordingSink struct {
	events []agentruntime.Event
}

func (s *recordingSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.events = append(s.events, event)
	return nil
}

type failingSink struct {
	err error
}

func (s failingSink) Publish(context.Context, agentruntime.Event) error {
	return s.err
}

func validRequest() agentruntime.ExecuteRequest {
	return agentruntime.ExecuteRequest{
		RunID:          "run-1",
		WorkspacePath:  "/workspace",
		Instruction:    "fix the failing test",
		Model:          "configured-model",
		EnvironmentRef: "environment-1",
	}
}
