package runtimeexecutor

import (
	"context"
	"sync"
	"time"

	"agent-platform/backend/internal/cliconnector"
)

// teamActiveClock counts setup, execution and merge time, pausing only when
// every outstanding invocation is blocked on a command approval.
type teamActiveClock struct {
	mu         sync.Mutex
	remaining  time.Duration
	last       time.Time
	running    bool
	active     map[string]int
	timer      *time.Timer
	generation uint64
	closed     bool
	cancel     context.CancelCauseFunc
	now        func() time.Time
}

func newTeamActiveClock(ctx context.Context, duration time.Duration) (context.Context, *teamActiveClock) {
	ctx, cancel := context.WithCancelCause(ctx)
	clock := &teamActiveClock{remaining: duration, last: time.Now(), running: true, active: map[string]int{}, cancel: cancel, now: time.Now}
	clock.schedule()
	return ctx, clock
}

func (clock *teamActiveClock) schedule() {
	clock.generation++
	if clock.timer != nil {
		clock.timer.Stop()
	}
	if !clock.closed && clock.remaining <= 0 {
		clock.cancel(context.DeadlineExceeded)
		return
	}
	if clock.closed || !clock.running {
		return
	}
	generation := clock.generation
	clock.timer = time.AfterFunc(max(time.Duration(0), clock.remaining), func() {
		clock.mu.Lock()
		defer clock.mu.Unlock()
		if !clock.closed && generation == clock.generation {
			clock.cancel(context.DeadlineExceeded)
		}
	})
}

func (clock *teamActiveClock) change(id string, operation string) {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	if clock.closed {
		return
	}
	now := clock.now()
	if clock.running {
		clock.remaining -= now.Sub(clock.last)
	}
	clock.last = now
	switch operation {
	case "begin":
		clock.active[id] = 0
	case "end":
		delete(clock.active, id)
	case "wait":
		if _, ok := clock.active[id]; ok {
			clock.active[id]++
		}
	case "resume":
		if value, ok := clock.active[id]; ok && value > 0 {
			clock.active[id]--
		}
	}
	clock.running = len(clock.active) == 0
	for _, waiting := range clock.active {
		if waiting == 0 {
			clock.running = true
			break
		}
	}
	clock.schedule()
}

func (clock *teamActiveClock) close() {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	clock.closed = true
	clock.generation++
	if clock.timer != nil {
		clock.timer.Stop()
	}
	clock.cancel(context.Canceled)
}

type invocationClockKey struct{}
type invocationClock struct {
	clock *teamActiveClock
	id    string
}
type timedTeamApproval struct {
	cliconnector.ApprovalCoordinator
	invocation invocationClock
}

func (approval timedTeamApproval) Await(ctx context.Context, request cliconnector.ApprovalRequest) (cliconnector.ApprovalGrant, error) {
	approval.invocation.clock.change(approval.invocation.id, "wait")
	defer approval.invocation.clock.change(approval.invocation.id, "resume")
	return approval.ApprovalCoordinator.Await(ctx, request)
}

func teamApprovalCoordinator(ctx context.Context, coordinator cliconnector.ApprovalCoordinator) cliconnector.ApprovalCoordinator {
	if invocation, ok := ctx.Value(invocationClockKey{}).(invocationClock); ok && coordinator != nil {
		return timedTeamApproval{coordinator, invocation}
	}
	return coordinator
}
