package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"github.com/google/uuid"
)

func TestVerifiedKnowledgeResults(t *testing.T) {
	valid := uuid.NewString()
	stale := uuid.NewString()
	citations := []anythingllm.Citation{
		{RevisionID: "unknown", Text: "unattributed"},
		{RevisionID: stale, Text: "deleted"},
	}
	for index := 0; index < 12; index++ {
		citation := anythingllm.Citation{RevisionID: valid + ".txt", Text: strings.Repeat("文", 1300), Relevance: float32(index)}
		if index == 0 {
			citation.RevisionID = ""
			citation.SourceLocation = "file:///docs/" + valid + ".txt"
		}
		citations = append(citations, citation)
	}
	results, err := verifiedKnowledgeResults(context.Background(), citations, func(_ context.Context, revisionID string) (domain.KnowledgeSearchSource, error) {
		if revisionID == stale {
			return domain.KnowledgeSearchSource{}, domain.ErrNotFound
		}
		return domain.KnowledgeSearchSource{DocumentID: "doc", RevisionID: valid, DocumentName: "source.txt", CategoryName: "manual"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 || results[0].DocumentName != "source.txt" || results[0].CategoryName != "manual" || len([]rune(results[0].Text)) != 1200 || results[9].Relevance != 9 {
		t.Fatalf("unexpected verified results: %#v", results)
	}
}

func TestVerifiedKnowledgeResultsFailsClosedOnStorageError(t *testing.T) {
	citations := []anythingllm.Citation{{RevisionID: uuid.NewString(), Text: "text"}}
	storageErr := errors.New("storage unavailable")
	_, err := verifiedKnowledgeResults(context.Background(), citations, func(context.Context, string) (domain.KnowledgeSearchSource, error) {
		return domain.KnowledgeSearchSource{}, storageErr
	})
	if !errors.Is(err, storageErr) {
		t.Fatalf("expected storage failure, got %v", err)
	}
}
