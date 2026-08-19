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
	if _, err := service.ChangeModelStatus(context.Background(), application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: model.ID, ExpectedVersion: 2, Enabled: true,
	}); !errors.Is(err, domain.ErrInvalidCatalogInput) {
		t.Fatalf("enable with disabled Credential Profile error = %v", err)
	}
}

func TestRepositorySerializesCredentialDisableAndModelEnable(t *testing.T) {
	database := openIntegrationDatabase(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("model-catalog-race-%d", time.Now().UnixNano())
	var organizationID string
	if err := database.ORM().Raw(`INSERT INTO organizations (slug, name) VALUES (?, 'Model Catalog Race Test') RETURNING id::text`, suffix).Scan(&organizationID).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.ORM().Exec("DELETE FROM organizations WHERE id = ?", organizationID) })

	now := time.Now().UTC()
	credentialID, modelID := uuid.NewString(), uuid.NewString()
	service := application.NewWithDependencies(modelgorm.New(database.ORM()), fixedClock(now), &sequenceIDs{values: []string{credentialID, modelID}})
	credential, err := service.RegisterCredential(ctx, application.RegisterCredentialCommand{
		OrganizationID: organizationID, Name: "race-model-key", Kind: domain.ModelCredential, SecretRef: "vault://agent-platform/race-model-key",
	})
	if err != nil {
		t.Fatal(err)
	}
	model, err := service.RegisterModel(ctx, application.RegisterModelCommand{
		OrganizationID: organizationID, Name: "race-model", ModelID: "race-model-id", Endpoint: "https://models.example.test/v1", CredentialProfileID: credential.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ChangeModelStatus(ctx, application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: model.ID, ExpectedVersion: model.Version, Enabled: false,
	}); err != nil {
		t.Fatal(err)
	}

	disableTx := database.ORM().Begin()
	if disableTx.Error != nil {
		t.Fatal(disableTx.Error)
	}
	disableService := application.NewWithDependencies(modelgorm.New(disableTx), fixedClock(now.Add(time.Second)), &sequenceIDs{})
	if _, err := disableService.ChangeCredentialStatus(ctx, application.ChangeStatusCommand{
		OrganizationID: organizationID, ID: credential.ID, ExpectedVersion: credential.Version, Enabled: false,
	}); err != nil {
		disableTx.Rollback()
		t.Fatal(err)
	}

	enableResult := make(chan error, 1)
	go func() {
		_, enableErr := service.ChangeModelStatus(ctx, application.ChangeStatusCommand{
			OrganizationID: organizationID, ID: model.ID, ExpectedVersion: 2, Enabled: true,
		})
		enableResult <- enableErr
	}()
	select {
	case err := <-enableResult:
		disableTx.Rollback()
		t.Fatalf("model enable completed before Credential Profile transaction committed: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := disableTx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-enableResult:
		if !errors.Is(err, domain.ErrInvalidCatalogInput) {
			t.Fatalf("concurrent model enable error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent model enable did not finish after Credential Profile commit")
	}
	persisted, err := service.GetModel(ctx, organizationID, model.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Enabled {
		t.Fatal("Configured Model remained enabled after its Credential Profile was disabled")
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
