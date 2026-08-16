package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/webhook/domain"
)

type repositoryStub struct {
	delivery domain.Delivery
	found    bool
	state    domain.State
	next     time.Time
}

func (repository *repositoryStub) Claim(context.Context, time.Time, time.Duration) (domain.Delivery, bool, error) {
	return repository.delivery, repository.found, nil
}
func (repository *repositoryStub) MarkDelivered(_ context.Context, _ string, _ time.Time) error {
	repository.state = domain.StateDelivered
	return nil
}
func (repository *repositoryStub) MarkFailed(_ context.Context, _ string, _ string, next time.Time, cancel bool) error {
	repository.next = next
	repository.state = domain.StateFailed
	if cancel {
		repository.state = domain.StateCancelled
	}
	return nil
}

type delivererStub struct{ err error }

func (deliverer delivererStub) Deliver(context.Context, domain.Delivery) error { return deliverer.err }

func TestDispatcherMarksSuccessfulDelivery(t *testing.T) {
	repository := &repositoryStub{delivery: validDelivery(1), found: true}
	dispatcher := newTestDispatcher(t, repository, delivererStub{})
	found, err := dispatcher.ProcessNext(context.Background())
	if err != nil || !found || repository.state != domain.StateDelivered {
		t.Fatalf("ProcessNext() = found %v, state %q, error %v", found, repository.state, err)
	}
}

func TestDispatcherRetriesAndEventuallyCancels(t *testing.T) {
	now := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		attempt int
		state   domain.State
	}{
		{attempt: 1, state: domain.StateFailed},
		{attempt: 3, state: domain.StateCancelled},
	} {
		repository := &repositoryStub{delivery: validDelivery(test.attempt), found: true}
		dispatcher := newTestDispatcher(t, repository, delivererStub{err: errors.New("endpoint unavailable")})
		dispatcher.now = func() time.Time { return now }
		if _, err := dispatcher.ProcessNext(context.Background()); err != nil {
			t.Fatal(err)
		}
		if repository.state != test.state {
			t.Fatalf("attempt %d state = %q, want %q", test.attempt, repository.state, test.state)
		}
	}
}

func newTestDispatcher(t *testing.T, repository Repository, deliverer Deliverer) *Dispatcher {
	t.Helper()
	dispatcher, err := NewDispatcher(repository, deliverer, Config{
		LeaseDuration: time.Minute, RetryBase: time.Second, RetryMaximum: time.Minute, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	return dispatcher
}

func validDelivery(attempt int) domain.Delivery {
	return domain.Delivery{
		ID: "delivery", OrganizationID: "organization", EventType: "run.completed",
		Payload: json.RawMessage(`{"run_id":"run"}`), TargetURL: "https://hooks.example.test/agent-platform",
		AttemptCount: attempt,
	}
}
