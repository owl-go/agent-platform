package ingestion

import (
	"context"
	"fmt"
	"io"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"agent-platform/backend/internal/objectstore"
)

type Processor struct {
	repository workspaceapplication.KnowledgeIngestionRepository
	objects    objectstore.Provider
	provider   anythingllm.Adapter
}

func New(repository workspaceapplication.KnowledgeIngestionRepository, objects objectstore.Provider, provider anythingllm.Adapter) (*Processor, error) {
	if repository == nil || objects == nil || provider == nil {
		return nil, fmt.Errorf("Knowledge Ingestion requires a repository, Object Store, and AnythingLLM adapter")
	}
	return &Processor{repository: repository, objects: objects, provider: provider}, nil
}

func (processor *Processor) ProcessNext(ctx context.Context) (bool, error) {
	job, err := processor.repository.ClaimKnowledgeIngestionJob(ctx)
	if err != nil || job == nil {
		return false, err
	}
	processingErr := processor.process(ctx, *job)
	finishErr := processor.repository.FinishKnowledgeIngestionJob(context.WithoutCancel(ctx), *job, processingErr)
	if processingErr != nil && finishErr != nil {
		return true, fmt.Errorf("Knowledge Ingestion failed: %v; recording failure: %w", processingErr, finishErr)
	}
	if finishErr != nil {
		return true, finishErr
	}
	if processingErr == nil {
		// Commit the new Ready revision before removing the previous provider
		// vectors. A cleanup failure cannot roll the committed job backward;
		// source validation still rejects superseded vectors at query time.
		previous, err := processor.repository.SupersededKnowledgeRevisions(ctx, job.DocumentID, job.RevisionID)
		if err != nil {
			return true, fmt.Errorf("find superseded Knowledge revisions: %w", err)
		}
		for _, revisionID := range previous {
			if err := processor.provider.RemoveRevision(ctx, job.KnowledgeBaseID, revisionID); err != nil {
				return true, fmt.Errorf("remove superseded Knowledge revision: %w", err)
			}
		}
	}
	return true, nil
}

func (processor *Processor) process(ctx context.Context, job workspaceapplication.KnowledgeIngestionJob) error {
	reader, object, err := processor.objects.Get(ctx, job.ObjectKey)
	if err != nil {
		return fmt.Errorf("read Knowledge source: %w", err)
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, object.Size+1))
	if err != nil {
		return fmt.Errorf("read Knowledge source bytes: %w", err)
	}
	if int64(len(content)) != object.Size {
		return fmt.Errorf("Knowledge source size changed while ingesting")
	}
	if err := processor.provider.UpsertRevision(ctx, job.KnowledgeBaseID, job.RevisionID, job.ContentType, content); err != nil {
		return fmt.Errorf("index Knowledge revision: %w", err)
	}
	return nil
}
