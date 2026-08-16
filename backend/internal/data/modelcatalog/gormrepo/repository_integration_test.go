package gormrepo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/modelcatalog/application"
	"agent-platform/backend/internal/biz/modelcatalog/domain"
	modelgorm "agent-platform/backend/internal/data/modelcatalog/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type sequenceIDs struct{ values []string }

func (ids *sequenceIDs) NewID() string {
	value := ids.values[0]
	ids.values = ids.values[1:]
	return value
}

func TestRepositoryCredentialAndModelLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	suffix := fmt.Sprintf("model-catalog-%d", time.Now().UnixNano())
	var organizationID string
	if err := tx.Raw(`INSERT INTO organizations (slug, name) VALUES (?, 'Model Catalog Test') RETURNING id::text`, suffix).Scan(&organizationID).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	credentialID, modelID := uuid.NewString(), uuid.NewString()
	service := application.NewWithDependencies(modelgorm.New(tx), fixedClock(now), &sequenceIDs{values: []string{credentialID, modelID}})
	credential, err := service.RegisterCredential(context.Background(), application.RegisterCredentialCommand{
		OrganizationID: organizationID, Name: "primary-model-key", Kind: domain.ModelCredential,
		SecretRef: "vault://agent-platform/primary-model-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	model, err := service.RegisterModel(context.Background(), application.RegisterModelCommand{
		OrganizationID: organizationID, Name: "primary", ModelID: "model-id",
		Endpoint: "https://models.example.test/v1", CredentialProfileID: credential.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Enabled || model.CredentialProfileID != credential.ID {
		t.Fatalf("Configured Model = %+v", model)
	}
	if _, err := service.GetCredential(context.Background(), uuid.NewString(), credential.ID); !errors.Is(err, domain.ErrCredentialProfileNotFound) {
		t.Fatalf("cross-Organization credential read error = %v", err)
	}
	if _, err := service.ChangeCredentialStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: credential.ID, ExpectedVersion: 1, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}
	disabledModel, err := service.GetModel(context.Background(), organizationID, model.ID)
	if err != nil {
		t.Fatal(err)
	}
	if disabledModel.Enabled || disabledModel.Version != 2 {
		t.Fatalf("credential revocation did not disable model: %+v", disabledModel)
	}
	if _, err := service.ChangeModelStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: model.ID, ExpectedVersion: 1, Enabled: true,
	}); !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("stale model update error = %v", err)
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
