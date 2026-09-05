package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
)

type artifactDownloadRepository struct {
	workspaceapplication.Repository
	artifact workspacedomain.Artifact
}

func (repository *artifactDownloadRepository) GetSessionArtifact(_ context.Context, owner, sessionID, artifactID string) (workspacedomain.Artifact, error) {
	if owner != "owner-1" || sessionID != "session-1" || artifactID != repository.artifact.ID {
		return workspacedomain.Artifact{}, workspacedomain.ErrNotFound
	}
	return repository.artifact, nil
}

func (repository *artifactDownloadRepository) GetArtifact(_ context.Context, owner, workflowID, artifactID string) (workspacedomain.Artifact, error) {
	if owner != "owner-1" || workflowID != "workflow-1" || artifactID != repository.artifact.ID {
		return workspacedomain.Artifact{}, workspacedomain.ErrNotFound
	}
	return repository.artifact, nil
}

func TestArtifactDownloadsStreamAuthenticatedContent(t *testing.T) {
	provider := memory.New()
	content := []byte("generated report")
	digest := sha256.Sum256(content)
	key := "artifacts/owner-1/report"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{
		Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), ContentType: "text/markdown",
	}); err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().Add(time.Hour)
	repository := &artifactDownloadRepository{artifact: workspacedomain.Artifact{ID: "artifact-1", Kind: "file", Name: "report.md", ObjectKey: key, ExpiresAt: &expiresAt}}
	workspace, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: workspace, objects: provider}

	for _, test := range []struct {
		name string
		path string
		call func(http.ResponseWriter, *http.Request)
	}{
		{name: "Session", path: "/api/v1/sessions/session-1/artifacts/artifact-1/download", call: service.downloadSessionArtifact},
		{name: "Workflow", path: "/api/v1/workflows/workflow-1/artifacts/artifact-1/download", call: service.downloadWorkflowArtifact},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			request = request.WithContext(accountapplication.WithPrincipal(request.Context(), accountdomain.Principal{UserID: "owner-1"}))
			response := httptest.NewRecorder()

			test.call(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
			}
			if got := response.Header().Get("Content-Disposition"); got != `attachment; filename=report.md` {
				t.Fatalf("Content-Disposition = %q", got)
			}
			if !bytes.Equal(response.Body.Bytes(), content) {
				t.Fatalf("body = %q", response.Body.Bytes())
			}
		})
	}
}
