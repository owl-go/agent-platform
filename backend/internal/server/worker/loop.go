package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type ProcessFunc func(context.Context) (bool, error)

func FatalAfterConsecutiveFailures(process ProcessFunc, maximum int) ProcessFunc {
	var failures int
	return func(ctx context.Context) (bool, error) {
		found, err := process(ctx)
		if err == nil || ctx.Err() != nil {
			failures = 0
			return found, err
		}
		failures++
		if failures >= maximum {
			return found, Fatal(fmt.Errorf("processor failed %d consecutive times: %w", failures, err))
		}
		return found, err
	}
}

type Loop struct {
	name     string
	interval time.Duration
	process  ProcessFunc
	state    *State

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewLoop(name string, interval time.Duration, process ProcessFunc) (*Loop, error) {
	return NewLoopWithState(name, interval, process, NewState())
}

func NewLoopWithState(name string, interval time.Duration, process ProcessFunc, state *State) (*Loop, error) {
	if name == "" || interval <= 0 || process == nil {
		return nil, fmt.Errorf("Worker loop name, positive interval, and Processor are required")
	}
	if state == nil {
		return nil, fmt.Errorf("Worker loop State is required")
	}
	if err := state.register(name); err != nil {
		return nil, err
	}
	return &Loop{name: name, interval: interval, process: process, state: state}, nil
}

func (server *Loop) Start(parent context.Context) error {
	server.mu.Lock()
	if server.done != nil {
		server.mu.Unlock()
		return fmt.Errorf("Worker loop %s already started", server.name)
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	server.cancel = cancel
	server.done = done
	server.mu.Unlock()
	defer close(done)
	server.state.markStarted(server.name)
	defer server.state.markStopped(server.name)

	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			found, err := server.process(ctx)
			if IsFatal(err) && ctx.Err() == nil {
				server.state.markFatal(server.name, err)
				return fmt.Errorf("Worker loop %s terminated: %w", server.name, err)
			}
			if err != nil && ctx.Err() == nil {
				slog.Error("Worker item failed", "server", server.name, "error", err)
			}
			delay := server.interval
			if found && err == nil {
				delay = 0
			}
			timer.Reset(delay)
		}
	}
}

func (server *Loop) Stop(ctx context.Context) error {
	server.mu.Lock()
	cancel, done := server.cancel, server.done
	server.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
