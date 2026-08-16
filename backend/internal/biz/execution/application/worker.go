package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"
)

type RunProcessor interface {
	Execute(context.Context, domain.Lease) (domain.Outcome, error)
}

type WorkerConfig struct {
	WorkerID      string
	LeaseDuration time.Duration
	RenewInterval time.Duration
}

type Worker struct {
	runs      *Service
	processor RunProcessor
	config    WorkerConfig
}

func NewWorker(runs *Service, processor RunProcessor, config WorkerConfig) (*Worker, error) {
	if runs == nil || processor == nil {
		return nil, fmt.Errorf("Run service and processor are required")
	}
	if strings.TrimSpace(config.WorkerID) == "" {
		return nil, fmt.Errorf("Worker ID is required")
	}
	if config.LeaseDuration <= 0 || config.RenewInterval <= 0 || config.RenewInterval >= config.LeaseDuration {
		return nil, fmt.Errorf("lease duration must exceed the positive renew interval")
	}
	return &Worker{runs: runs, processor: processor, config: config}, nil
}

// ProcessNext claims and executes at most one Run. A lost lease cancels the
// processor and leaves terminal ownership to the lease reconciler.
func (worker *Worker) ProcessNext(ctx context.Context) (bool, error) {
	lease, found, err := worker.runs.Claim(ctx, worker.config.WorkerID, worker.config.LeaseDuration)
	if err != nil || !found {
		return found, err
	}
	if err := worker.runs.MarkRunning(ctx, lease.Token); err != nil {
		return true, fmt.Errorf("mark claimed Run running: %w", err)
	}

	executionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type executionResult struct {
		outcome domain.Outcome
		err     error
	}
	completed := make(chan executionResult, 1)
	go func() {
		outcome, executeErr := worker.processor.Execute(executionCtx, lease)
		completed <- executionResult{outcome: outcome, err: executeErr}
	}()

	ticker := time.NewTicker(worker.config.RenewInterval)
	defer ticker.Stop()
	interruptRequested := false
	for {
		select {
		case <-ctx.Done():
			cancel()
			return true, ctx.Err()
		case result := <-completed:
			if interruptRequested {
				outcome := interruptedExecutionOutcome()
				if err := worker.runs.Finish(ctx, lease.Token, outcome); err != nil {
					return true, fmt.Errorf("acknowledge Run interruption: %w", err)
				}
				return true, nil
			}
			if result.err != nil {
				result.outcome = failedExecutionOutcome()
			}
			if err := worker.runs.Finish(ctx, lease.Token, result.outcome); err != nil {
				return true, fmt.Errorf("finish Run: %w", err)
			}
			return true, result.err
		case <-ticker.C:
			if err := worker.runs.Renew(ctx, lease.Token, worker.config.LeaseDuration); err != nil {
				if errors.Is(err, domain.ErrInterruptionRequested) {
					interruptRequested = true
					cancel()
					continue
				}
				cancel()
				return true, fmt.Errorf("renew Run lease: %w", err)
			}
		}
	}
}

func interruptedExecutionOutcome() domain.Outcome {
	payload, _ := json.Marshal(map[string]string{
		"code": "run_interrupted", "message": "Run execution was interrupted by request",
	})
	return domain.Outcome{State: domain.Cancelled, Error: payload, Usage: json.RawMessage(`{}`), Cost: "0"}
}

func failedExecutionOutcome() domain.Outcome {
	payload, _ := json.Marshal(map[string]string{
		"code": "runtime_execution_failed", "message": "runtime execution failed",
	})
	return domain.Outcome{State: domain.Failed, Error: payload, Usage: json.RawMessage(`{}`), Cost: "0"}
}

func IsLeaseLost(err error) bool {
	return errors.Is(err, domain.ErrLeaseLost)
}
