package gormrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/ragflow"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"github.com/google/uuid"
)

func TestKnowledgeGenerationRetainsRevisionAndRechecksAccess(t *testing.T) {
	db := conversationTestDatabase(t)
	r := New(db, nil)
	ctx := context.Background()
	owner, other, base, document := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{owner, other} {
		exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@test", id)
	}
	exec("INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Frozen','private')", base, owner)
	exec("INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'source.txt','upload','source','accepted')", document, base)
	revisions := []string{uuid.NewString(), uuid.NewString()}
	for i, revision := range revisions {
		exec("INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,?,?,?,20,'text/plain','accepted')", revision, document, i+1, "knowledge/"+revision, strings.Repeat("a", 64))
		exec("INSERT INTO knowledge_ingestion_jobs(revision_id,idempotency_key,state) VALUES(?,?,'queued')", revision, revision)
		job, err := r.ClaimKnowledgeIngestionJob(ctx)
		if err != nil || job == nil {
			t.Fatalf("claim=%v %v", job, err)
		}
		if err = r.SaveMapping(ctx, "local", ragflow.Mapping{BaseID: base, RevisionID: revision, DatasetID: "dataset", DocumentID: "doc" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
		if err = r.FinishKnowledgeIngestionJob(ctx, *job, nil); err != nil {
			t.Fatal(err)
		}
	}
	for i, revision := range revisions {
		mappings, err := r.GenerationMappings(ctx, "local", base, int64(i+1))
		if err != nil || len(mappings) != 1 || mappings[0].RevisionID != revision {
			t.Fatalf("generation %d=%+v %v", i+1, mappings, err)
		}
		if err = r.ValidateKnowledgeGeneration(ctx, owner, base, int64(i+1)); err != nil {
			t.Fatal(err)
		}
		if _, err = r.ResolveKnowledgeGenerationSource(ctx, owner, base, revision, int64(i+1)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.ResolveKnowledgeSearchSource(ctx, owner, base, revisions[0], true); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("preview accepted superseded source: %v", err)
	}
	if err := r.ValidateKnowledgeGeneration(ctx, other, base, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private generation leaked: %v", err)
	}
	if _, err := r.GenerationMappings(ctx, "another-deployment", base, 1); err == nil {
		t.Fatal("missing provider mapping accepted")
	}
	exec("UPDATE knowledge_documents SET deleted_at = now() WHERE id = ?", document)
	if _, err := r.ResolveKnowledgeGenerationSource(ctx, owner, base, revisions[0], 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted source leaked: %v", err)
	}
	mappings, err := r.CleanupMappings(ctx, "local")
	if err != nil || len(mappings) != 0 {
		t.Fatalf("restore window not preserved: %v %v", mappings, err)
	}
	exec("UPDATE knowledge_documents SET deleted_at = now() - interval '31 days' WHERE id = ?", document)
	mappings, err = r.CleanupMappings(ctx, "local")
	if err != nil || len(mappings) != 2 {
		t.Fatalf("expired sources not cleaned: %v %v", mappings, err)
	}
}

func TestKnowledgeIngestionRecoversLeaseAndCannotReviveDeletedSource(t *testing.T) {
	db := conversationTestDatabase(t)
	r := New(db, nil)
	ctx := context.Background()
	owner, base, document, revision, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@test", owner)
	exec("INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Recovery','private')", base, owner)
	exec("INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'source.txt','upload','source','processing')", document, base)
	exec("INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,1,?,?,20,'text/plain','processing')", revision, document, "knowledge/"+revision, strings.Repeat("a", 64))
	exec("INSERT INTO knowledge_ingestion_jobs(id,revision_id,idempotency_key,state,attempts,lease_expires_at) VALUES(?,?,?,'running',1,now() - interval '1 minute')", jobID, revision, revision)
	job, err := r.ClaimKnowledgeIngestionJob(ctx)
	if err != nil || job == nil || job.Attempts != 2 {
		t.Fatalf("lease not recovered: %+v %v", job, err)
	}
	if err = r.FinishKnowledgeIngestionJob(ctx, *job, errors.New("provider unavailable")); err != nil {
		t.Fatal(err)
	}
	var state string
	db.Table("knowledge_ingestion_jobs").Where("id = ?", jobID).Select("state").Scan(&state)
	if state != "queued" {
		t.Fatalf("retry not queued: %s", state)
	}
	exec("UPDATE knowledge_ingestion_jobs SET next_attempt_at = now() WHERE id = ?", jobID)
	job, err = r.ClaimKnowledgeIngestionJob(ctx)
	if err != nil || job == nil {
		t.Fatalf("retry claim=%+v %v", job, err)
	}
	if err = r.DeleteKnowledgeDocument(ctx, owner, base, document, false); err != nil {
		t.Fatal(err)
	}
	if err = r.FinishKnowledgeIngestionJob(ctx, *job, nil); err != nil {
		t.Fatal(err)
	}
	var generations int64
	db.Table("knowledge_index_generations").Where("knowledge_base_id = ?", base).Count(&generations)
	if generations != 0 {
		t.Fatal("deleted source published a generation")
	}
	if err = r.RestoreKnowledgeDocument(ctx, owner, base, document, false); err != nil {
		t.Fatal(err)
	}
	restored, err := r.ClaimKnowledgeIngestionJob(ctx)
	if err != nil || restored == nil || restored.Attempts <= job.Attempts {
		t.Fatalf("restored job was not resumed with a new attempt: %+v %v", restored, err)
	}
	if err = r.FinishKnowledgeIngestionJob(ctx, *job, nil); err != nil {
		t.Fatal(err)
	}
	db.Table("knowledge_index_generations").Where("knowledge_base_id = ?", base).Count(&generations)
	if generations != 0 {
		t.Fatal("stale pre-deletion claim published Ready after restore")
	}
	if err = r.FinishKnowledgeIngestionJob(ctx, *restored, nil); err != nil {
		t.Fatal(err)
	}
	db.Table("knowledge_index_generations").Where("knowledge_base_id = ?", base).Count(&generations)
	if generations != 1 {
		t.Fatal("restored ingestion did not publish a generation")
	}
}

func TestKnowledgeSourceCleanupPreservesSharedRetainedKey(t *testing.T) {
	db := conversationTestDatabase(t)
	r := New(db, nil)
	ctx := context.Background()
	objects := memory.New()
	owner, base := uuid.NewString(), uuid.NewString()
	expired, retained := uuid.NewString(), uuid.NewString()
	revisionIDs := []string{uuid.NewString(), uuid.NewString()}
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@test", owner)
	exec("INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Cleanup','private')", base, owner)
	for i, id := range []string{expired, retained} {
		exec("INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'source','upload',?,'ready')", id, base, id)
		exec("INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,1,'knowledge/shared',?,4,'text/plain','ready')", revisionIDs[i], id, strings.Repeat("a", 64))
	}
	digest := sha256.Sum256([]byte("text"))
	_, err := objects.Put(ctx, "knowledge/shared", strings.NewReader("text"), objectstore.PutOptions{Size: 4, SHA256: hex.EncodeToString(digest[:]), ContentType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	exec("UPDATE knowledge_documents SET deleted_at = now() - interval '31 days' WHERE id = ?", expired)
	if worked, err := r.CleanupExpiredKnowledgeSources(ctx, objects); err != nil || worked {
		t.Fatalf("shared source removed before all references expired: %v %v", worked, err)
	}
	exec("UPDATE knowledge_documents SET deleted_at = now() - interval '31 days' WHERE id = ?", retained)
	if worked, err := r.CleanupExpiredKnowledgeSources(ctx, objects); err != nil || !worked {
		t.Fatalf("expired source cleanup=%v %v", worked, err)
	}
	if _, err := objects.Stat(ctx, "knowledge/shared"); err == nil {
		t.Fatal("expired object still present")
	}
}
