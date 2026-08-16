package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/artifact/domain"
	"agent-platform/backend/internal/biz/authz"
	"agent-platform/backend/internal/objectstore"

	"github.com/google/uuid"
)

type Service struct {
	repository domain.Repository
	objects    objectstore.Provider
	retention  time.Duration
	now        func() time.Time
}

func New(repository domain.Repository, objects objectstore.Provider, retention time.Duration) (*Service, error) {
	if repository == nil || objects == nil || retention <= 0 {
		return nil, fmt.Errorf("Artifact Repository, Object Store, and positive retention are required")
	}
	return &Service{repository: repository, objects: objects, retention: retention, now: time.Now}, nil
}

func (service *Service) RecordRuntimeOutput(ctx context.Context, runID, stream string, object objectstore.Object) error {
	kind := "runtime_" + stream
	if stream != "stdout" && stream != "stderr" {
		return fmt.Errorf("unsupported Runtime output stream %q", stream)
	}
	now := service.now().UTC()
	artifact, err := domain.New(uuid.NewString(), runID, kind, object.Key, object.Size, object.SHA256, object.ContentType, object.Metadata, now.Add(service.retention), now)
	if err != nil {
		return err
	}
	return service.repository.Create(ctx, artifact)
}

func (service *Service) Get(ctx context.Context, artifactID string) (domain.Artifact, error) {
	if strings.TrimSpace(artifactID) == "" {
		return domain.Artifact{}, fmt.Errorf("Artifact ID is required")
	}
	return service.repository.Get(ctx, artifactID)
}

func (service *Service) GetInScope(ctx context.Context, artifactID string, scope authz.ReadScope) (domain.Artifact, error) {
	if strings.TrimSpace(artifactID) == "" || !scope.Valid() {
		return domain.Artifact{}, fmt.Errorf("Artifact ID and authorized read scope are required")
	}
	return service.repository.GetInScope(ctx, artifactID, scope)
}

func (service *Service) ListByRun(ctx context.Context, runID string) ([]domain.Artifact, error) {
	if strings.TrimSpace(runID) == "" {
		return nil, fmt.Errorf("Run ID is required")
	}
	return service.repository.ListByRun(ctx, runID)
}

func (service *Service) PresignDownload(ctx context.Context, artifact domain.Artifact, expiresIn time.Duration) (objectstore.SignedURL, error) {
	if artifact.DeletedAt != nil {
		return objectstore.SignedURL{}, domain.ErrNotFound
	}
	if expiresIn <= 0 || expiresIn > 15*time.Minute {
		return objectstore.SignedURL{}, fmt.Errorf("Artifact download expiry must be between zero and 15 minutes")
	}
	return service.objects.PresignGet(ctx, artifact.ObjectKey, expiresIn)
}
