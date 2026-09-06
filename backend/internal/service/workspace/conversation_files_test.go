package workspace

import (
	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/workspacefs"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type conversationFilesRepository struct {
	application.Repository
	messages []domain.Message
}

func (r *conversationFilesRepository) ListMessages(_ context.Context, owner, session string, _ int64, _ int) ([]domain.Message, error) {
	if owner != "owner" || session != "session" {
		return nil, domain.ErrNotFound
	}
	return r.messages, nil
}
func TestConversationFileCopiesAreScopedDeduplicatedAndIndependent(t *testing.T) {
	ctx := context.Background()
	provider := memory.New()
	content := []byte("original result")
	hash := sha256.Sum256(content)
	digest := hex.EncodeToString(hash[:])
	if _, err := provider.Put(ctx, "artifacts/original", bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: digest}); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	repository := &conversationFilesRepository{messages: []domain.Message{{ID: 1, Artifacts: []domain.Artifact{{ID: "artifact", Name: "result.md", ObjectKey: "artifacts/original", Size: int64(len(content)), SHA256: digest, ExpiresAt: &expires}}}}}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, objects: provider}
	scope := domain.ConversationScope{SessionID: "session"}
	reference := &workspacev1.FileReference{Kind: "artifact", Id: "artifact"}
	items, cleanup, err := service.resolveMessageAttachments(ctx, "owner", scope, nil, []*workspacev1.FileReference{reference, reference})
	if err != nil || len(items) != 1 {
		t.Fatalf("resolve = %#v, %v", items, err)
	}
	defer cleanup()
	if err := provider.Delete(ctx, "artifacts/original"); err != nil {
		t.Fatal(err)
	}
	reader, _, err := provider.Get(ctx, items[0].ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	if !bytes.Equal(got, content) {
		t.Fatal("snapshot depends on expired source")
	}
	for _, bad := range []*workspacev1.FileReference{{Kind: "artifact", Id: "other-conversation"}, {Kind: "workspace", Path: "secret"}} {
		_, clean, err := service.resolveMessageAttachments(ctx, "owner", scope, nil, []*workspacev1.FileReference{bad})
		clean()
		if !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("cross-scope reference accepted: %v", err)
		}
	}
	expires = time.Now().Add(-time.Hour)
	_, clean, err := service.resolveMessageAttachments(ctx, "owner", scope, nil, []*workspacev1.FileReference{reference})
	clean()
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expired reference accepted: %v", err)
	}
	cleanup()
	if _, _, err := provider.Get(ctx, items[0].ObjectKey); err == nil {
		t.Fatal("failed submission copy not cleaned")
	}
}

func (r *conversationFilesRepository) ListRunTurns(_ context.Context, owner, workflow, run string) ([]domain.Run, error) {
	if owner != "owner" || workflow != "workflow" || run != "root" {
		return nil, domain.ErrNotFound
	}
	return []domain.Run{{ID: "root"}}, nil
}
func (r *conversationFilesRepository) ListArtifacts(context.Context, string, string) ([]domain.Artifact, error) {
	return nil, nil
}
func (r *conversationFilesRepository) GetWorkflow(_ context.Context, owner, workflow string, _ bool) (domain.Workflow, error) {
	if owner != "owner" || workflow != "workflow" {
		return domain.Workflow{}, domain.ErrNotFound
	}
	return domain.Workflow{WorkspacePath: "users/owner/workflows/workflow"}, nil
}
func TestWorkspaceReferenceFreezesBytesAndRejectsTraversalAndSymlinks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	files, err := workspacefs.New(root, "")
	if err != nil {
		t.Fatal(err)
	}
	workspace := "users/owner/workflows/workflow"
	if _, err = files.Upload(ctx, workspace, "report.md", []byte("accepted bytes")); err != nil {
		t.Fatal(err)
	}
	repository := &conversationFilesRepository{}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	provider := memory.New()
	service := &Service{workspace: app, objects: provider, files: files}
	scope := domain.ConversationScope{WorkflowID: "workflow", RunID: "root"}
	selected, cleanup, err := service.resolveMessageAttachments(ctx, "owner", scope, nil, []*workspacev1.FileReference{{Kind: "workspace", Path: "report.md"}})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err = files.Upload(ctx, workspace, "report.md", []byte("changed while queued")); err != nil {
		t.Fatal(err)
	}
	reader, _, err := provider.Get(ctx, selected[0].ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(reader)
	reader.Close()
	if string(content) != "accepted bytes" {
		t.Fatal("queued reference read mutable Workspace")
	}
	if err = os.Symlink(filepath.Join(root, workspace, "report.md"), filepath.Join(root, workspace, "link.md")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../outside", "link.md", "/etc/passwd"} {
		_, clean, err := service.resolveMessageAttachments(ctx, "owner", scope, nil, []*workspacev1.FileReference{{Kind: "workspace", Path: path}})
		clean()
		if !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("accepted unsafe path %q: %v", path, err)
		}
	}
}
