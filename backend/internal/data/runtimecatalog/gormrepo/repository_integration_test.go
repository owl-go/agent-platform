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

func TestRepositoryRuntimeImageLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	now := time.Now().UTC()
	id := uuid.NewString()
	service := application.NewWithDependencies(runtimegorm.New(tx), fixedClock(now), fixedID(id))
	digest := "registry.example/runtime@sha256:" + strings.Repeat("b", 64)
	image, err := service.Register(context.Background(), application.RegisterCommand{
		Runtime: domain.Codex, CLIVersion: "1.2.3", AdapterVersion: "2.0.0", ImageDigest: digest,
		Capabilities: map[string]bool{"streaming": true, "usage": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(context.Background(), id)
	if err != nil || loaded.ImageDigest != digest || !loaded.Capabilities["usage"] {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	updated, err := service.ChangeStatus(context.Background(), application.ChangeStatusCommand{
		ID: id, ExpectedVersion: image.Version, Status: domain.Blocked, BlockedReason: "security advisory",
	})
	if err != nil || updated.Status != domain.Blocked || updated.Version != 2 {
		t.Fatalf("ChangeStatus() = (%+v, %v)", updated, err)
	}
	if _, err := service.ChangeStatus(context.Background(), application.ChangeStatusCommand{
		ID: id, ExpectedVersion: 1, Status: domain.Production,
	}); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale update error = %v", err)
	}
	images, err := service.List(context.Background())
	if err != nil || len(images) == 0 {
		t.Fatalf("List() = (%d, %v)", len(images), err)
	}
	duplicateService := application.NewWithDependencies(runtimegorm.New(tx), fixedClock(now), fixedID(uuid.NewString()))
	if _, err := duplicateService.Register(context.Background(), application.RegisterCommand{
		Runtime: domain.Codex, CLIVersion: "1.2.3", AdapterVersion: "2.0.0", ImageDigest: digest,
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
