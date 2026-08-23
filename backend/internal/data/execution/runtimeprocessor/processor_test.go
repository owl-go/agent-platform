package runtimeprocessor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/runtimefake"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/data/execution/runtimeprocessor"
)

type repositoryStub struct {
	details domain.Details
	events  []domain.EventInput
}

func (repository *repositoryStub) Get(context.Context, string) (domain.Details, error) {
	if repository.details.ID == "" {
		return domain.Details{}, domain.ErrRunNotFound
	}
	return repository.details, nil
}
func (*repositoryStub) Claim(context.Context, string, time.Duration, time.Time) (domain.Lease, bool, error) {
	return domain.Lease{}, false, nil
}
func (*repositoryStub) Renew(context.Context, string, time.Duration, time.Time) error { return nil }
func (*repositoryStub) MarkRunning(context.Context, string, time.Time) error          { return nil }
func (*repositoryStub) FinishOwned(context.Context, string, domain.Outcome, time.Time) (domain.CompletionProjection, error) {
	return domain.CompletionProjection{}, nil
}
func (repository *repositoryStub) AppendEvent(_ context.Context, _ string, event domain.EventInput, _ time.Time) error {
	repository.events = append(repository.events, event)
	return nil
}
func (*repositoryStub) ReconcileExpired(context.Context, int, time.Time) (domain.ReconcileResult, error) {
	return domain.ReconcileResult{}, nil
}
func (*repositoryStub) ListEventsAfter(context.Context, string, int64, int) ([]domain.Event, error) {
	return nil, nil
}
func (*repositoryStub) Control(context.Context, string, int64, domain.ControlAction, string, time.Time) (domain.Details, error) {
	return domain.Details{}, nil
}

type resolverFunc func(context.Context, string, []runtimeprocessor.CredentialBinding) (credentials.Request, error)

func (function resolverFunc) Resolve(ctx context.Context, runID string, bindings []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
	return function(ctx, runID, bindings)
}

type factoryFunc func(domain.Lease, runtimeprocessor.Plan, *credentials.Environment) (agentruntime.Adapter, error)

func (function factoryFunc) New(lease domain.Lease, plan runtimeprocessor.Plan, environment *credentials.Environment) (agentruntime.Adapter, error) {
	return function(lease, plan, environment)
}

type approvalRequesterStub struct {
	mu        sync.Mutex
	requested chan struct{}
	approved  chan struct{}
	request   json.RawMessage
	sequence  int64
}

func (requester *approvalRequesterStub) AwaitDecision(ctx context.Context, request executionapplication.RuntimeApprovalRequest) error {
	if request.RunID != "run-1" || request.AttemptID != "attempt-1" || request.Kind != "high_risk_change" {
		return errors.New("unexpected Approval request identity")
	}
	requester.mu.Lock()
	requester.request = append(json.RawMessage(nil), request.Request...)
	requester.sequence = request.Sequence
	requester.mu.Unlock()
	close(requester.requested)
	select {
	case <-requester.approved:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type approvalGateFunc func(context.Context, executionapplication.RuntimeApprovalRequest) error

func (function approvalGateFunc) AwaitDecision(ctx context.Context, request executionapplication.RuntimeApprovalRequest) error {
	return function(ctx, request)
}

var allowApprovals = approvalGateFunc(func(context.Context, executionapplication.RuntimeApprovalRequest) error { return nil })

func TestProcessorExecutesFrozenPlanAndRedactsEvents(t *testing.T) {
	const secret = "super-secret-model-key"
	repository := &repositoryStub{}
	runs := executionapplication.New(repository)
	credentialDirectory := ""
	adapter := &runtimefake.Adapter{
		DescribeResult: agentruntime.Descriptor{Name: "claude", Version: "1.0.0"},
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if request.WorkspacePath != "/workspace" || request.Model != "model-id" || request.Instruction != "fix tests" {
				t.Fatalf("ExecuteRequest = %+v", request)
			}
			for sequence, event := range []agentruntime.Event{
				{RunID: request.RunID, Kind: agentruntime.EventRuntimeStarted, Payload: []byte(`{"message":"` + secret + `"}`)},
				{RunID: request.RunID, Kind: agentruntime.EventRuntimeCompleted, Payload: []byte(`{"ok":true}`)},
			} {
				event.Sequence = int64(sequence + 1)
				event.OccurredAt = time.Now().UTC()
				if err := events.Publish(ctx, event); err != nil {
					return agentruntime.Result{}, err
				}
			}
			return agentruntime.Result{Usage: agentruntime.Usage{InputTokens: 10, OutputTokens: 5, CostMicros: 1_234_567}}, nil
		},
	}
	processor, err := runtimeprocessor.New(
		runs,
		resolverFunc(func(_ context.Context, runID string, bindings []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
			if runID != "run-1" || len(bindings) != 1 || bindings[0].Ref != "vault://model" {
				t.Fatalf("Resolve(%q, %+v)", runID, bindings)
			}
			return credentials.Request{Ref: "run-1", Variables: map[string]string{"MODEL_API_KEY": secret}}, nil
		}),
		credentials.Materializer{Root: t.TempDir()},
		factoryFunc(func(_ domain.Lease, plan runtimeprocessor.Plan, environment *credentials.Environment) (agentruntime.Adapter, error) {
			if plan.Timeout != time.Minute || plan.Limits.CPUs != 2 {
				t.Fatalf("Plan = %+v", plan)
			}
			credentialDirectory = environment.Directory()
			return adapter, nil
		}),
		allowApprovals,
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := processor.Execute(context.Background(), validLease())
	if err != nil {
		t.Fatal(err)
	}
	if outcome.State != domain.Completed || outcome.Cost != "1.234567" || !strings.Contains(string(outcome.Usage), `"input_tokens":10`) {
		t.Fatalf("Outcome = %+v", outcome)
	}
	if len(repository.events) != 2 || strings.Contains(string(repository.events[0].Payload), secret) || !strings.Contains(string(repository.events[0].Payload), "[REDACTED]") {
		t.Fatalf("persisted Events = %+v", repository.events)
	}
	if _, err := os.Stat(credentialDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("credential directory still exists: %q error=%v", credentialDirectory, err)
	}
}

func TestProcessorPausesRuntimeApprovalEventUntilDecision(t *testing.T) {
	repository := &repositoryStub{}
	requester := &approvalRequesterStub{requested: make(chan struct{}), approved: make(chan struct{})}
	adapter := &runtimefake.Adapter{
		DescribeResult: agentruntime.Descriptor{Name: "claude", Version: "1.0.0"},
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			for sequence, event := range []agentruntime.Event{
				{RunID: request.RunID, Kind: agentruntime.EventRuntimeStarted, Payload: []byte(`{}`)},
				{RunID: request.RunID, Kind: agentruntime.EventApprovalRequested, Payload: []byte(`{"kind":"high_risk_change","request":{"risk_reason":"protected write"}}`)},
				{RunID: request.RunID, Kind: agentruntime.EventRuntimeCompleted, Payload: []byte(`{}`)},
			} {
				event.Sequence = int64(sequence + 1)
				event.OccurredAt = time.Now().UTC()
				if err := events.Publish(ctx, event); err != nil {
					return agentruntime.Result{}, err
				}
			}
			return agentruntime.Result{}, nil
		},
	}
	processor, err := runtimeprocessor.New(
		executionapplication.New(repository),
		resolverFunc(func(context.Context, string, []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
			return credentials.Request{Ref: "run-1", Variables: map[string]string{"MODEL_API_KEY": "secret-value"}}, nil
		}),
		credentials.Materializer{Root: t.TempDir()},
		factoryFunc(func(domain.Lease, runtimeprocessor.Plan, *credentials.Environment) (agentruntime.Adapter, error) {
			return adapter, nil
		}),
		requester,
	)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, executeErr := processor.Execute(context.Background(), validLease())
		done <- executeErr
	}()
	select {
	case <-requester.requested:
	case <-time.After(time.Second):
		t.Fatal("Runtime Approval was not requested")
	}
	select {
	case err := <-done:
		t.Fatalf("Runtime resumed before Approval decision: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	requester.mu.Lock()
	if requester.sequence != 2 || !strings.Contains(string(requester.request), "protected write") {
		t.Fatalf("Approval request sequence=%d body=%s", requester.sequence, requester.request)
	}
	requester.mu.Unlock()
	close(requester.approved)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Runtime did not resume after Approval")
	}
}

func TestProcessorPreservesRejectedApprovalCauseAfterRedaction(t *testing.T) {
	repository := &repositoryStub{}
	adapter := &runtimefake.Adapter{
		DescribeResult: agentruntime.Descriptor{Name: "claude", Version: "1.0.0"},
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if err := events.Publish(ctx, agentruntime.Event{RunID: request.RunID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte(`{}`)}); err != nil {
				return agentruntime.Result{}, err
			}
			if err := events.Publish(ctx, agentruntime.Event{RunID: request.RunID, Sequence: 2, Kind: agentruntime.EventApprovalRequested, OccurredAt: time.Now(), Payload: []byte(`{"kind":"high_risk_change","request":{"risk_reason":"protected write"}}`)}); err != nil {
				return agentruntime.Result{}, err
			}
			return agentruntime.Result{}, nil
		},
	}
	processor, err := runtimeprocessor.New(
		executionapplication.New(repository),
		resolverFunc(func(context.Context, string, []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
			return credentials.Request{Ref: "run-1", Variables: map[string]string{"MODEL_API_KEY": "secret-value"}}, nil
		}),
		credentials.Materializer{Root: t.TempDir()},
		factoryFunc(func(domain.Lease, runtimeprocessor.Plan, *credentials.Environment) (agentruntime.Adapter, error) {
			return adapter, nil
		}),
		approvalGateFunc(func(context.Context, executionapplication.RuntimeApprovalRequest) error {
			return fmt.Errorf("%w: secret-value", domain.ErrApprovalRejected)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = processor.Execute(context.Background(), validLease())
	if !errors.Is(err, domain.ErrApprovalRejected) || strings.Contains(err.Error(), "secret-value") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("Processor rejection error = %v", err)
	}
}

func TestParsePlanFailsClosed(t *testing.T) {
	lease := validLease()
	lease.ExecutionLimits = []byte(`{"timeout_seconds":60,"cpus":2,"memory_bytes":1024,"pids":10,"temp_bytes":1024,"egress":"public","unknown":true}`)
	if _, _, err := runtimeprocessor.ParsePlan(lease); err == nil {
		t.Fatal("ParsePlan accepted an unknown field")
	}
	lease = validLease()
	lease.CredentialBindings = []byte(`[]`)
	if _, _, err := runtimeprocessor.ParsePlan(lease); err == nil {
		t.Fatal("ParsePlan accepted no Credential Bindings")
	}
}

func TestProcessorFailsRunWhenFrozenModelBudgetIsExceeded(t *testing.T) {
	repository := &repositoryStub{}
	runs := executionapplication.New(repository)
	adapter := &runtimefake.Adapter{
		DescribeResult: agentruntime.Descriptor{Name: "claude", Version: "1.0.0"},
		ExecuteFunc: func(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			for sequence, kind := range []agentruntime.EventKind{agentruntime.EventRuntimeStarted, agentruntime.EventRuntimeCompleted} {
				if err := events.Publish(ctx, agentruntime.Event{RunID: request.RunID, Sequence: int64(sequence + 1), Kind: kind, OccurredAt: time.Now(), Payload: []byte(`{}`)}); err != nil {
					return agentruntime.Result{}, err
				}
			}
			return agentruntime.Result{Usage: agentruntime.Usage{InputTokens: 101, OutputTokens: 5, CostMicros: 250_000}}, nil
		},
	}
	processor, err := runtimeprocessor.New(
		runs,
		resolverFunc(func(context.Context, string, []runtimeprocessor.CredentialBinding) (credentials.Request, error) {
			return credentials.Request{Ref: "run-1", Variables: map[string]string{"MODEL_API_KEY": "secret-value"}}, nil
		}),
		credentials.Materializer{Root: t.TempDir()},
		factoryFunc(func(domain.Lease, runtimeprocessor.Plan, *credentials.Environment) (agentruntime.Adapter, error) {
			return adapter, nil
		}),
		allowApprovals,
	)
	if err != nil {
		t.Fatal(err)
	}
	lease := validLease()
	lease.ModelBudget = []byte(`{"max_input_tokens":100,"max_output_tokens":50,"max_cost_amount":"1.00"}`)
	outcome, err := processor.Execute(context.Background(), lease)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.State != domain.Failed || outcome.Cost != "0.250000" || !strings.Contains(string(outcome.Error), "model_budget_exceeded") || !strings.Contains(string(outcome.Usage), `"input_tokens":101`) {
		t.Fatalf("Outcome = %+v", outcome)
	}
}

func validLease() domain.Lease {
	return domain.Lease{
		RunID: "run-1", AttemptID: "attempt-1", Token: "lease-token", RequestText: "fix tests",
		RuntimeName: "claude", RuntimeCLIVersion: "1.0.0", AdapterVersion: "adapter-1",
		ImageDigest: "registry.example/claude@sha256:" + strings.Repeat("a", 64), WorkspaceVolume: "workspace-run-1",
		ModelBinding:       []byte(`{"model_id":"model-id","endpoint":"https://models.example.test","credential_profile_id":"model-credential"}`),
		ModelBudget:        []byte(`{"max_input_tokens":1000,"max_output_tokens":500,"max_cost_amount":"10.00"}`),
		CredentialBindings: []byte(`[{"ref":"vault://model","purpose":"model"}]`),
		ExecutionLimits:    []byte(`{"timeout_seconds":60,"cpus":2,"memory_bytes":1073741824,"pids":256,"temp_bytes":536870912,"egress":"public"}`),
		Capabilities:       []byte(`{"streaming":true,"usage":true}`),
	}
}
