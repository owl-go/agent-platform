package gormrepo

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestKnowledgeIngestionWithoutEmbeddingNeverMarksDocumentReady(t *testing.T) {
	db := rateLimitTestDatabase(t)
	ctx := context.Background()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users (id, oidc_subject, username, email, display_name) VALUES (?, ?, ?, ?, ?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	baseID, documentID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if err := db.Exec("INSERT INTO knowledge_bases (id, owner_user_id, name, visibility) VALUES (?, ?, ?, 'private')", baseID, owner, "知识库").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO knowledge_documents (id, knowledge_base_id, name, source_type, normalized_source, content, state) VALUES (?, ?, ?, 'upload', ?, ?, 'processing')", documentID, baseID, "介绍.txt", "test-source", "检索测试文档").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO ai_application_knowledge_jobs (id, document_id, state) VALUES (?, ?, 'queued')", jobID, documentID).Error; err != nil {
		t.Fatal(err)
	}
	processed, err := New(db, nil).ProcessNextKnowledgeDocument(ctx)
	if err != nil || !processed {
		t.Fatalf("process document: processed=%t error=%v", processed, err)
	}
	var result struct{ State, Error string }
	if err := db.Table("knowledge_documents").Select("state, error").Where("id = ?", documentID).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.State != "failed" {
		t.Fatalf("unembedded document state = %q, want failed", result.State)
	}
	if !strings.Contains(result.Error, "embedding provider is not configured") {
		t.Fatalf("failure reason = %q, want missing embedding configuration", result.Error)
	}
	var chunks int64
	if err := db.Table("knowledge_chunks").Where("document_id = ? AND embedding IS NOT NULL", documentID).Count(&chunks).Error; err != nil {
		t.Fatal(err)
	}
	if chunks != 0 {
		t.Fatalf("unembedded document has %d vector chunks", chunks)
	}
}
