package application

import (
	"context"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type JobKind string

const (
	JobWorkflow JobKind = "workflow"
	JobSession  JobKind = "session"
	JobMCPTest  JobKind = "mcp_test"
)

type ExecutionJob struct {
	Kind               JobKind
	ID                 string
	OwnerID            string
	WorkflowID         string
	SessionID          string
	AssistantMessageID int64
	MCPServerID        string
	MCPServer          domain.MCPServerSnapshot
	Instruction        string
	Attachments        []domain.Attachment
	CheckpointRef      string
	Snapshot           domain.ExecutionSnapshot
}

type ExecutionResult struct {
	FinalMessage  string
	FinalJSON     map[string]any
	CheckpointRef string
	Events        []ExecutionEvent
	Artifacts     []ExecutionArtifact
	ExpertStages  []domain.ExpertStage
}

type ExecutionArtifact struct {
	Name        string
	Path        string
	ObjectKey   string
	TextPreview string
	Size        int64
	SHA256      string
	ExpiresAt   time.Time
}

type ExecutionEvent struct {
	Type    string
	Payload []byte
}

type WorkerRepository interface {
	ClaimNext(context.Context) (*ExecutionJob, error)
	FinishSucceeded(context.Context, ExecutionJob, ExecutionResult) error
	FinishFailed(context.Context, ExecutionJob, string) error
	FinishCancelled(context.Context, ExecutionJob) error
	FinishMCPTest(context.Context, ExecutionJob, string) error
	RecordProgress(context.Context, ExecutionJob, ExecutionEvent) error
	CancellationRequested(context.Context, ExecutionJob) (bool, error)
}

type Executor interface {
	Execute(context.Context, ExecutionJob, ProgressRecorder) (ExecutionResult, error)
}

type ProgressRecorder interface {
	RecordProgress(context.Context, ExecutionJob, ExecutionEvent) error
}

type Worker struct {
	repository WorkerRepository
	executor   Executor
}

const cancellationPollInterval = 200 * time.Millisecond

func NewWorker(repository WorkerRepository, executor Executor) (*Worker, error) {
	if repository == nil || executor == nil {
		return nil, fmt.Errorf("Agent Workspace Worker Repository and Executor are required")
	}
	return &Worker{repository: repository, executor: executor}, nil
}

func (worker *Worker) ProcessNext(ctx context.Context) (bool, error) {
	job, err := worker.repository.ClaimNext(ctx)
	if err != nil || job == nil {
		return false, err
	}
	if job.Kind == JobMCPTest {
		_, executeErr := worker.executor.Execute(ctx, *job, worker.repository)
		message := ""
		if executeErr != nil {
			message = executeErr.Error()
		}
		return true, worker.repository.FinishMCPTest(context.WithoutCancel(ctx), *job, message)
	}
	executionCtx, cancel := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go worker.monitorCancellation(executionCtx, *job, cancel, monitorDone)
	result, executeErr := worker.executor.Execute(executionCtx, *job, worker.repository)
	close(monitorDone)
	cancel()
	if executeErr != nil {
		cancelled, checkErr := worker.repository.CancellationRequested(context.WithoutCancel(ctx), *job)
		if checkErr == nil && cancelled {
			return true, worker.repository.FinishCancelled(context.WithoutCancel(ctx), *job)
		}
		if err := worker.repository.FinishFailed(context.WithoutCancel(ctx), *job, executeErr.Error()); err != nil {
			return true, fmt.Errorf("record failed execution after %v: %w", executeErr, err)
		}
		return true, nil
	}
	if cancelled, checkErr := worker.repository.CancellationRequested(context.WithoutCancel(ctx), *job); checkErr != nil {
		return true, checkErr
	} else if cancelled {
		return true, worker.repository.FinishCancelled(context.WithoutCancel(ctx), *job)
	}
	if err := worker.repository.FinishSucceeded(ctx, *job, result); err != nil {
		return true, err
	}
	return true, nil
}

func (worker *Worker) monitorCancellation(ctx context.Context, job ExecutionJob, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(cancellationPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			requested, err := worker.repository.CancellationRequested(ctx, job)
			if err == nil && requested {
				cancel()
				return
			}
		}
	}
}
