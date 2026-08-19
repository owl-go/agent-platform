package gormrepo_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/application"
	"agent-platform/backend/internal/biz/runtimecatalog/domain"
	runtimegorm "agent-platform/backend/internal/data/runtimecatalog/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type fixedID string

func (id fixedID) NewID() string { return string(id) }

type evidenceVerifierStub struct{}

func (evidenceVerifierStub) Verify(_ context.Context, key string, _ domain.RuntimeImage) (application.VerifiedEvidence, error) {
	return application.VerifiedEvidence{Key: key, SHA256: strings.Repeat("b", 64)}, nil
}

func TestRepositoryRuntimeImageLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	now := time.Now().UTC()
	organizationID := uuid.NewString()
	if err := tx.Exec("INSERT INTO organizations (id, slug, name) VALUES (?, ?, ?)", organizationID, "runtime-"+strings.ReplaceAll(organizationID, "-", ""), "Runtime Catalog Test").Error; err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	service := application.NewWithEvidenceDependencies(runtimegorm.New(tx), evidenceVerifierStub{}, fixedClock(now), fixedID(id))
	digest := "registry.example/runtime@sha256:" + strings.Repeat("b", 64)
	image, err := service.Register(context.Background(), application.RegisterCommand{
		OrganizationID: organizationID, Runtime: domain.Codex, CLIVersion: "1.2.3", AdapterVersion: "2.0.0", ImageDigest: digest,
		Capabilities: map[string]bool{"streaming": true, "usage": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(context.Background(), organizationID, id)
	if err != nil || loaded.ImageDigest != digest || !loaded.Capabilities["usage"] {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	updated, err := service.ChangeStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: id, ExpectedVersion: image.Version, Status: domain.Blocked, BlockedReason: "security advisory",
	})
	if err != nil || updated.Status != domain.Blocked || updated.Version != 2 {
		t.Fatalf("ChangeStatus() = (%+v, %v)", updated, err)
	}
	if _, err := service.ChangeStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: id, ExpectedVersion: 1, Status: domain.Production, ConformanceEvidenceKey: "phase-0/codex/evidence.tar",
	}); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale update error = %v", err)
	}
	images, err := service.List(context.Background(), application.ListQuery{OrganizationID: organizationID, PageSize: 1})
	if err != nil || len(images.Items) != 1 {
		t.Fatalf("List() = (%d, %v)", len(images.Items), err)
	}
	otherOrganizationID := uuid.NewString()
	if err := tx.Exec("INSERT INTO organizations (id, slug, name) VALUES (?, ?, ?)", otherOrganizationID, "runtime-other-"+strings.ReplaceAll(otherOrganizationID, "-", ""), "Other Runtime Catalog").Error; err != nil {
		t.Fatal(err)
	}
	otherService := application.NewWithDependencies(runtimegorm.New(tx), fixedClock(now), fixedID(uuid.NewString()))
	if _, err := otherService.Register(context.Background(), application.RegisterCommand{
		OrganizationID: otherOrganizationID, Runtime: domain.Codex, CLIVersion: "1.2.3", AdapterVersion: "2.0.0", ImageDigest: digest,
	}); err != nil {
		t.Fatalf("same digest in another Organization: %v", err)
	}
	if _, err := otherService.Get(context.Background(), otherOrganizationID, id); !errors.Is(err, domain.ErrRuntimeImageNotFound) {
		t.Fatalf("cross-Organization Get() error = %v", err)
	}
	firstOrganization, err := service.List(context.Background(), application.ListQuery{OrganizationID: organizationID, PageSize: 10})
	if err != nil || len(firstOrganization.Items) != 1 {
		t.Fatalf("Organization-scoped List() = (%d, %v)", len(firstOrganization.Items), err)
	}
	duplicateService := application.NewWithDependencies(runtimegorm.New(tx), fixedClock(now), fixedID(uuid.NewString()))
	if _, err := duplicateService.Register(context.Background(), application.RegisterCommand{
		OrganizationID: organizationID, Runtime: domain.Codex, CLIVersion: "1.2.3", AdapterVersion: "2.0.0", ImageDigest: digest,
	}); !errors.Is(err, domain.ErrImageDigestExists) {
		t.Fatalf("duplicate digest error = %v", err)
	}
}

func openIntegrationDatabase(t *testing.T) *gormdb.Database {
	t.Helper()
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("EXECUTION_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{
		DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2,
		ConnectionMaxIdle: time.Minute, ConnectionMaxLife: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
