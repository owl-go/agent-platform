package runtimefake

import (
	"context"
	"fmt"
	"sync"

	"agent-platform/internal/agentruntime"
)

type Adapter struct {
	DescribeResult agentruntime.Descriptor
	DescribeErr    error
	ExecuteFunc    func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error)

	mu       sync.Mutex
	requests []agentruntime.ExecuteRequest
}

func (a *Adapter) Describe(context.Context) (agentruntime.Descriptor, error) {
	return a.DescribeResult, a.DescribeErr
}

func (a *Adapter) Execute(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
	a.mu.Lock()
	a.requests = append(a.requests, request)
	a.mu.Unlock()

	if a.ExecuteFunc == nil {
		return agentruntime.Result{}, fmt.Errorf("runtimefake: ExecuteFunc is required")
	}
	return a.ExecuteFunc(ctx, request, events)
}

func (a *Adapter) Requests() []agentruntime.ExecuteRequest {
	a.mu.Lock()
	defer a.mu.Unlock()

	requests := make([]agentruntime.ExecuteRequest, len(a.requests))
	copy(requests, a.requests)
	return requests
}

var _ agentruntime.Adapter = (*Adapter)(nil)
