package runtimeexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

const (
	knowledgeResultLimit = 8
	knowledgeTokenLimit  = 6000
)

func (executor *Executor) injectKnowledgeContext(ctx context.Context, owner string, snapshot workspacedomain.ExecutionSnapshot, instruction string, stagePosition int) (string, []workspacedomain.Evidence, error) {
	if len(snapshot.KnowledgeBaseIDs) == 0 {
		return instruction, nil, nil
	}
	if executor.knowledge == nil {
		evidence := make([]workspacedomain.Evidence, 0, len(snapshot.KnowledgeBaseIDs))
		for _, knowledgeBaseID := range snapshot.KnowledgeBaseIDs {
			evidence = append(evidence, knowledgeEvidence(knowledgeBaseID, "Knowledge Base", "failed", "retrieval unavailable", stagePosition, nil))
		}
		return "", evidence, fmt.Errorf("Knowledge retrieval is configured but no retrieval provider is available")
	}
	query := strings.TrimSpace(strings.Join([]string{snapshot.Goal, instruction}, "\n\n"))
	if query == "" {
		return instruction, nil, nil
	}
	contextParts := make([]string, 0, knowledgeResultLimit)
	evidence := make([]workspacedomain.Evidence, 0, knowledgeResultLimit)
	seen := make(map[string]struct{}, knowledgeResultLimit)
	usedTokens := 0
	for _, knowledgeBaseID := range snapshot.KnowledgeBaseIDs {
		generation := snapshot.KnowledgeIndexGenerations[knowledgeBaseID]
		if generation <= 0 {
			evidence = append(evidence, knowledgeEvidence(knowledgeBaseID, "Knowledge Base", "failed", "ready index unavailable", stagePosition, nil))
			return "", evidence, fmt.Errorf("Knowledge Base %s has no frozen ready index generation", knowledgeBaseID)
		}
		remaining := knowledgeResultLimit - len(contextParts)
		if remaining <= 0 {
			break
		}
		hits, err := executor.knowledge.Search(ctx, owner, knowledgeBaseID, generation, query, remaining, knowledgeTokenLimit-usedTokens)
		if err != nil {
			evidence = append(evidence, knowledgeEvidence(knowledgeBaseID, "Knowledge Base", "failed", "retrieval failed", stagePosition, nil))
			return "", evidence, fmt.Errorf("retrieve Knowledge Base %s: %w", knowledgeBaseID, err)
		}
		usedBase := false
		for _, citation := range hits {
			if len(contextParts) >= knowledgeResultLimit {
				break
			}
			text := strings.TrimSpace(citation.Text)
			if text == "" {
				continue
			}
			digest := sha256.Sum256([]byte(text))
			key := hex.EncodeToString(digest[:])
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			remainingTokens := knowledgeTokenLimit - usedTokens
			if remainingTokens <= 0 {
				break
			}
			if len([]rune(text)) > remainingTokens*4 {
				text = string([]rune(text)[:remainingTokens*4])
			}
			usedTokens += max(1, len([]rune(text))/4)
			location := citation.Source.DocumentName
			if citation.Source.CategoryName != "" {
				location = citation.Source.CategoryName + "/" + location
			}
			contextParts = append(contextParts, fmt.Sprintf("[%d] Knowledge Base %s (%s, revision %s, relevance %.3f)\n%s", len(contextParts)+1, knowledgeBaseID, location, citation.Source.RevisionID, citation.Relevance, text))
			item := knowledgeEvidence(citation.Source.DocumentID, truncateEvidenceText(citation.Source.DocumentName, 255), "succeeded", "retrieved indexed source", stagePosition, &workspacedomain.EvidenceCitation{RevisionID: citation.Source.RevisionID, CategoryName: truncateEvidenceText(citation.Source.CategoryName, 255), SourceLocation: truncateEvidenceText(location, 500), Relevance: boundedRelevance(citation.Relevance)})
			item.ContainerID = knowledgeBaseID
			evidence = append(evidence, item)
			usedBase = true
		}
		if !usedBase {
			evidence = append(evidence, knowledgeEvidence(knowledgeBaseID, "Knowledge Base", "not_used", "no relevant indexed source", stagePosition, nil))
		}
	}
	if len(contextParts) == 0 {
		return instruction + "\n\n[Knowledge Retrieval: no relevant indexed source found]", evidence, nil
	}
	return instruction + "\n\n[Knowledge Retrieval Context]\n" + strings.Join(contextParts, "\n\n") + "\n[End Knowledge Retrieval Context]", evidence, nil
}

func knowledgeEvidence(sourceID, sourceName, state, action string, stagePosition int, citation *workspacedomain.EvidenceCitation) workspacedomain.Evidence {
	return workspacedomain.Evidence{ID: uuid.NewString(), Kind: "knowledge", SourceID: sourceID, SourceName: sourceName, State: state, Action: action, StagePosition: stagePosition, Citation: citation}
}

func truncateEvidenceText(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}

func boundedRelevance(value float32) float32 {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
