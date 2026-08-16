package application_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	retentionapplication "agent-platform/backend/internal/biz/retention/application"
	"agent-platform/backend/internal/biz/retention/domain"
	"agent-platform/backend/internal/objectstore"
	objectmemory "agent-platform/backend/internal/objectstore/memory"
)

type repositoryStub struct {
	artifacts  []domain.Artifact
	marked     []string
	workspaces []domain.Workspace
	purged     []string
}

func (repository *repositoryStub) ListExpiredArtifacts(context.Context, time.Time, time.Time, time.Time, int) ([]domain.Artifact, error) {
	return repository.artifacts, nil
}
func (repository *repositoryStub) ListExpiredWorkspaces(context.Context, time.Time, int) ([]domain.Workspace, error) {
	return repository.workspaces, nil
}
func (repository *repositoryStub) MarkWorkspacePurged(_ context.Context, id string, _ time.Time) (bool, error) {
	repository.purged = append(repository.purged, id)
	return true, nil
}
func (repository *repositoryStub) MarkArtifactDeleted(_ context.Context, id string, _ time.Time) (bool, error) {
	repository.marked = append(repository.marked, id)
	return true, nil
}
func (*repositoryStub) DeleteRunEvents(context.Context, time.Time, int) (int64, error) { return 3, nil }
func (*repositoryStub) DeleteAuditEvents(context.Context, time.Time, int) (int64, error) {
	return 2, nil
}
func (*repositoryStub) DeleteIdempotencyKeys(context.Context, time.Time, int) (int64, error) {
	return 1, nil
}

func TestSweepDeletesObjectBeforeMarkingMetadata(t *testing.T) {
	provider := objectmemory.New()
	contents := []byte("artifact")
	digest := sha256.Sum256(contents)
	if _, err := provider.Put(context.Background(), "runs/one/result.json", bytes.NewReader(contents), objectstore.PutOptions{
		Size: int64(len(contents)), SHA256: hex.EncodeToString(digest[:]), ContentType: "application/json",
	}); err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{artifacts: []domain.Artifact{{ID: "artifact-1", ObjectKey: "runs/one/result.json"}}}
	service, err := retentionapplication.New(repository, provider, domain.Policy{
		BatchSize: 100, RunEventPeriod: 90 * 24 * time.Hour, ArtifactPeriod: 90 * 24 * time.Hour,
		WorkspacePeriod: 30 * 24 * time.Hour, AuditPeriod: 365 * 24 * time.Hour, IdempotencyGrace: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Sweep(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Artifacts != 1 || result.RunEvents != 3 || result.AuditEvents != 2 || result.IdempotencyKey != 1 || len(repository.marked) != 1 {
		t.Fatalf("Sweep() = %+v, marked = %+v", result, repository.marked)
	}
	if _, err := provider.Stat(context.Background(), "runs/one/result.json"); err != objectstore.ErrNotFound {
		t.Fatalf("expired Artifact object remains: %v", err)
	}
}

func TestSweepPurgesExpiredWorkspaceAfterVolumeDeletion(t *testing.T) {
	repository := &repositoryStub{workspaces: []domain.Workspace{{SessionID: "session-1", Volume: "agent-platform-session-6ba7b810-9dad-11d1-80b4-00c04fd430c8"}}}
	removed := ""
	service, err := retentionapplication.NewWithWorkspaceRemover(repository, objectmemory.New(), workspaceRemoverFunc(func(_ context.Context, volume string) error {
		removed = volume
		return nil
	}), domain.Policy{
		BatchSize: 100, RunEventPeriod: 90 * 24 * time.Hour, ArtifactPeriod: 90 * 24 * time.Hour,
		WorkspacePeriod: 30 * 24 * time.Hour, AuditPeriod: 365 * 24 * time.Hour, IdempotencyGrace: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Sweep(context.Background())
	if err != nil || result.Workspaces != 1 || len(repository.purged) != 1 || removed == "" {
		t.Fatalf("Sweep() = (%+v, %v), removed=%q purged=%v", result, err, removed, repository.purged)
	}
}

type workspaceRemoverFunc func(context.Context, string) error

func (function workspaceRemoverFunc) Remove(ctx context.Context, volume string) error {
	return function(ctx, volume)
}
