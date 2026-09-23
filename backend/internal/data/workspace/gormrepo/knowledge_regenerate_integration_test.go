package gormrepo

import (
	"context"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestRegenerateReadyKnowledgeDocumentQueuesNewRevisionWithoutReplacingReadySource(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	owner, other, baseID, documentID, revisionID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		if err := db.Exec("INSERT INTO users(id, oidc_subject, username, email, display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO knowledge_bases(id, owner_user_id, name, visibility) VALUES(?, ?, '测试知识库', 'private')", []any{baseID, owner}},
		{"INSERT INTO knowledge_documents(id, knowledge_base_id, name, source_type, normalized_source, state) VALUES(?, ?, '测试.txt', 'upload', 'test-source', 'ready')", []any{documentID, baseID}},
		{"INSERT INTO knowledge_document_revisions(id, document_id, revision, object_key, sha256, size_bytes, content_type, state) VALUES(?, ?, 1, 'knowledge/source.txt', repeat('a', 64), 12, 'text/plain', 'ready')", []any{revisionID, documentID}},
	} {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := New(db, nil)
	if err := repository.RegenerateKnowledgeDocument(ctx, other, baseID, documentID, false); err == nil {
		t.Fatal("another User regenerated a private document")
	}
	if err := repository.RegenerateKnowledgeDocument(ctx, owner, baseID, documentID, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.RegenerateKnowledgeDocument(ctx, owner, baseID, documentID, false); err == nil {
		t.Fatal("duplicate regeneration was queued while one is pending")
	}
	var revisions []struct {
		Revision  int
		ObjectKey string
		State     string
	}
	if err := db.Table("knowledge_document_revisions").Where("document_id = ?", documentID).Order("revision").Find(&revisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0].State != string(domain.KnowledgeReady) || revisions[1].State != string(domain.KnowledgeAccepted) || revisions[1].ObjectKey != revisions[0].ObjectKey {
		t.Fatalf("revision history after regeneration = %+v", revisions)
	}
	var queued int64
	if err := db.Table("knowledge_ingestion_jobs").Where("state = 'queued' AND revision_id IN (SELECT id FROM knowledge_document_revisions WHERE document_id = ? AND revision = 2)", documentID).Count(&queued).Error; err != nil || queued != 1 {
		t.Fatalf("queued jobs = %d, err = %v", queued, err)
	}
	if err := db.Exec("UPDATE knowledge_document_revisions SET state = 'failed' WHERE document_id = ? AND revision = 2", documentID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE knowledge_ingestion_jobs SET state = 'failed' WHERE revision_id IN (SELECT id FROM knowledge_document_revisions WHERE document_id = ? AND revision = 2)", documentID).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.RetryKnowledgeDocument(ctx, owner, baseID, documentID, false); err != nil {
		t.Fatal(err)
	}
	var documentState string
	if err := db.Table("knowledge_documents").Select("state").Where("id = ?", documentID).Scan(&documentState).Error; err != nil || documentState != string(domain.KnowledgeReady) {
		t.Fatalf("document state during retry = %q, err = %v", documentState, err)
	}
}
