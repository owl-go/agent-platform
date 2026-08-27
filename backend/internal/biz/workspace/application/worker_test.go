package application

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type cancellationRepository struct {
	claimed   atomic.Bool
	requested atomic.Bool
	finished  chan struct{}
}

func (repository *cancellationRepository) ClaimNext(context.Context) (*ExecutionJob, error) {
	if repository.claimed.Swap(true) {
		return nil, nil
	}
	return &ExecutionJob{Kind: JobSession, ID: "session-1-2", OwnerID: "owner-1", SessionID: "session-1", AssistantMessageID: 2}, nil
}

func (*cancellationRepository) FinishSucceeded(context.Context, ExecutionJob, ExecutionResult) error {
	return nil
}

func (*cancellationRepository) FinishFailed(context.Context, ExecutionJob, string) error { return nil }

func (repository *cancellationRepository) FinishCancelled(context.Context, ExecutionJob) error {
	close(repository.finished)
	return nil
}

func (*cancellationRepository) FinishMCPTest(context.Context, ExecutionJob, string) error { return nil }

func (*cancellationRepository) RecordProgress(context.Context, ExecutionJob, ExecutionEvent) error {
	return nil
}

func (repository *cancellationRepository) CancellationRequested(context.Context, ExecutionJob) (bool, error) {
	return repository.requested.Load(), nil
}

type cancellationExecutor struct{ started chan struct{} }

func (executor *cancellationExecutor) Execute(ctx context.Context, _ ExecutionJob, _ ProgressRecorder) (ExecutionResult, error) {
	close(executor.started)
	<-ctx.Done()
	return ExecutionResult{}, ctx.Err()
}

func TestWorkerStopsActiveExecutionAfterCancellationRequest(t *testing.T) {
	repository := &cancellationRepository{finished: make(chan struct{})}
	executor := &cancellationExecutor{started: make(chan struct{})}
	worker, err := NewWorker(repository, executor)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, processErr := worker.ProcessNext(context.Background())
		done <- processErr
	}()

	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	repository.requested.Store(true)

	select {
	case <-repository.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not finish the Session message as cancelled")
	}
	if err := <-done; err != nil {
		t.Fatalf("ProcessNext: %v", err)
	}
}
