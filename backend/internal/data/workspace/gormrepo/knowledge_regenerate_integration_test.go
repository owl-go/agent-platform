package gormrepo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestKnowledgeRegenerationKeepsReadySourceAndQueuesNewRevision(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	owner, other := uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	baseID, documentID, oldID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Base','private')", []any{baseID, owner}},
		{"INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'source.txt','upload','source','ready')", []any{documentID, baseID}},
		{"INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,1,'knowledge/source',?,12,'text/plain','ready')", []any{oldID, documentID, strings.Repeat("a", 64)}},
		{"INSERT INTO knowledge_index_generations(knowledge_base_id,generation,state) VALUES(?,1,'ready')", []any{baseID}},
	} {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository := New(db, nil)
	if err := repository.RegenerateKnowledgeDocument(ctx, other, baseID, documentID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-owner regenerate = %v", err)
	}
	if err := repository.RegenerateKnowledgeDocument(ctx, owner, baseID, documentID, false); err != nil {
		t.Fatal(err)
	}
	if err := repository.RegenerateKnowledgeDocument(ctx, owner, baseID, documentID, false); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate regeneration = %v", err)
	}
	var newRevision struct {
		ID        string
		ObjectKey string
		State     string
	}
	if err := db.Table("knowledge_document_revisions").Where("document_id = ? AND revision = 2", documentID).Take(&newRevision).Error; err != nil {
		t.Fatal(err)
	}
	if newRevision.ObjectKey != "knowledge/source" || newRevision.State != "accepted" {
		t.Fatalf("new revision = %+v", newRevision)
	}
	if _, err := repository.ResolveKnowledgeSearchSource(ctx, owner, baseID, oldID, false); err != nil {
		t.Fatalf("old Ready source disappeared during regeneration: %v", err)
	}
	var queued int64
	if err := db.Table("knowledge_ingestion_jobs").Where("revision_id = ? AND state = 'queued'", newRevision.ID).Count(&queued).Error; err != nil || queued != 1 {
		t.Fatalf("queued jobs = %d, %v", queued, err)
	}
	if err := db.Table("knowledge_document_revisions").Where("id = ?", newRevision.ID).Update("state", string(domain.KnowledgeFailed)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("knowledge_ingestion_jobs").Where("revision_id = ?", newRevision.ID).Update("state", "failed").Error; err != nil {
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

func TestUnifiedKnowledgeMigrationRequeuesUnverifiedReadySources(t *testing.T) {
	db := conversationTestDatabase(t)
	owner, baseID, documentID, revisionID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", []any{owner, owner, owner, owner + "@example.test", owner}},
		{"INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Base','private')", []any{baseID, owner}},
		{"INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'source.txt','upload','source','ready')", []any{documentID, baseID}},
		{"INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,1,'knowledge/source',?,12,'text/plain','ready')", []any{revisionID, documentID, strings.Repeat("a", 64)}},
		{"INSERT INTO knowledge_index_generations(knowledge_base_id,generation,state) VALUES(?,1,'ready')", []any{baseID}},
	} {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	migration, err := os.ReadFile("../../../infrastructure/gormdb/migrations/000056_unified_knowledge_reindex.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	var state string
	if err := db.Table("knowledge_document_revisions").Where("id = ?", revisionID).Select("state").Scan(&state).Error; err != nil || state != "accepted" {
		t.Fatalf("revision state = %q, %v", state, err)
	}
	if err := db.Table("knowledge_documents").Where("id = ?", documentID).Select("state").Scan(&state).Error; err != nil || state != "accepted" {
		t.Fatalf("document state = %q, %v", state, err)
	}
	var queued int64
	if err := db.Table("knowledge_ingestion_jobs").Where("revision_id = ? AND state = 'queued'", revisionID).Count(&queued).Error; err != nil || queued != 1 {
		t.Fatalf("reindex jobs = %d, %v", queued, err)
	}
	generation, err := New(db, nil).ReadyKnowledgeSearchGeneration(context.Background(), owner, baseID, false)
	if err != nil || generation != 0 {
		t.Fatalf("unverified generation = %d, %v", generation, err)
	}
}
