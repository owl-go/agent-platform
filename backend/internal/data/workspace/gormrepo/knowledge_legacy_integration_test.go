package gormrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"agent-platform/backend/internal/knowledgebase/ingestion"
	"agent-platform/backend/internal/objectstore/memory"
	"github.com/google/uuid"
)

func TestLegacyAssistantKnowledgeMovesToCommonRevisionQueue(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	owner, baseID, documentID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	content := "旧智能助手文档中的检索内容"
	digest := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(digest[:])
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", []any{owner, owner, owner, owner + "@example.test", owner}},
		{"INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Legacy','private')", []any{baseID, owner}},
		{"INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,content,content_sha256,state) VALUES(?,?,'legacy.txt','upload',?,?,?,'accepted')", []any{documentID, baseID, sha, content, sha}},
		{"INSERT INTO ai_application_knowledge_jobs(id,document_id,state) VALUES(?,?,'queued')", []any{jobID, documentID}},
	} {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	objects := memory.New()
	processor, err := ingestion.NewLegacy(New(db, nil), objects)
	if err != nil {
		t.Fatal(err)
	}
	worked, err := processor.ProcessNext(ctx)
	if err != nil || !worked {
		t.Fatalf("legacy migration worked=%t err=%v", worked, err)
	}
	var revision struct {
		ObjectKey string
		State     string
	}
	if err := db.Table("knowledge_document_revisions").Where("document_id = ?", documentID).Take(&revision).Error; err != nil {
		t.Fatal(err)
	}
	if revision.State != "accepted" {
		t.Fatalf("revision state = %q", revision.State)
	}
	reader, _, err := objects.Get(ctx, revision.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	var queued, oldSucceeded int64
	if err := db.Table("knowledge_ingestion_jobs").Where("state = 'queued'").Count(&queued).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("ai_application_knowledge_jobs").Where("id = ? AND state = 'succeeded'", jobID).Count(&oldSucceeded).Error; err != nil {
		t.Fatal(err)
	}
	if queued != 1 || oldSucceeded != 1 {
		t.Fatalf("common queue=%d legacy done=%d", queued, oldSucceeded)
	}
	worked, err = processor.ProcessNext(ctx)
	if err != nil || worked {
		t.Fatalf("duplicate legacy migration worked=%t err=%v", worked, err)
	}
}

func TestFailedLegacyKnowledgeCanBeRequeued(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	owner, baseID, documentID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{"INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", []any{owner, owner, owner, owner + "@example.test", owner}},
		{"INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Legacy','private')", []any{baseID, owner}},
		{"INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,content,state) VALUES(?,?,'legacy.txt','upload','legacy','answer','failed')", []any{documentID, baseID}},
		{"INSERT INTO ai_application_knowledge_jobs(id,document_id,state) VALUES(?,?,'failed')", []any{jobID, documentID}},
	} {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO ai_application_knowledge_jobs(id,document_id,state,created_at) VALUES(?,?,'failed',now() - interval '1 day')", uuid.NewString(), documentID).Error; err != nil {
		t.Fatal(err)
	}
	if err := New(db, nil).RetryKnowledgeDocument(ctx, owner, baseID, documentID, false); err != nil {
		t.Fatal(err)
	}
	var queued int64
	if err := db.Table("ai_application_knowledge_jobs").Where("id = ? AND state = 'queued'", jobID).Count(&queued).Error; err != nil || queued != 1 {
		t.Fatalf("legacy queued = %d, %v", queued, err)
	}
}
