package runworker

import (
	"context"
	"errors"

	"agent-platform/backend/internal/agentruntime"
)

type Runner struct {
	adapter agentruntime.Adapter
}

func New(adapter agentruntime.Adapter) *Runner {
	return &Runner{adapter: adapter}
}

func (r *Runner) Execute(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
	if err := request.Validate(); err != nil {
		return agentruntime.Result{}, err
	}
	contractEvents := agentruntime.NewContractSink(ctx, request.RunID, events)
	result, executeErr := r.adapter.Execute(ctx, request, contractEvents)
	closeErr := contractEvents.Close()
	return result, errors.Join(executeErr, closeErr)
}
