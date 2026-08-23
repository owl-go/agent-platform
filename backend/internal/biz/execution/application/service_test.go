package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type repositoryStub struct {
	details       domain.Details
	getRunID      string
	finishOutcome domain.Outcome
	finishTime    time.Time
	event         domain.EventInput
}

func (repository *repositoryStub) Get(_ context.Context, runID string) (domain.Details, error) {
	repository.getRunID = runID
	return repository.details, nil
}

func (*repositoryStub) Claim(context.Context, string, time.Duration, time.Time) (domain.Lease, bool, error) {
	return domain.Lease{}, false, nil
}
func (*repositoryStub) Renew(context.Context, string, time.Duration, time.Time) error { return nil }
func (*repositoryStub) MarkRunning(context.Context, string, time.Time) error          { return nil }
func (repository *repositoryStub) AppendEvent(_ context.Context, _ string, event domain.EventInput, _ time.Time) error {
	repository.event = event
	return nil
}
func (repository *repositoryStub) FinishOwned(_ context.Context, _ string, outcome domain.Outcome, now time.Time) (domain.CompletionProjection, error) {
	repository.finishOutcome = outcome
	repository.finishTime = now
	return domain.CompletionProjection{}, nil
}
func (*repositoryStub) ReconcileExpired(context.Context, int, time.Time) (domain.ReconcileResult, error) {
	return domain.ReconcileResult{}, nil
}
func (*repositoryStub) ListEventsAfter(context.Context, string, int64, int) ([]domain.Event, error) {
	return nil, nil
}
func (repository *repositoryStub) Control(context.Context, string, int64, domain.ControlAction, string, time.Time) (domain.Details, error) {
	return repository.details, nil
}

func TestGetValidatesRunID(t *testing.T) {
	repository := &repositoryStub{details: domain.Details{ID: "run-1"}}
	service := New(repository)
	if _, err := service.Get(context.Background(), " "); err == nil {
		t.Fatal("Get accepted an empty Run ID")
	}
	details, err := service.Get(context.Background(), "run-1")
	if err != nil || details.ID != "run-1" || repository.getRunID != "run-1" {
		t.Fatalf("Get() = (%+v, %v), repository Run ID = %q", details, err, repository.getRunID)
	}
}

func TestFinishValidatesAndNormalizesBeforeRepository(t *testing.T) {
	now := time.Now().UTC()
	repository := &repositoryStub{}
	service := NewWithClock(repository, fixedClock(now))

	if err := service.Finish(context.Background(), "token", domain.Outcome{State: domain.Completed}); err != nil {
		t.Fatal(err)
	}
	if string(repository.finishOutcome.Usage) != `{}` || repository.finishOutcome.Cost != "0" || !repository.finishTime.Equal(now) {
		t.Fatalf("repository call = (%+v, %s)", repository.finishOutcome, repository.finishTime)
	}

	for _, outcome := range []domain.Outcome{
		{State: domain.Running},
		{State: domain.Completed, Usage: json.RawMessage(`{}`), Cost: "-1"},
		{State: domain.Completed, Usage: json.RawMessage(`{}`), Cost: "NaN"},
	} {
		if err := service.Finish(context.Background(), "token", outcome); err == nil {
			t.Fatalf("Finish accepted invalid outcome %+v", outcome)
		}
	}
}

func TestAppendEventAcceptsTheRuntimeContractAllowlist(t *testing.T) {
	repository := &repositoryStub{}
	service := New(repository)
	if err := service.AppendEvent(context.Background(), "lease", domain.EventInput{Type: "message.completed", Payload: json.RawMessage(`{"message":"done"}`)}); err != nil {
		t.Fatal(err)
	}
	if repository.event.Type != "message.completed" {
		t.Fatalf("persisted Event = %+v", repository.event)
	}
	if err := service.AppendEvent(context.Background(), "lease", domain.EventInput{Type: "runtime.untrusted", Payload: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("AppendEvent accepted an event outside the Runtime Contract")
	}
}
