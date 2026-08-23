package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"
)

type workerRepository struct {
	mu            sync.Mutex
	lease         domain.Lease
	found         bool
	renewError    error
	markedRunning bool
	renewals      int
	finished      *domain.Outcome
	details       domain.Details
}

func (repository *workerRepository) Get(context.Context, string) (domain.Details, error) {
	if repository.details.ID == "" {
		return domain.Details{}, domain.ErrRunNotFound
	}
	return repository.details, nil
}
func (repository *workerRepository) Claim(context.Context, string, time.Duration, time.Time) (domain.Lease, bool, error) {
	return repository.lease, repository.found, nil
}
func (repository *workerRepository) Renew(context.Context, string, time.Duration, time.Time) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.renewals++
	return repository.renewError
}
func (repository *workerRepository) MarkRunning(context.Context, string, time.Time) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.markedRunning = true
	return nil
}
func (*workerRepository) AppendEvent(context.Context, string, domain.EventInput, time.Time) error {
	return nil
}
func (repository *workerRepository) FinishOwned(_ context.Context, _ string, outcome domain.Outcome, _ time.Time) (domain.CompletionProjection, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	repository.finished = &outcome
	return domain.CompletionProjection{}, nil
}
func (*workerRepository) ReconcileExpired(context.Context, int, time.Time) (domain.ReconcileResult, []domain.CompletionProjection, error) {
	return domain.ReconcileResult{}, nil, nil
}
func (*workerRepository) ListEventsAfter(context.Context, string, int64, int) ([]domain.Event, error) {
	return nil, nil
}
func (*workerRepository) Control(context.Context, string, int64, domain.ControlAction, string, time.Time) (domain.Details, domain.CompletionProjection, error) {
	return domain.Details{}, domain.CompletionProjection{}, nil
}

type processorFunc func(context.Context, domain.Lease) (domain.Outcome, error)

func (function processorFunc) Execute(ctx context.Context, lease domain.Lease) (domain.Outcome, error) {
	return function(ctx, lease)
}

func TestWorkerProcessesClaimedRun(t *testing.T) {
	repository := &workerRepository{found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}}
	processor := processorFunc(func(_ context.Context, lease domain.Lease) (domain.Outcome, error) {
		if lease.RunID != "run-1" {
			t.Fatalf("processor lease = %+v", lease)
		}
		return domain.Outcome{State: domain.Completed}, nil
	})
	worker, err := NewWorker(New(repository), processor, WorkerConfig{
		WorkerID: "worker-a", LeaseDuration: time.Second, RenewInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	found, err := worker.ProcessNext(context.Background())
	if err != nil || !found {
		t.Fatalf("ProcessNext() = (%v, %v)", found, err)
	}
	if !repository.markedRunning || repository.finished == nil || repository.finished.State != domain.Completed {
		t.Fatalf("repository state: marked=%v outcome=%+v", repository.markedRunning, repository.finished)
	}
	if string(repository.finished.Usage) != `{}` || repository.finished.Cost != "0" {
		t.Fatalf("normalized outcome = %+v", repository.finished)
	}
}

func TestWorkerConvertsProcessorFailureToSafeOutcome(t *testing.T) {
	repository := &workerRepository{found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}}
	processorError := errors.New("secret-bearing runtime details")
	worker, err := NewWorker(New(repository), processorFunc(func(context.Context, domain.Lease) (domain.Outcome, error) {
		return domain.Outcome{}, processorError
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: time.Second, RenewInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, gotErr := worker.ProcessNext(context.Background())
	if !found || !errors.Is(gotErr, processorError) {
		t.Fatalf("ProcessNext() = (%v, %v)", found, gotErr)
	}
	if repository.finished == nil || repository.finished.State != domain.Failed || string(repository.finished.Error) != `{"code":"runtime_execution_failed","message":"runtime execution failed"}` {
		t.Fatalf("failure outcome = %+v", repository.finished)
	}
}

func TestWorkerLeavesRejectedApprovalTerminalStateUntouched(t *testing.T) {
	repository := &workerRepository{found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}}
	worker, err := NewWorker(New(repository), processorFunc(func(context.Context, domain.Lease) (domain.Outcome, error) {
		return domain.Outcome{}, domain.ErrApprovalRejected
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: time.Second, RenewInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, gotErr := worker.ProcessNext(context.Background())
	if !found || gotErr != nil || repository.finished != nil {
		t.Fatalf("ProcessNext() = (%v, %v), outcome=%+v", found, gotErr, repository.finished)
	}
}

func TestWorkerTreatsApprovalRejectionLeaseReleaseAsCompleted(t *testing.T) {
	repository := &workerRepository{
		found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}, renewError: domain.ErrLeaseLost,
		details: domain.Details{ID: "run-1", State: domain.Failed},
	}
	cancelled := make(chan struct{})
	worker, err := NewWorker(New(repository), processorFunc(func(ctx context.Context, _ domain.Lease) (domain.Outcome, error) {
		<-ctx.Done()
		close(cancelled)
		return domain.Outcome{}, ctx.Err()
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: 50 * time.Millisecond, RenewInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, gotErr := worker.ProcessNext(context.Background())
	if !found || gotErr != nil || repository.finished != nil {
		t.Fatalf("ProcessNext() = (%v, %v), outcome=%+v", found, gotErr, repository.finished)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("processor was not cancelled after Approval released its lease")
	}
}

func TestWorkerCancelsProcessorWhenLeaseIsLost(t *testing.T) {
	repository := &workerRepository{
		found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}, renewError: domain.ErrLeaseLost,
	}
	cancelled := make(chan struct{})
	worker, err := NewWorker(New(repository), processorFunc(func(ctx context.Context, _ domain.Lease) (domain.Outcome, error) {
		<-ctx.Done()
		close(cancelled)
		return domain.Outcome{}, ctx.Err()
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: 50 * time.Millisecond, RenewInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, gotErr := worker.ProcessNext(context.Background())
	if !found || !IsLeaseLost(gotErr) {
		t.Fatalf("ProcessNext() = (%v, %v)", found, gotErr)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("processor was not cancelled")
	}
	if repository.finished != nil {
		t.Fatalf("lost lease must not finish Run: %+v", repository.finished)
	}
}

func TestWorkerAcknowledgesRequestedInterruption(t *testing.T) {
	repository := &workerRepository{
		found: true, lease: domain.Lease{RunID: "run-1", Token: "lease-1"}, renewError: domain.ErrInterruptionRequested,
	}
	cancelled := make(chan struct{})
	worker, err := NewWorker(New(repository), processorFunc(func(ctx context.Context, _ domain.Lease) (domain.Outcome, error) {
		<-ctx.Done()
		close(cancelled)
		return domain.Outcome{}, ctx.Err()
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: 50 * time.Millisecond, RenewInterval: 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, gotErr := worker.ProcessNext(context.Background())
	if !found || gotErr != nil {
		t.Fatalf("ProcessNext() = (%v, %v)", found, gotErr)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("processor was not cancelled")
	}
	if repository.finished == nil || repository.finished.State != domain.Cancelled || !strings.Contains(string(repository.finished.Error), "run_interrupted") {
		t.Fatalf("interruption outcome = %+v", repository.finished)
	}
}

func TestWorkerReturnsWhenQueueIsEmpty(t *testing.T) {
	processorCalled := false
	worker, err := NewWorker(New(&workerRepository{}), processorFunc(func(context.Context, domain.Lease) (domain.Outcome, error) {
		processorCalled = true
		return domain.Outcome{}, nil
	}), WorkerConfig{WorkerID: "worker-a", LeaseDuration: time.Second, RenewInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	found, err := worker.ProcessNext(context.Background())
	if err != nil || found || processorCalled {
		t.Fatalf("ProcessNext() = (%v, %v), processor called=%v", found, err, processorCalled)
	}
}
