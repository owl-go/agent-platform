package gormrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/ingestion"
	"agent-platform/backend/internal/knowledgebase/ragflow"
	"agent-platform/backend/internal/knowledgebase/retrieval"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"github.com/google/uuid"
)

// The protected JSON file is outside the repository. No credentials enter
// command arguments or test logs. The test uses a disposable platform database.
func TestRAGFlowLiveKnowledgeLoop(t *testing.T) {
	path := os.Getenv("RAGFLOW_TEST_CONFIG_FILE")
	if path == "" {
		t.Skip("RAGFLOW_TEST_CONFIG_FILE is not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read protected RAGFlow test configuration")
	}
	var config ragflow.Config
	if json.Unmarshal(data, &config) != nil {
		t.Fatal("invalid RAGFlow test configuration")
	}
	config.ParseTimeout = 5 * time.Minute
	db := conversationTestDatabase(t)
	r := New(db, nil)
	objects := memory.New()
	provider, err := ragflow.New(config, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	engine, _ := retrieval.New(r, provider)
	processor, _ := ingestion.New(r, objects, provider)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
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
	exec("INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'RAGFlow conformance','private')", base, owner)
	exec("INSERT INTO knowledge_documents(id,knowledge_base_id,name,source_type,normalized_source,state) VALUES(?,?,'fixture.txt','upload','fixture','accepted')", document, base)
	revisions := []string{uuid.NewString(), uuid.NewString()}
	for i, revision := range revisions {
		text := "知识库测试规定：银河项目的售后服务期限为七天。The Galaxy project support period is seven days."
		if i == 1 {
			text = "知识库测试规定：银河项目的售后服务期限为十五天。The Galaxy project support period is fifteen days."
		}
		digest := sha256.Sum256([]byte(text))
		hash := hex.EncodeToString(digest[:])
		key := "knowledge/test/" + revision
		_, err = objects.Put(ctx, key, bytes.NewReader([]byte(text)), objectstore.PutOptions{Size: int64(len(text)), SHA256: hash, ContentType: "text/plain"})
		if err != nil {
			t.Fatal(err)
		}
		exec("INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,?,?,?,?,'text/plain','accepted')", revision, document, i+1, key, hash, len(text))
		exec("INSERT INTO knowledge_ingestion_jobs(revision_id,idempotency_key,state) VALUES(?,?,'queued')", revision, revision)
		worked, err := processor.ProcessNext(ctx)
		if err != nil || !worked {
			t.Fatalf("ingestion=%v %v", worked, err)
		}
		var state string
		db.Table("knowledge_document_revisions").Where("id = ?", revision).Select("state").Scan(&state)
		if state != "ready" {
			var failure string
			db.Table("knowledge_document_revisions").Where("id = ?", revision).Select("error").Scan(&failure)
			t.Fatalf("RAGFlow parsing failed: state=%s, reason=%s", state, failure)
		}
		defer func(revision string) {
			if err := provider.RemoveRevision(context.Background(), base, revision); err != nil {
				t.Error(err)
			}
		}(revision)
		hits, err := engine.Search(ctx, owner, base, 0, "银河项目售后服务期限是多少天？", 10, 6000)
		if err != nil || len(hits) == 0 || hits[0].Source.RevisionID != revision {
			t.Fatalf("live hit=%+v %v", hits, err)
		}
	}
	hits, err := engine.Search(ctx, owner, base, 1, "银河项目售后服务期限", 10, 6000)
	if err != nil || len(hits) == 0 || !strings.Contains(hits[0].Text, "七天") {
		t.Fatalf("frozen generation changed: %+v %v", hits, err)
	}
	if _, err = engine.Search(ctx, other, base, 0, "银河项目", 10, 6000); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unauthorized search=%v", err)
	}
	if err = r.DeleteKnowledgeDocument(ctx, owner, base, document, false); err != nil {
		t.Fatal(err)
	}
	if hits, err = engine.Search(ctx, owner, base, 0, "银河项目", 10, 6000); err != nil || len(hits) != 0 {
		t.Fatalf("deleted document leaked: %+v %v", hits, err)
	}
	if err = r.RestoreKnowledgeDocument(ctx, owner, base, document, false); err != nil {
		t.Fatal(err)
	}
	if hits, err = engine.Search(ctx, owner, base, 0, "银河项目售后服务期限", 10, 6000); err != nil || len(hits) == 0 {
		t.Fatalf("restore did not recover: %+v %v", hits, err)
	}
	// An empty indexed query result is distinct from provider failure.
	if hits, err = engine.Search(ctx, owner, base, 0, "ZXQ987654321 gravitational quasar axolotl", 10, 6000); err != nil || len(hits) != 0 {
		t.Fatalf("no-hit query failed: %v", err)
	}
	exec("UPDATE knowledge_documents SET deleted_at = now() - interval '31 days' WHERE id = ?", document)
	if err = r.RestoreKnowledgeDocument(ctx, owner, base, document, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired source restored=%v", err)
	}
	if worked, err := provider.Cleanup(ctx); err != nil || !worked {
		t.Fatalf("provider cleanup=%v %v", worked, err)
	}
	if worked, err := r.CleanupExpiredKnowledgeSources(ctx, objects); err != nil || !worked {
		t.Fatalf("source cleanup=%v %v", worked, err)
	}
	t.Log("verified real RAGFlow upload/parse/hit, replacement, frozen generation, private access, deletion/restore and expired provider/source cleanup")
}
