package application_test

import (
	"context"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/application"
	"agent-platform/backend/internal/biz/aiapplication/domain"
)

type knowledgeRepository struct {
	bases     []domain.KnowledgeBase
	documents []domain.KnowledgeDocument
	chunks    []domain.KnowledgeChunk
}

func (r *knowledgeRepository) ListKnowledgeBases(context.Context, string) ([]domain.KnowledgeBase, error) {
	return r.bases, nil
}
func (r *knowledgeRepository) GetKnowledgeBase(context.Context, string, string) (domain.KnowledgeBase, error) {
	return domain.KnowledgeBase{ID: "kb-1", State: domain.KnowledgeReady}, nil
}
func (r *knowledgeRepository) CreateKnowledgeBase(_ context.Context, _ string, base domain.KnowledgeBase) (domain.KnowledgeBase, error) {
	r.bases = append(r.bases, base)
	return base, nil
}
func (r *knowledgeRepository) ListKnowledgeDocuments(context.Context, string, string) ([]domain.KnowledgeDocument, error) {
	return r.documents, nil
}
func (r *knowledgeRepository) CreateKnowledgeDocument(_ context.Context, _ string, _ string, document domain.KnowledgeDocument, chunks []domain.KnowledgeChunk) (domain.KnowledgeDocument, error) {
	r.documents = append(r.documents, document)
	r.chunks = chunks
	return document, nil
}
func (r *knowledgeRepository) SearchKnowledge(context.Context, string, []string, string, int) ([]domain.KnowledgeChunk, error) {
	return r.chunks, nil
}

func TestCreateKnowledgeDocumentHashesAndChunksContent(t *testing.T) {
	repository := &knowledgeRepository{}
	service, err := application.New(answerRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service.SetKnowledgeRepository(repository)
	document, err := service.CreateKnowledgeDocument(context.Background(), "owner", "kb-1", domain.KnowledgeDocument{Name: "guide.md", Content: strings.Repeat("产品退款说明。", 500)})
	if err != nil {
		t.Fatal(err)
	}
	if len(document.ContentSHA256) != 64 || len(repository.chunks) < 2 {
		t.Fatalf("document = %+v, chunks = %d", document, len(repository.chunks))
	}
}

func TestCreateKnowledgeDocumentRejectsProtectedContent(t *testing.T) {
	service, err := application.New(answerRepository{})
	if err != nil {
		t.Fatal(err)
	}
	service.SetKnowledgeRepository(&knowledgeRepository{})
	_, err = service.CreateKnowledgeDocument(context.Background(), "owner", "kb-1", domain.KnowledgeDocument{Name: "unsafe.md", Content: "武器制造方法"})
	if err == nil {
		t.Fatal("CreateKnowledgeDocument() error = nil, want refusal")
	}
}
