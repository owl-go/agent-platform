package worker_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	workerserver "agent-platform/backend/internal/server/worker"
)

func TestLoopServerStopsAtASafeBoundary(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server, err := workerserver.NewLoop("execution", time.Millisecond, func(context.Context) (bool, error) {
		calls.Add(1)
		entered <- struct{}{}
		<-release
		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	startDone := make(chan error, 1)
	go func() { startDone <- server.Start(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("Worker loop did not process an item")
	}

	stopDone := make(chan error, 1)
	go func() { stopDone <- server.Stop(context.Background()) }()
	select {
	case err := <-stopDone:
		t.Fatalf("Stop returned before the current safe boundary: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-stopDone; err != nil {
		t.Fatal(err)
	}
	if err := <-startDone; err != nil {
		t.Fatal(err)
	}
	if err := server.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("process calls = %d, want 1", calls.Load())
	}
}

func TestLoopServerTerminatesAndMarksReadinessOnFatalError(t *testing.T) {
	state := workerserver.NewState()
	server, err := workerserver.NewLoopWithState("execution", time.Millisecond, func(context.Context) (bool, error) {
		return false, workerserver.Fatal(errors.New("processor configuration lost"))
	}, state)
	if err != nil {
		t.Fatal(err)
	}
	err = server.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "processor configuration lost") {
		t.Fatalf("Start() error = %v", err)
	}
	if state.Ready() {
		t.Fatal("Worker State remained ready after fatal loop failure")
	}
	statuses := state.Statuses()
	if len(statuses) != 1 || !statuses[0].Fatal || statuses[0].Started {
		t.Fatalf("statuses = %+v", statuses)
	}
}

func TestLoopServerRetriesRecoverableItemError(t *testing.T) {
	var calls atomic.Int64
	state := workerserver.NewState()
	server, err := workerserver.NewLoopWithState("webhook", time.Millisecond, func(context.Context) (bool, error) {
		if calls.Add(1) == 1 {
			return false, errors.New("single delivery failed")
		}
		return false, workerserver.Fatal(errors.New("stop test"))
	}, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(context.Background()); err == nil {
		t.Fatal("Start() returned nil after terminal test error")
	}
	if calls.Load() != 2 {
		t.Fatalf("process calls = %d, want recoverable retry", calls.Load())
	}
}

func TestFailurePolicyEscalatesOnlyConsecutiveFailures(t *testing.T) {
	calls := 0
	process := workerserver.FatalAfterConsecutiveFailures(func(context.Context) (bool, error) {
		calls++
		if calls == 2 {
			return false, nil
		}
		return false, errors.New("database unavailable")
	}, 2)
	if _, err := process(context.Background()); err == nil || workerserver.IsFatal(err) {
		t.Fatalf("first failure = %v, want recoverable", err)
	}
	if _, err := process(context.Background()); err != nil {
		t.Fatalf("successful item did not reset failures: %v", err)
	}
	if _, err := process(context.Background()); err == nil || workerserver.IsFatal(err) {
		t.Fatalf("failure after reset = %v, want recoverable", err)
	}
	if _, err := process(context.Background()); err == nil || !workerserver.IsFatal(err) {
		t.Fatalf("second consecutive failure = %v, want fatal", err)
	}
}
