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

type evidenceVerifierStub struct{}

func (evidenceVerifierStub) Verify(_ context.Context, key string, _ domain.RuntimeImage) (VerifiedEvidence, error) {
	return VerifiedEvidence{Key: key, SHA256: strings.Repeat("b", 64)}, nil
}

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
func (repository *repositoryStub) Get(context.Context, string, string) (domain.RuntimeImage, error) {
	if repository.image.ID == "" {
		return domain.RuntimeImage{}, domain.ErrRuntimeImageNotFound
	}
	return repository.image, nil
}
func (repository *repositoryStub) List(_ context.Context, query domain.PageQuery) (domain.Page, error) {
	return domain.Page{Items: []domain.RuntimeImage{repository.image}, HasMore: query.After == nil}, nil
}

func TestServicePaginatesRuntimeImagesWithOpaqueTokens(t *testing.T) {
	repository := &repositoryStub{image: domain.RuntimeImage{ID: "image-1", Runtime: domain.Codex, CreatedAt: time.Now().UTC()}}
	service := NewWithDependencies(repository, fixedClock(time.Now()), fixedIDs("image-2"))

	first, err := service.List(context.Background(), ListQuery{OrganizationID: "org-1", PageSize: 1})
	if err != nil || first.NextToken == "" || len(first.Items) != 1 {
		t.Fatalf("first List() = (%+v, %v)", first, err)
	}
	second, err := service.List(context.Background(), ListQuery{OrganizationID: "org-1", PageSize: 1, Token: first.NextToken})
	if err != nil || second.NextToken != "" || len(second.Items) != 1 {
		t.Fatalf("second List() = (%+v, %v)", second, err)
	}
	for _, query := range []ListQuery{{PageSize: -1}, {OrganizationID: "org-1", PageSize: 101}, {OrganizationID: "org-1", PageSize: 1, Token: "not-a-token"}} {
		if _, err := service.List(context.Background(), query); !errors.Is(err, ErrInvalidPage) {
			t.Fatalf("List(%+v) error = %v", query, err)
		}
	}
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
	service := NewWithEvidenceDependencies(repository, evidenceVerifierStub{}, fixedClock(now), fixedIDs("image-1"))
	image, err := service.Register(context.Background(), RegisterCommand{
		OrganizationID: "org-1", Runtime: domain.Claude, CLIVersion: "1", AdapterVersion: "1",
		ImageDigest:  "registry.example/claude@sha256:" + strings.Repeat("a", 64),
		Capabilities: map[string]bool{"streaming": true},
	})
	if err != nil || repository.created == nil || image.ID != "image-1" {
		t.Fatalf("Register() = (%+v, %v), created=%+v", image, err, repository.created)
	}
	updated, err := service.ChangeStatus(context.Background(), ChangeStatusCommand{
		OrganizationID: "org-1", ID: image.ID, ExpectedVersion: 1, Status: domain.Production, ConformanceEvidenceKey: "phase-0/claude/evidence.tar",
	})
	if err != nil || updated.Status != domain.Production || updated.Version != 2 || repository.expectedVersion != 1 {
		t.Fatalf("ChangeStatus() = (%+v, %v), expected version=%d", updated, err, repository.expectedVersion)
	}
	if _, err := service.ChangeStatus(context.Background(), ChangeStatusCommand{OrganizationID: "org-1", ID: image.ID, ExpectedVersion: 1, Status: domain.Blocked, BlockedReason: "CVE"}); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale ChangeStatus() error = %v", err)
	}
}

func TestServiceDoesNotAllowProductionWithoutEvidenceVerifier(t *testing.T) {
	image, err := domain.Register(domain.Registration{
		ID: "image-1", OrganizationID: "org-1", Runtime: domain.Codex, CLIVersion: "1", AdapterVersion: "1",
		ImageDigest: "registry.example/codex@sha256:" + strings.Repeat("a", 64), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewWithDependencies(&repositoryStub{image: image}, fixedClock(time.Now()), fixedIDs("unused"))
	_, err = service.ChangeStatus(context.Background(), ChangeStatusCommand{
		OrganizationID: "org-1", ID: image.ID, ExpectedVersion: 1, Status: domain.Production,
		ConformanceEvidenceKey: "phase-0/codex/evidence.tar",
	})
	if !errors.Is(err, ErrEvidenceUnavailable) {
		t.Fatalf("ChangeStatus without verifier error = %v", err)
	}
}
