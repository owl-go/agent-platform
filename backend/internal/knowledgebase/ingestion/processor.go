package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/objectstore"
)

// Provider is the replaceable indexing boundary. Provider-specific identifiers
// and credentials must remain inside its implementation.
type Provider interface {
	UpsertRevision(context.Context, string, string, string, []byte) error
	RemoveRevision(context.Context, string, string) error
}

type Processor struct {
	repository workspaceapplication.KnowledgeIngestionRepository
	objects    objectstore.Provider
	provider   Provider
}

func New(repository workspaceapplication.KnowledgeIngestionRepository, objects objectstore.Provider, provider Provider) (*Processor, error) {
	if repository == nil || objects == nil || provider == nil {
		return nil, fmt.Errorf("Knowledge Ingestion requires a repository, Object Store, and indexing provider")
	}
	return &Processor{repository: repository, objects: objects, provider: provider}, nil
}

func (processor *Processor) ProcessNext(ctx context.Context) (bool, error) {
	job, err := processor.repository.ClaimKnowledgeIngestionJob(ctx)
	if err != nil || job == nil {
		return false, err
	}
	processingCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	processingErr := processor.process(processingCtx, *job)
	cancel()
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer finishCancel()
	finishErr := processor.repository.FinishKnowledgeIngestionJob(finishCtx, *job, processingErr)
	if processingErr != nil && finishErr != nil {
		return true, fmt.Errorf("Knowledge Ingestion failed: %v; recording failure: %w", processingErr, finishErr)
	}
	if finishErr != nil {
		return true, finishErr
	}
	retained, supportsRetention := processor.provider.(interface{ RetainsRevisions() bool })
	if processingErr == nil && !(supportsRetention && retained.RetainsRevisions()) {
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
	if object.Size < 0 || object.Size > 100*1024*1024 || (job.SHA256 != "" && object.Size != job.Size) {
		return fmt.Errorf("Knowledge source size does not match its immutable revision")
	}
	content, err := io.ReadAll(io.LimitReader(reader, object.Size+1))
	if err != nil {
		return fmt.Errorf("read Knowledge source bytes: %w", err)
	}
	if int64(len(content)) != object.Size {
		return fmt.Errorf("Knowledge source size changed while ingesting")
	}
	if job.SHA256 != "" {
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != job.SHA256 {
			return fmt.Errorf("Knowledge source digest does not match its immutable revision")
		}
	}
	if err := processor.provider.UpsertRevision(ctx, job.KnowledgeBaseID, job.RevisionID, job.ContentType, content); err != nil {
		return fmt.Errorf("index Knowledge revision: %w", err)
	}
	return nil
}
