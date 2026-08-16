package runtimeprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/agentruntime"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/runworker"
	"agent-platform/backend/internal/sandbox"
)

type CredentialBinding = credentials.Binding
type SecretResolver = credentials.Resolver

type RuntimeFactory interface {
	New(domain.Lease, Plan, *credentials.Environment) (agentruntime.Adapter, error)
}

type Plan struct {
	ModelID      string
	Budget       ModelBudget
	Timeout      time.Duration
	Egress       sandbox.EgressMode
	Limits       sandbox.Limits
	Capabilities map[agentruntime.Capability]bool
}

type Processor struct {
	runs         *executionapplication.Service
	resolver     SecretResolver
	materializer credentials.Materializer
	factory      RuntimeFactory
}

func New(runs *executionapplication.Service, resolver SecretResolver, materializer credentials.Materializer, factory RuntimeFactory) (*Processor, error) {
	if runs == nil || resolver == nil || factory == nil {
		return nil, fmt.Errorf("Run service, Secret Resolver, and Runtime Factory are required")
	}
	return &Processor{runs: runs, resolver: resolver, materializer: materializer, factory: factory}, nil
}

func (processor *Processor) Execute(ctx context.Context, lease domain.Lease) (outcome domain.Outcome, returnErr error) {
	plan, bindings, err := ParsePlan(lease)
	if err != nil {
		return domain.Outcome{}, err
	}
	request, err := processor.resolver.Resolve(ctx, lease.RunID, bindings)
	if err != nil {
		return domain.Outcome{}, fmt.Errorf("resolve Run credentials: %w", err)
	}
	environment, err := processor.materializer.Create(request)
	if err != nil {
		return domain.Outcome{}, err
	}
	defer func() {
		if cleanupErr := environment.Cleanup(); cleanupErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("clean Run credential environment: %w", cleanupErr))
		}
	}()

	adapter, err := processor.factory.New(lease, plan, environment)
	if err != nil {
		return domain.Outcome{}, err
	}
	executionCtx, cancel := context.WithTimeout(ctx, plan.Timeout)
	defer cancel()
	descriptor, err := adapter.Describe(executionCtx)
	if err != nil {
		return domain.Outcome{}, redactError(environment.Redactor(), err)
	}
	if descriptor.Name != lease.RuntimeName || descriptor.Version != lease.RuntimeCLIVersion {
		return domain.Outcome{}, fmt.Errorf("Runtime Image descriptor does not match frozen Run binding")
	}
	events := agentruntime.NewRedactingEventSink(environment.Redactor(), &eventSink{
		runs: processor.runs, leaseToken: lease.Token, runID: lease.RunID,
	})
	result, err := runworker.New(adapter).Execute(executionCtx, agentruntime.ExecuteRequest{
		RunID: lease.RunID, WorkspacePath: "/workspace", Instruction: lease.RequestText,
		Model: plan.ModelID, EnvironmentRef: request.Ref,
	}, events)
	if err != nil {
		return domain.Outcome{}, redactError(environment.Redactor(), err)
	}
	if result.Usage.InputTokens < 0 || result.Usage.OutputTokens < 0 || result.Usage.CostMicros < 0 {
		return domain.Outcome{}, fmt.Errorf("Runtime returned invalid negative Usage")
	}
	usage, err := json.Marshal(map[string]int64{
		"input_tokens": result.Usage.InputTokens, "output_tokens": result.Usage.OutputTokens,
		"cost_micros": result.Usage.CostMicros,
	})
	if err != nil {
		return domain.Outcome{}, err
	}
	cost := dollars(result.Usage.CostMicros)
	if reason := plan.Budget.ExceededBy(result.Usage); reason != "" {
		terminalError, marshalErr := json.Marshal(map[string]string{
			"code": "model_budget_exceeded", "message": reason,
		})
		if marshalErr != nil {
			return domain.Outcome{}, marshalErr
		}
		return domain.Outcome{State: domain.Failed, Error: terminalError, Usage: usage, Cost: cost}, nil
	}
	return domain.Outcome{State: domain.Completed, Usage: usage, Cost: cost}, nil
}

type eventSink struct {
	runs       *executionapplication.Service
	leaseToken string
	runID      string
}

func (sink *eventSink) Publish(ctx context.Context, event agentruntime.Event) error {
	if event.RunID != sink.runID {
		return fmt.Errorf("Runtime Event Run ID does not match lease")
	}
	payload, err := json.Marshal(map[string]any{
		"runtime_sequence": event.Sequence,
		"occurred_at":      event.OccurredAt.UTC(),
		"payload":          json.RawMessage(event.Payload),
	})
	if err != nil {
		return err
	}
	return sink.runs.AppendEvent(ctx, sink.leaseToken, domain.EventInput{Type: string(event.Kind), Payload: payload})
}

func redactError(redactor *credentials.Redactor, err error) error {
	if err == nil {
		return nil
	}
	return errors.New(string(redactor.Bytes([]byte(err.Error()))))
}

func dollars(micros int64) string {
	if micros == 0 {
		return "0"
	}
	return fmt.Sprintf("%d.%06d", micros/1_000_000, micros%1_000_000)
}
