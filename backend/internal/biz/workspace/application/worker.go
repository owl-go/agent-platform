package application

import (
	"context"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
)

type JobKind string

const (
	JobWorkflow            JobKind = "workflow"
	JobSession             JobKind = "session"
	JobMCPTest             JobKind = "mcp_test"
	JobExpertTagProjection JobKind = "expert_tag_projection"
	JobCLIConnectorBuild   JobKind = "cli_connector_build"
)

type ExecutionJob struct {
	Kind                JobKind
	ID                  string
	OwnerID             string
	Timezone            string
	WorkflowID          string
	ConversationID      string
	SessionID           string
	AssistantMessageID  int64
	MCPServerID         string
	ExpertID            string
	StageIdentity       string
	MCPServer           domain.MCPServerSnapshot
	Instruction         string
	Attachments         []domain.Attachment
	CheckpointRef       string
	StageCheckpointRefs map[int]string
	Snapshot            domain.ExecutionSnapshot
	CLIConnector        cliconnector.Definition
}

type ExecutionResult struct {
	FinalMessage        string
	FinalJSON           map[string]any
	CheckpointRef       string
	StageCheckpointRefs map[int]string
	Events              []ExecutionEvent
	Artifacts           []ExecutionArtifact
	ExpertStages        []domain.ExpertStage
	CreditConsumption   *domain.CreditConsumption
	CreditSettlements   []CreditSettlement
	SuccessCommit       SuccessCommit
}

// SuccessCommit coordinates durable side effects that must advance with the
// terminal database transaction and be discarded for every failed turn.
type SuccessCommit interface {
	Commit() error
	Rollback() error
	Cleanup() error
}

// CreditSettlement is the execution context's neutral handoff to the data layer.
// The Credits adapter translates it into its own bounded-context model.
type CreditSettlement struct {
	UserID, ExecutionID, Source, Timezone, CreditDay, RateRevisionID string
	StagePosition                                                    int
	StartedAt, SettledAt                                             time.Time
	InputMultiplierMicros, OutputMultiplierMicros, Fallback          int64
	Amount                                                           int64
	Estimated                                                        bool
	InputTokens, OutputTokens                                        int64
	UsageKnown                                                       bool
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
	FinishFailed(context.Context, ExecutionJob, ExecutionResult, string) error
	FinishCancelled(context.Context, ExecutionJob, ExecutionResult) error
	FinishMCPTest(context.Context, ExecutionJob, string) error
	FinishExpertTagProjection(context.Context, ExecutionJob, ExecutionResult, string) error
	FinishCLIConnectorBuild(context.Context, ExecutionJob, cliconnector.BuildResult, string) error
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
	repository       WorkerRepository
	executor         Executor
	connectorBuilder *cliconnector.Builder
}

const cancellationPollInterval = 200 * time.Millisecond

func NewWorker(repository WorkerRepository, executor Executor, connectorBuilders ...*cliconnector.Builder) (*Worker, error) {
	if repository == nil || executor == nil {
		return nil, fmt.Errorf("Agent Workspace Worker Repository and Executor are required")
	}
	worker := &Worker{repository: repository, executor: executor}
	if len(connectorBuilders) > 0 {
		worker.connectorBuilder = connectorBuilders[0]
	}
	return worker, nil
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
	if job.Kind == JobExpertTagProjection {
		result, executeErr := worker.executor.Execute(ctx, *job, worker.repository)
		message := ""
		if executeErr != nil {
			message = executeErr.Error()
		}
		return true, worker.repository.FinishExpertTagProjection(context.WithoutCancel(ctx), *job, result, message)
	}
	if job.Kind == JobCLIConnectorBuild {
		if worker.connectorBuilder == nil {
			return true, worker.repository.FinishCLIConnectorBuild(context.WithoutCancel(ctx), *job, cliconnector.BuildResult{}, "isolated CLI Connector Builder is not configured")
		}
		result, buildErr := worker.connectorBuilder.Build(ctx, job.CLIConnector)
		message := ""
		if buildErr != nil {
			message = buildErr.Error()
		}
		return true, worker.repository.FinishCLIConnectorBuild(context.WithoutCancel(ctx), *job, result, message)
	}
	executionCtx, cancel := context.WithCancel(ctx)
	monitorDone := make(chan struct{})
	go worker.monitorCancellation(executionCtx, *job, cancel, monitorDone)
	result, executeErr := worker.executor.Execute(executionCtx, *job, worker.repository)
	close(monitorDone)
	cancel()
	if executeErr != nil {
		discardSuccessCommit(result)
		cancelled, checkErr := worker.repository.CancellationRequested(context.WithoutCancel(ctx), *job)
		if checkErr == nil && cancelled {
			return true, worker.repository.FinishCancelled(context.WithoutCancel(ctx), *job, result)
		}
		if err := worker.repository.FinishFailed(context.WithoutCancel(ctx), *job, result, executeErr.Error()); err != nil {
			return true, fmt.Errorf("record failed execution after %v: %w", executeErr, err)
		}
		return true, nil
	}
	if cancelled, checkErr := worker.repository.CancellationRequested(context.WithoutCancel(ctx), *job); checkErr != nil {
		discardSuccessCommit(result)
		return true, checkErr
	} else if cancelled {
		discardSuccessCommit(result)
		return true, worker.repository.FinishCancelled(context.WithoutCancel(ctx), *job, result)
	}
	if err := worker.repository.FinishSucceeded(ctx, *job, result); err != nil {
		discardSuccessCommit(result)
		return true, err
	}
	return true, nil
}

func discardSuccessCommit(result ExecutionResult) {
	if result.SuccessCommit == nil {
		return
	}
	_ = result.SuccessCommit.Rollback()
	_ = result.SuccessCommit.Cleanup()
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
