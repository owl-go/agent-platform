package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestKnowledgeSearchUsesAuthorizedCurrentSources(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, reader := uuid.NewString(), uuid.NewString()
	for _, userID := range []string{owner, reader} {
		exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", userID, userID, userID, userID+"@example.test", userID)
	}
	base, publicBase, category, document := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO knowledge_bases(id,owner_user_id,name,visibility) VALUES(?,?,'Private','private')", base, owner)
	exec("INSERT INTO knowledge_bases(id,owner_user_id,platform,name,visibility) VALUES(?,?,true,'Public','public')", publicBase, owner)
	exec("INSERT INTO knowledge_categories(id,knowledge_base_id,name) VALUES(?,?,'Manual')", category, base)
	exec("INSERT INTO knowledge_documents(id,knowledge_base_id,category_id,name,source_type,normalized_source,state) VALUES(?,?,?,'guide.txt','upload','guide','ready')", document, base, category)
	oldRevision, currentRevision, failedRevision := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for number, revisionID := range []string{oldRevision, currentRevision, failedRevision} {
		state := "ready"
		if number == 2 {
			state = "failed"
		}
		exec("INSERT INTO knowledge_document_revisions(id,document_id,revision,object_key,sha256,size_bytes,content_type,state) VALUES(?,?,?,?,?,12,'text/plain',?)", revisionID, document, number+1, "knowledge/"+revisionID, strings.Repeat("a", 64), state)
	}
	bases, err := repository.ListKnowledgeBases(ctx, owner, false, false)
	if err != nil || len(bases) != 2 || (bases[0].ID != publicBase && bases[1].ID != publicBase) {
		t.Fatalf("Knowledge Base summaries = %#v, %v", bases, err)
	}
	var privateSummary domain.KnowledgeBase
	for _, item := range bases {
		if item.ID == base {
			privateSummary = item
		}
	}
	if privateSummary.DocumentCount != 1 || privateSummary.ReadyDocumentCount != 1 {
		t.Fatalf("private Knowledge Base summary = %#v", privateSummary)
	}
	if _, err := repository.CreateWorkflow(ctx, owner, domain.WorkflowInput{Name: "Grounded", Goal: "Use ready knowledge", KnowledgeBaseIDs: []string{base}}, nil); err != nil {
		t.Fatalf("create Workflow with retrieval-ready Knowledge Base: %v", err)
	}
	if _, err := repository.CreateWorkflow(ctx, owner, domain.WorkflowInput{Name: "Unready", Goal: "Reject empty knowledge", KnowledgeBaseIDs: []string{publicBase}}, nil); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("create Workflow with empty Knowledge Base = %v", err)
	}
	if _, err := repository.ReadyKnowledgeSearchGeneration(ctx, owner, base, false); err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO knowledge_index_generations(knowledge_base_id,generation,state) VALUES(?,1,'ready'), (?,2,'ready')", base, base)
	generation, err := repository.ReadyKnowledgeSearchGeneration(ctx, owner, base, false)
	if err != nil || generation != 2 {
		t.Fatalf("ready generation = %d, %v", generation, err)
	}
	if _, err := repository.ReadyKnowledgeSearchGeneration(ctx, reader, base, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private Knowledge Base was accessible: %v", err)
	}
	if _, err := repository.ReadyKnowledgeSearchGeneration(ctx, reader, publicBase, false); err != nil {
		t.Fatalf("public Knowledge Base was inaccessible: %v", err)
	}
	for _, revisionID := range []string{oldRevision, failedRevision} {
		if _, err := repository.ResolveKnowledgeSearchSource(ctx, owner, base, revisionID, false); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("stale or failed revision %s was cited: %v", revisionID, err)
		}
	}
	if _, err := repository.ResolveKnowledgeSearchSource(ctx, reader, base, currentRevision, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("reader accessed private source: %v", err)
	}
	source, err := repository.ResolveKnowledgeSearchSource(ctx, owner, base, currentRevision, false)
	if err != nil || source.DocumentName != "guide.txt" || source.CategoryName != "Manual" || source.DocumentID != document {
		t.Fatalf("current source = %#v, %v", source, err)
	}
	name, retained, err := repository.GetKnowledgeDocumentRevisionSource(ctx, owner, base, document, oldRevision, false)
	if err != nil || name != "guide.txt" || retained.ID != oldRevision || retained.ObjectKey != "knowledge/"+oldRevision {
		t.Fatalf("retained source = %q, %#v, %v", name, retained, err)
	}
	if _, _, err := repository.GetKnowledgeDocumentRevisionSource(ctx, reader, base, document, oldRevision, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("reader downloaded private retained source: %v", err)
	}
	exec("UPDATE knowledge_categories SET deleted_at = now() WHERE id = ?", category)
	if _, err := repository.ResolveKnowledgeSearchSource(ctx, owner, base, currentRevision, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted Category source was cited: %v", err)
	}
	if _, _, err := repository.GetKnowledgeDocumentRevisionSource(ctx, owner, base, document, oldRevision, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted Category retained source remained downloadable: %v", err)
	}
	exec("UPDATE knowledge_categories SET deleted_at = NULL WHERE id = ?", category)
	exec("UPDATE knowledge_documents SET deleted_at = now() WHERE id = ?", document)
	if _, err := repository.ResolveKnowledgeSearchSource(ctx, owner, base, currentRevision, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted Document source was cited: %v", err)
	}
}
