// Package retrieval is the single, permission-checked read path for Knowledge
// Bases. A replaceable provider supplies candidate chunks; platform revisions
// determine whether a candidate can actually be used or cited.
package retrieval

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

var revisionIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

type SourceRepository interface {
	ReadyKnowledgeSearchGeneration(context.Context, string, string, bool) (int64, error)
	ResolveKnowledgeSearchSource(context.Context, string, string, string, bool) (domain.KnowledgeSearchSource, error)
}

// GenerationSourceRepository supports retained, immutable generation manifests.
type GenerationSourceRepository interface {
	ValidateKnowledgeGeneration(context.Context, string, string, int64) error
	ResolveKnowledgeGenerationSource(context.Context, string, string, string, int64) (domain.KnowledgeSearchSource, error)
}

type Hit struct {
	Source    domain.KnowledgeSearchSource
	Text      string
	Relevance float32
}

type Searcher interface {
	Search(context.Context, string, string, int64, string, int, int) ([]Hit, error)
}

type Citation struct {
	RevisionID     string
	SourceLocation string
	Relevance      float32
	Text           string
}

type Result struct {
	Citations []Citation
}

type Provider interface {
	Query(context.Context, string, int64, string, int, int) (Result, error)
}

type Engine struct {
	repository SourceRepository
	provider   Provider
}

func New(repository SourceRepository, provider Provider) (*Engine, error) {
	if repository == nil || provider == nil {
		return nil, fmt.Errorf("Knowledge retrieval requires a repository and retrieval provider")
	}
	return &Engine{repository: repository, provider: provider}, nil
}

// Search resolves candidates under current access. Snapshot-aware repositories
// support retained frozen revisions; legacy providers fail closed on older generations.
func (engine *Engine) Search(ctx context.Context, principalID, baseID string, generation int64, question string, limit, tokenLimit int) ([]Hit, error) {
	if principalID == "" || baseID == "" {
		return nil, domain.ErrNotFound
	}
	question = strings.TrimSpace(question)
	if question == "" || limit <= 0 || tokenLimit <= 0 {
		return []Hit{}, nil
	}
	// The owner/public rule in the repository is sufficient for reads: an owner
	// may read their private platform base, and other Users may read public ones.
	latest, err := engine.repository.ReadyKnowledgeSearchGeneration(ctx, principalID, baseID, true)
	if err != nil {
		return nil, err
	}
	if latest == 0 {
		if generation > 0 {
			return nil, fmt.Errorf("%w: frozen Knowledge index generation is unavailable", domain.ErrInvalid)
		}
		return []Hit{}, nil
	}
	preview := generation == 0
	if generation == 0 {
		generation = latest
	}
	snapshotRepository, snapshots := engine.repository.(GenerationSourceRepository)
	if snapshots {
		if err := snapshotRepository.ValidateKnowledgeGeneration(ctx, principalID, baseID, generation); err != nil {
			return nil, err
		}
	}
	if !snapshots && generation != latest {
		return nil, fmt.Errorf("%w: Knowledge index generation is unavailable", domain.ErrInvalid)
	}
	candidates := limit * 4
	if candidates < 30 {
		candidates = 30
	}
	if candidates > 100 {
		candidates = 100
	}
	retrieved, err := engine.provider.Query(ctx, baseID, generation, question, candidates, tokenLimit)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, citation := range retrieved.Citations {
		if len(hits) == limit {
			break
		}
		revisionID := revisionIDPattern.FindString(citation.RevisionID)
		if revisionID == "" {
			revisionID = revisionIDPattern.FindString(citation.SourceLocation)
		}
		if _, err := uuid.Parse(revisionID); err != nil {
			continue
		}
		var source domain.KnowledgeSearchSource
		var err error
		if snapshots {
			source, err = snapshotRepository.ResolveKnowledgeGenerationSource(ctx, principalID, baseID, revisionID, generation)
		} else {
			source, err = engine.repository.ResolveKnowledgeSearchSource(ctx, principalID, baseID, revisionID, true)
		}
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		content := strings.TrimSpace(citation.Text)
		if content == "" {
			continue
		}
		key := revisionID + "\x00" + content
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		hits = append(hits, Hit{Source: source, Text: content, Relevance: citation.Relevance})
	}
	current, err := engine.repository.ReadyKnowledgeSearchGeneration(ctx, principalID, baseID, true)
	if err != nil {
		return nil, err
	}
	if snapshots {
		if preview && current != generation {
			return nil, fmt.Errorf("%w: Knowledge index generation changed during retrieval", domain.ErrInvalid)
		}
		if err := snapshotRepository.ValidateKnowledgeGeneration(ctx, principalID, baseID, generation); err != nil {
			return nil, err
		}
		// Recheck all source permissions after the provider call and source reads.
		for _, hit := range hits {
			if _, err := snapshotRepository.ResolveKnowledgeGenerationSource(ctx, principalID, baseID, hit.Source.RevisionID, generation); err != nil {
				return nil, err
			}
		}
	} else if current != generation {
		return nil, fmt.Errorf("%w: Knowledge index generation changed during retrieval", domain.ErrInvalid)
	}
	return hits, nil
}
