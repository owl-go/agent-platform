package runtimeexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

const (
	knowledgeResultLimit = 8
	knowledgeTokenLimit  = 6000
)

func (executor *Executor) injectKnowledgeContext(ctx context.Context, owner string, snapshot workspacedomain.ExecutionSnapshot, instruction string) (string, error) {
	if len(snapshot.KnowledgeBaseIDs) == 0 {
		return instruction, nil
	}
	if executor.knowledge == nil {
		return "", fmt.Errorf("Knowledge retrieval is configured but AnythingLLM is unavailable")
	}
	query := strings.TrimSpace(strings.Join([]string{snapshot.Goal, instruction}, "\n\n"))
	if query == "" {
		return instruction, nil
	}
	contextParts := make([]string, 0, knowledgeResultLimit)
	seen := make(map[string]struct{}, knowledgeResultLimit)
	usedTokens := 0
	for _, knowledgeBaseID := range snapshot.KnowledgeBaseIDs {
		generation := snapshot.KnowledgeIndexGenerations[knowledgeBaseID]
		if generation <= 0 {
			return "", fmt.Errorf("Knowledge Base %s has no frozen ready index generation", knowledgeBaseID)
		}
		remaining := knowledgeResultLimit - len(contextParts)
		if remaining <= 0 {
			break
		}
		hits, err := executor.knowledge.Search(ctx, owner, knowledgeBaseID, generation, query, remaining, knowledgeTokenLimit-usedTokens)
		if err != nil {
			return "", fmt.Errorf("retrieve Knowledge Base %s: %w", knowledgeBaseID, err)
		}
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
		}
	}
	if len(contextParts) == 0 {
		return instruction + "\n\n[Knowledge Retrieval: no relevant indexed source found]", nil
	}
	return instruction + "\n\n[Knowledge Retrieval Context]\n" + strings.Join(contextParts, "\n\n") + "\n[End Knowledge Retrieval Context]", nil
}
