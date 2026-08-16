package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/domain"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type fixedIDs string

func (ids fixedIDs) NewID() string { return string(ids) }

type repositoryStub struct {
	image           domain.RuntimeImage
	created         *domain.RuntimeImage
	updated         *domain.RuntimeImage
	expectedVersion int64
}

func (repository *repositoryStub) Create(_ context.Context, image domain.RuntimeImage) error {
	repository.created = &image
	repository.image = image
	return nil
}
func (repository *repositoryStub) Get(context.Context, string) (domain.RuntimeImage, error) {
	if repository.image.ID == "" {
		return domain.RuntimeImage{}, domain.ErrRuntimeImageNotFound
	}
	return repository.image, nil
}
func (repository *repositoryStub) List(context.Context) ([]domain.RuntimeImage, error) {
	return []domain.RuntimeImage{repository.image}, nil
}
func (repository *repositoryStub) UpdateStatus(_ context.Context, image domain.RuntimeImage, expectedVersion int64) error {
	repository.updated = &image
	repository.expectedVersion = expectedVersion
	repository.image = image
	return nil
}

func TestServiceRegistersAndUpdatesRuntimeImage(t *testing.T) {
	now := time.Now().UTC()
	repository := &repositoryStub{}
	service := NewWithDependencies(repository, fixedClock(now), fixedIDs("image-1"))
	image, err := service.Register(context.Background(), RegisterCommand{
		Runtime: domain.Claude, CLIVersion: "1", AdapterVersion: "1",
		ImageDigest:  "registry.example/claude@sha256:" + strings.Repeat("a", 64),
		Capabilities: map[string]bool{"streaming": true},
	})
	if err != nil || repository.created == nil || image.ID != "image-1" {
		t.Fatalf("Register() = (%+v, %v), created=%+v", image, err, repository.created)
	}
	updated, err := service.ChangeStatus(context.Background(), ChangeStatusCommand{
		ID: image.ID, ExpectedVersion: 1, Status: domain.Production,
	})
	if err != nil || updated.Status != domain.Production || updated.Version != 2 || repository.expectedVersion != 1 {
		t.Fatalf("ChangeStatus() = (%+v, %v), expected version=%d", updated, err, repository.expectedVersion)
	}
	if _, err := service.ChangeStatus(context.Background(), ChangeStatusCommand{ID: image.ID, ExpectedVersion: 1, Status: domain.Blocked, BlockedReason: "CVE"}); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale ChangeStatus() error = %v", err)
	}
}
