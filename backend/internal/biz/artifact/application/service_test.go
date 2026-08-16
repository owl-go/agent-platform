package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	artifactapplication "agent-platform/backend/internal/biz/artifact/application"
	"agent-platform/backend/internal/biz/artifact/domain"
	"agent-platform/backend/internal/biz/authz"
	"agent-platform/backend/internal/objectstore"
	objectmemory "agent-platform/backend/internal/objectstore/memory"
)

type repositoryStub struct{ artifacts []domain.Artifact }

func (repository *repositoryStub) Create(_ context.Context, artifact domain.Artifact) error {
	repository.artifacts = append(repository.artifacts, artifact)
	return nil
}
func (repository *repositoryStub) Get(_ context.Context, id string) (domain.Artifact, error) {
	for _, artifact := range repository.artifacts {
		if artifact.ID == id {
			return artifact, nil
		}
	}
	return domain.Artifact{}, domain.ErrNotFound
}
func (repository *repositoryStub) GetInScope(ctx context.Context, id string, _ authz.ReadScope) (domain.Artifact, error) {
	return repository.Get(ctx, id)
}
func (repository *repositoryStub) ListByRun(_ context.Context, runID string) ([]domain.Artifact, error) {
	var values []domain.Artifact
	for _, artifact := range repository.artifacts {
		if artifact.RunID == runID {
			values = append(values, artifact)
		}
	}
	return values, nil
}

func TestRecordRuntimeOutputAndPresign(t *testing.T) {
	repository := &repositoryStub{}
	provider := objectmemory.New()
	service, err := artifactapplication.New(repository, provider, 90*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("output"))
	object := objectstore.Object{Key: "runs/run-1/runtime/stdout.log", Size: 6, SHA256: hex.EncodeToString(digest[:]), ContentType: "text/plain", Metadata: map[string]string{"run_id": "run-1"}}
	if err := service.RecordRuntimeOutput(context.Background(), "run-1", "stdout", object); err != nil {
		t.Fatal(err)
	}
	if len(repository.artifacts) != 1 || repository.artifacts[0].Kind != "runtime_stdout" || repository.artifacts[0].ExpiresAt == nil {
		t.Fatalf("Artifacts = %+v", repository.artifacts)
	}
}
