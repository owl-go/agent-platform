package application

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/productanalytics"
)

type cancellationRepository struct {
	claimed   atomic.Bool
	requested atomic.Bool
	finished  chan struct{}
}

func (repository *cancellationRepository) ClaimNext(context.Context) (*ExecutionJob, error) {
	if repository.claimed.Swap(true) {
		return nil, nil
	}
	return &ExecutionJob{Kind: JobSession, ID: "session-1-2", OwnerID: "owner-1", SessionID: "session-1", AssistantMessageID: 2}, nil
}

func (*cancellationRepository) FinishSucceeded(context.Context, ExecutionJob, ExecutionResult) error {
	return nil
}

func (*cancellationRepository) FinishFailed(context.Context, ExecutionJob, ExecutionResult, string) error {
	return nil
}

func (repository *cancellationRepository) FinishCancelled(context.Context, ExecutionJob, ExecutionResult) error {
	close(repository.finished)
	return nil
}

func (*cancellationRepository) FinishMCPTest(context.Context, ExecutionJob, string) error { return nil }
func (*cancellationRepository) FinishExpertTagProjection(context.Context, ExecutionJob, ExecutionResult, string) error {
	return nil
}
func (*cancellationRepository) FinishCLIConnectorBuild(context.Context, ExecutionJob, cliconnector.BuildResult, string) error {
	return nil
}

func (*cancellationRepository) RecordProgress(context.Context, ExecutionJob, ExecutionEvent) error {
	return nil
}

func (repository *cancellationRepository) CancellationRequested(context.Context, ExecutionJob) (bool, error) {
	return repository.requested.Load(), nil
}

type cancellationExecutor struct{ started chan struct{} }

func (executor *cancellationExecutor) Execute(ctx context.Context, _ ExecutionJob, _ ProgressRecorder) (ExecutionResult, error) {
	close(executor.started)
	<-ctx.Done()
	return ExecutionResult{}, ctx.Err()
}

type connectorBuildRepository struct {
	cancellationRepository
	failure string
}

func (repository *connectorBuildRepository) ClaimNext(context.Context) (*ExecutionJob, error) {
	if repository.claimed.Swap(true) {
		return nil, nil
	}
	return &ExecutionJob{Kind: JobCLIConnectorBuild, ID: "cli-build-definition-1", CLIConnector: cliconnector.Definition{ID: "definition-1"}}, nil
}

func (repository *connectorBuildRepository) FinishCLIConnectorBuild(_ context.Context, _ ExecutionJob, _ cliconnector.BuildResult, failure string) error {
	repository.failure = failure
	return nil
}

type unusedExecutor struct{}

func (unusedExecutor) Execute(context.Context, ExecutionJob, ProgressRecorder) (ExecutionResult, error) {
	return ExecutionResult{}, nil
}

type terminalRepository struct {
	cancellationRepository
	job ExecutionJob
}

func (repository *terminalRepository) ClaimNext(context.Context) (*ExecutionJob, error) {
	if repository.claimed.Swap(true) {
		return nil, nil
	}
	return &repository.job, nil
}

type terminalExecutor struct {
	result ExecutionResult
	err    error
}

func (executor terminalExecutor) Execute(context.Context, ExecutionJob, ProgressRecorder) (ExecutionResult, error) {
	return executor.result, executor.err
}

type recordingAnalytics struct {
	productanalytics.Nop
	sessions  []productanalytics.SessionTerminalObservation
	workflows []productanalytics.WorkflowTerminalObservation
}

func (analytics *recordingAnalytics) SessionTerminal(_ context.Context, observation productanalytics.SessionTerminalObservation) {
	analytics.sessions = append(analytics.sessions, observation)
}

func (analytics *recordingAnalytics) WorkflowTerminal(_ context.Context, observation productanalytics.WorkflowTerminalObservation) {
	analytics.workflows = append(analytics.workflows, observation)
}

func TestWorkerFailsCLIConnectorBuildClosedWithoutIsolatedBuilder(t *testing.T) {
	repository := &connectorBuildRepository{}
	worker, err := NewWorker(repository, unusedExecutor{})
	if err != nil {
		t.Fatal(err)
	}
	processed, err := worker.ProcessNext(context.Background())
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	if repository.failure != "isolated CLI Connector Builder is not configured" {
		t.Fatalf("failure = %q", repository.failure)
	}
}

func TestWorkerStopsActiveExecutionAfterCancellationRequest(t *testing.T) {
	repository := &cancellationRepository{finished: make(chan struct{})}
	executor := &cancellationExecutor{started: make(chan struct{})}
	worker, err := NewWorker(repository, executor)
	if err != nil {
		t.Fatalf("NewWorker: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, processErr := worker.ProcessNext(context.Background())
		done <- processErr
	}()

	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	repository.requested.Store(true)

	select {
	case <-repository.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not finish the Session message as cancelled")
	}
	if err := <-done; err != nil {
		t.Fatalf("ProcessNext: %v", err)
	}
}

func TestWorkerRecordsOnlyPrivacyBoundedSessionTerminalFacts(t *testing.T) {
	repository := &terminalRepository{job: ExecutionJob{Kind: JobSession, ID: "execution-1", OwnerID: "owner-1", SessionID: "session-1", AssistantMessageID: 2}}
	result := ExecutionResult{FinalMessage: "private answer", ExpertStages: []domain.ExpertStage{{ElapsedMS: 1_500}}, Artifacts: []ExecutionArtifact{{Name: "private.txt"}}}
	analytics := &recordingAnalytics{}
	worker, err := NewWorker(repository, terminalExecutor{result: result})
	if err != nil {
		t.Fatal(err)
	}
	worker.EnableProductAnalytics(analytics)

	if processed, processErr := worker.ProcessNext(context.Background()); processErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, processErr)
	}
	if len(analytics.sessions) != 1 {
		t.Fatalf("observations = %#v", analytics.sessions)
	}
	observation := analytics.sessions[0]
	if observation.State != "succeeded" || observation.Duration != 1500*time.Millisecond || observation.ArtifactCount != 1 || observation.SafeErrorCode != "none" {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestWorkerClassifiesSessionFailuresWithoutRecordingRawError(t *testing.T) {
	repository := &terminalRepository{job: ExecutionJob{Kind: JobSession, ID: "execution-2", OwnerID: "owner-1", SessionID: "session-1", AssistantMessageID: 2}}
	runtimeError := &agentruntime.Error{Code: agentruntime.ErrorAuthenticationFailed, Message: "private provider response", Cause: errors.New("credential detail")}
	analytics := &recordingAnalytics{}
	worker, err := NewWorker(repository, terminalExecutor{err: runtimeError})
	if err != nil {
		t.Fatal(err)
	}
	worker.EnableProductAnalytics(analytics)

	if processed, processErr := worker.ProcessNext(context.Background()); processErr != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, processErr)
	}
	observation := analytics.sessions[0]
	if observation.SafeErrorCode != "authentication_failed" || observation.FailureStage != "authorization" || !observation.Recoverable {
		t.Fatalf("observation = %#v", observation)
	}
}
