package gormrepo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/application"
	"agent-platform/backend/internal/biz/sourcecontrol/domain"
	providergorm "agent-platform/backend/internal/data/sourcecontrol/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type fixedID string

func (id fixedID) NewID() string { return string(id) }

func TestRepositorySourceControlProviderLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	suffix := fmt.Sprintf("source-control-%d", time.Now().UnixNano())
	var organizationID string
	if err := tx.Raw(`INSERT INTO organizations (slug, name) VALUES (?, 'Source Control Test') RETURNING id::text`, suffix).Scan(&organizationID).Error; err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	service := application.NewWithDependencies(providergorm.New(tx), fixedClock(time.Now().UTC()), fixedID(id))
	provider, err := service.Register(context.Background(), application.RegisterCommand{
		OrganizationID: organizationID, Name: "enterprise-gitlab", Kind: domain.GitLabSelfManaged,
		BaseURL: "https://gitlab.example.test",
	})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := service.Get(context.Background(), organizationID, provider.ID)
	if err != nil || !loaded.Enabled || loaded.Kind != domain.GitLabSelfManaged {
		t.Fatalf("Get() = (%+v, %v)", loaded, err)
	}
	if _, err := service.Get(context.Background(), uuid.NewString(), provider.ID); !errors.Is(err, domain.ErrProviderNotFound) {
		t.Fatalf("cross-Organization read error = %v", err)
	}
	disabled, err := service.ChangeStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: provider.ID, ExpectedVersion: 1, Enabled: false,
	})
	if err != nil || disabled.Enabled || disabled.Version != 2 {
		t.Fatalf("ChangeStatus() = (%+v, %v)", disabled, err)
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
