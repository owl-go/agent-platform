package runtimeexecutor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTeamActiveTimePausesOnlyWhenAllInvocationsAwaitApproval(t *testing.T) {
	ctx, clock := newTeamActiveClock(context.Background(), time.Hour)
	defer clock.close()
	now := clock.last
	clock.now = func() time.Time { return now }
	clock.change("one", "begin")
	clock.change("two", "begin")
	now = now.Add(10 * time.Minute)
	clock.change("one", "wait")
	if !clock.running || clock.remaining != 50*time.Minute {
		t.Fatal("one waiting invocation paused a running member")
	}
	now = now.Add(10 * time.Minute)
	clock.change("two", "wait")
	if clock.running || clock.remaining != 40*time.Minute {
		t.Fatal("all waiting calls did not pause active time")
	}
	now = now.Add(2 * time.Hour)
	clock.change("one", "resume")
	if !clock.running || clock.remaining != 40*time.Minute || ctx.Err() != nil {
		t.Fatal("approval wait consumed or reset active budget")
	}
	now = now.Add(10 * time.Minute)
	clock.change("one", "end")
	if clock.running || clock.remaining != 30*time.Minute {
		t.Fatal("finished member prevented remaining approval wait")
	}
	now = now.Add(time.Hour)
	clock.change("two", "resume")
	if clock.remaining != 30*time.Minute {
		t.Fatal("last member approval wait consumed time")
	}
	now = now.Add(30 * time.Minute)
	clock.change("two", "wait")
	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			t.Fatal(context.Cause(ctx))
		}
	case <-time.After(time.Second):
		t.Fatal("approval request escaped an exhausted active budget")
	}
}
