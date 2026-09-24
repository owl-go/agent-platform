package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/objectstore/memory"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

type cliCatalogRepository struct {
	workspaceapplication.Repository
	cliConnectorRepository
	items           []cliconnector.Definition
	deletedID       string
	deletedVersion  int64
	disabledOwner   string
	disabledID      string
	disabledVersion int64
	enabledOwner    string
	enabledID       string
	approvalOwner   string
}

func (repository *cliCatalogRepository) GetAvailableCLIConnectorDefinition(_ context.Context, id string) (cliconnector.Definition, error) {
	return cliconnector.Definition{ID: id, State: cliconnector.StateAvailable, AuthenticationDriver: "none"}, nil
}

func (repository *cliCatalogRepository) EnableCLIConnector(_ context.Context, ownerID, id string) (cliconnector.Enablement, error) {
	repository.enabledOwner, repository.enabledID = ownerID, id
	return cliconnector.Enablement{ID: "enablement-1", OwnerID: ownerID, DefinitionID: id, State: "enabled", Version: 1}, nil
}

func (repository *cliCatalogRepository) DecideCommandApproval(_ context.Context, ownerID, approvalID string, state workspacedomain.ApprovalState, identity workspacedomain.ExecutionIdentity, _ int64, _ time.Time) (workspacedomain.CommandApproval, error) {
	repository.approvalOwner = ownerID
	return workspacedomain.CommandApproval{ID: approvalID, OwnerID: ownerID, State: state, Identity: identity, ExpiresAt: time.Now().Add(time.Minute), Version: 2}, nil
}

func (repository *cliCatalogRepository) DeleteCLIConnectorDefinition(_ context.Context, id string, version int64) error {
	repository.deletedID, repository.deletedVersion = id, version
	return nil
}

func (repository *cliCatalogRepository) DisableCLIConnector(_ context.Context, ownerID, id string, version int64) (cliconnector.Enablement, error) {
	repository.disabledOwner, repository.disabledID, repository.disabledVersion = ownerID, id, version
	return cliconnector.Enablement{ID: "enablement-1", OwnerID: ownerID, DefinitionID: id, State: "disabled", Version: version + 1}, nil
}

func TestCLIConnectorDeletionRequiresAdministratorAndVersion(t *testing.T) {
	repository := &cliCatalogRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	user := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user"})
	admin := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "admin", Administrator: true})
	request := &workspacev1.DeleteCLIConnectorDefinitionRequest{DefinitionId: "definition-1", ExpectedVersion: 3}
	if _, err := service.DeleteCLIConnectorDefinition(user, request); kratoserrors.Code(err) != http.StatusForbidden {
		t.Fatalf("user deletion = %v", err)
	}
	if repository.deletedID != "" {
		t.Fatal("unauthorized deletion reached repository")
	}
	if _, err := service.DeleteCLIConnectorDefinition(admin, &workspacev1.DeleteCLIConnectorDefinitionRequest{DefinitionId: request.DefinitionId}); kratoserrors.Code(err) != http.StatusUnprocessableEntity {
		t.Fatalf("unversioned deletion = %v", err)
	}
	response, err := service.DeleteCLIConnectorDefinition(admin, request)
	if err != nil || !response.Deleted || repository.deletedID != request.DefinitionId || repository.deletedVersion != 3 {
		t.Fatalf("administrator deletion failed: %v", err)
	}
}

func (repository *cliCatalogRepository) ListCLIConnectorDefinitions(context.Context, bool) ([]cliconnector.Definition, error) {
	return repository.items, nil
}

func TestAvailableCLIConnectorIsEditableForAdministrator(t *testing.T) {
	repository := &cliCatalogRepository{items: []cliconnector.Definition{{ID: "definition-1", Name: "Example CLI", State: cliconnector.StateAvailable}}}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	response, err := service.ListCLIConnectorDefinitions(ctx, &workspacev1.ListCLIConnectorDefinitionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || !response.Items[0].Mutable {
		t.Fatalf("available CLI Connector response = %#v", response.Items)
	}
}

func TestCLIUploadInputStoresValidatedImmutableSource(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	manifest, err := writer.Create("package.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = manifest.Write([]byte(`{"name":"example-cli","version":"1.2.3","bin":{"example":"cli.js"}}`))
	command, _ := writer.Create("cli.js")
	_, _ = command.Write([]byte("#!/usr/bin/env node\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	objects := memory.New()
	service := &Service{objects: objects}
	value, err := service.cliDefinitionInput(context.Background(), &workspacev1.CLIConnectorDefinitionInput{Name: "Example", Icon: "terminal", Description: "Reads examples", InstallationType: "upload", Archive: archive.Bytes()}, "11111111-1111-1111-1111-111111111111", 1)
	if err != nil {
		t.Fatal(err)
	}
	if value.SourceObjectKey == "" || value.SourceSHA256 == "" || value.InstallationType != "upload" {
		t.Fatalf("definition = %#v", value)
	}
	if _, err := objects.Stat(context.Background(), value.SourceObjectKey); err != nil {
		t.Fatal(err)
	}
}

func TestAdministratorCanDecideOwnCommandApproval(t *testing.T) {
	repository := &cliCatalogRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	identity := "user"
	response, err := service.DecideCommandApproval(ctx, &workspacev1.DecideCommandApprovalRequest{ApprovalId: "approval-1", Decision: "approved", Identity: &identity, ExpectedVersion: 1})
	if err != nil || response.State != "approved" || repository.approvalOwner != "administrator-1" {
		t.Fatalf("administrator approval = %#v, %v", response, err)
	}
}

func TestAdministratorCanEnableOwnCLIConnector(t *testing.T) {
	repository := &cliCatalogRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	response, err := service.EnableCLIConnector(ctx, &workspacev1.EnableCLIConnectorRequest{DefinitionId: "definition-1"})
	if err != nil || response.State != "enabled" || repository.enabledOwner != "administrator-1" || repository.enabledID != "definition-1" {
		t.Fatalf("administrator enablement = %#v, %v", response, err)
	}
}

func TestUserCanDisableCLIConnectorWithVersion(t *testing.T) {
	repository := &cliCatalogRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	user := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user-1"})
	admin := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "admin", Administrator: true})
	request := &workspacev1.DisableCLIConnectorRequest{DefinitionId: "definition-1", ExpectedVersion: 4}
	if response, err := service.DisableCLIConnector(admin, request); err != nil || response.State != "disabled" || repository.disabledOwner != "admin" {
		t.Fatalf("administrator disablement = %#v, %v", response, err)
	}
	if _, err := service.DisableCLIConnector(user, &workspacev1.DisableCLIConnectorRequest{DefinitionId: request.DefinitionId}); kratoserrors.Code(err) != http.StatusUnprocessableEntity {
		t.Fatalf("unversioned disablement = %v", err)
	}
	response, err := service.DisableCLIConnector(user, request)
	if err != nil || response.State != "disabled" || repository.disabledOwner != "user-1" || repository.disabledID != "definition-1" || repository.disabledVersion != 4 {
		t.Fatalf("user disablement failed: response=%#v err=%v", response, err)
	}
}

func TestUserCannotListAggregateCLIConnectorHealth(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user-1"})
	_, err := service.ListCLIConnectorHealth(ctx, &workspacev1.ListCLIConnectorHealthRequest{})
	if code := kratoserrors.Code(err); code != http.StatusForbidden {
		t.Fatalf("User aggregate health code = %d, want %d", code, http.StatusForbidden)
	}
}
