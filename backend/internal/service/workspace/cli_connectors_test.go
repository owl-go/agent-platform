package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type cliCatalogRepository struct {
	workspaceapplication.Repository
	cliConnectorRepository
	items                []cliconnector.Definition
	deletedID            string
	deletedVersion       int64
	enabledOwner         string
	enabledID            string
	action               cliconnector.ActionRequirement
	providerURL          string
	authorizationAttempt cliconnector.AuthorizationAttempt
	resetActionURLCount  int
}

func (repository *cliCatalogRepository) ListConnectorActions(_ context.Context, ownerID string, _ time.Time) ([]cliconnector.ActionRequirement, error) {
	if repository.action.OwnerID != ownerID {
		return nil, nil
	}
	item := repository.action
	return []cliconnector.ActionRequirement{item}, nil
}

func (repository *cliCatalogRepository) GetConnectorAction(_ context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	if repository.action.OwnerID != ownerID || repository.action.ID != id {
		return cliconnector.ActionRequirement{}, workspacedomain.ErrNotFound
	}
	return repository.action, nil
}

func (repository *cliCatalogRepository) ResetConnectorActionProviderURL(_ context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	item, err := repository.GetConnectorAction(context.Background(), ownerID, id)
	repository.resetActionURLCount++
	item.ActionURLToken = "action-token"
	return item, err
}

func (repository *cliCatalogRepository) GetCLIConnectorAuthorizationAttemptForEnablement(_ context.Context, ownerID, enablementID string) (cliconnector.AuthorizationAttempt, error) {
	if repository.authorizationAttempt.OwnerID != ownerID || repository.authorizationAttempt.EnablementID != enablementID {
		return cliconnector.AuthorizationAttempt{}, workspacedomain.ErrNotFound
	}
	return repository.authorizationAttempt, nil
}

func (repository *cliCatalogRepository) ConsumeConnectorActionProviderURL(_ context.Context, id string, expectedVersion int64, token string) (string, error) {
	if repository.action.ID != id || repository.action.Version != expectedVersion || token != "action-token" || repository.providerURL == "" {
		return "", workspacedomain.ErrNotFound
	}
	value := repository.providerURL
	repository.providerURL = ""
	return value, nil
}

func (repository *cliCatalogRepository) DeleteCLIConnectorDefinition(_ context.Context, id string, version int64) error {
	repository.deletedID, repository.deletedVersion = id, version
	return nil
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

func (repository *cliCatalogRepository) GetAvailableCLIConnectorDefinition(_ context.Context, id string) (cliconnector.Definition, error) {
	for _, item := range repository.items {
		if item.ID == id && item.State == cliconnector.StateAvailable {
			return item, nil
		}
	}
	return cliconnector.Definition{}, workspacedomain.ErrNotFound
}

func (repository *cliCatalogRepository) EnableCLIConnector(_ context.Context, ownerID, id string) (cliconnector.Enablement, error) {
	repository.enabledOwner, repository.enabledID = ownerID, id
	return cliconnector.Enablement{ID: "enablement-1", OwnerID: ownerID, DefinitionID: id, State: "enabled", Version: 1}, nil
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

func TestAdministratorCannotDecideCommandApproval(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	_, err := service.DecideCommandApproval(ctx, &workspacev1.DecideCommandApprovalRequest{ApprovalId: "approval-1", Decision: "approved", ExpectedVersion: 1})
	if code := kratoserrors.Code(err); code != http.StatusForbidden {
		t.Fatalf("Administrator approval code = %d, want %d", code, http.StatusForbidden)
	}
}

func TestAdministratorCannotEnableCLIConnector(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	_, err := service.EnableCLIConnector(ctx, &workspacev1.EnableCLIConnectorRequest{DefinitionId: "definition-1"})
	if code := kratoserrors.Code(err); code != http.StatusForbidden {
		t.Fatalf("Administrator enablement code = %d, want %d", code, http.StatusForbidden)
	}
}

func TestEnablingProtectedConnectorDoesNotStartAuthorization(t *testing.T) {
	repository := &cliCatalogRepository{items: []cliconnector.Definition{{ID: "definition-1", Name: "Feishu CLI", State: cliconnector.StateAvailable, AuthenticationDriver: "feishu"}}}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user-1"})
	response, err := service.EnableCLIConnector(ctx, &workspacev1.EnableCLIConnectorRequest{DefinitionId: "definition-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.State != "enabled" || response.ActionUrl != nil || repository.enabledOwner != "user-1" || repository.enabledID != "definition-1" {
		t.Fatalf("enablement=%#v repository=%#v", response, repository)
	}
}

func TestConversationAuthorizationUsesOwnerCheckedPlatformURL(t *testing.T) {
	action := cliconnector.ActionRequirement{ID: "action-1", OwnerID: "user-1", State: cliconnector.ActionPending, ExpiresAt: time.Now().Add(time.Minute), Version: 1}
	repository := &cliCatalogRepository{action: action, providerURL: "https://open.feishu.cn/open-apis/authen/v1/authorize?secret=value"}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	platformAction := connectorActionResponseWithPlatformURL(cliconnector.ActionRequirement{ID: action.ID, Version: 1, ActionURL: repository.providerURL, ActionURLToken: "action-token"})
	if platformAction.ActionUrl == nil || strings.Contains(*platformAction.ActionUrl, "secret") || !strings.Contains(*platformAction.ActionUrl, "token=action-token") {
		t.Fatalf("platform action = %#v", platformAction)
	}

	authentication, err := NewAuthenticationFilter(service.accounts, application)
	if err != nil {
		t.Fatal(err)
	}
	server := kratoshttp.NewServer(kratoshttp.Filter(authentication))
	service.RegisterHTTP(server)
	request := httptest.NewRequest(http.MethodGet, *platformAction.ActionUrl, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "https://open.feishu.cn/open-apis/authen/v1/authorize?secret=value" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("status=%d headers=%v", response.Code, response.Header())
	}
	replayed := httptest.NewRequest(http.MethodGet, *platformAction.ActionUrl, nil)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, replayed)
	if response.Code != http.StatusNotFound {
		t.Fatalf("replayed URL status=%d", response.Code)
	}

	other := httptest.NewRequest(http.MethodGet, "/api/v1/connector-actions/action-1/open?version=1&token=wrong", nil)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, other)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-owner status=%d", response.Code)
	}
}

func TestStartingConnectorActionReusesCompatibleAuthorizationAttempt(t *testing.T) {
	action := cliconnector.ActionRequirement{
		ID: "action-1", OwnerID: "user-1", State: cliconnector.ActionPending,
		Reason: cliconnector.ReasonAuthorizationRequired, Identity: cliconnector.IdentityUser,
		EnablementID: "enablement-1", Permissions: []string{"chat:read"},
		ExpiresAt: time.Now().Add(time.Minute), Version: 1,
	}
	repository := &cliCatalogRepository{
		action: action,
		authorizationAttempt: cliconnector.AuthorizationAttempt{
			ID: "attempt-1", OwnerID: action.OwnerID, EnablementID: action.EnablementID,
			Identity: cliconnector.IdentityUser, Scopes: []string{"chat:read", "offline_access"},
			ActionURL: "https://open.feishu.cn/authorize?secret=value", ExpiresAt: time.Now().Add(time.Minute),
		},
	}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: action.OwnerID})
	response, err := service.StartConnectorAction(ctx, &workspacev1.StartConnectorActionRequest{ActionId: action.ID})
	if err != nil {
		t.Fatal(err)
	}
	if repository.resetActionURLCount != 1 || response.ActionUrl == nil || !strings.Contains(*response.ActionUrl, "token=action-token") || strings.Contains(*response.ActionUrl, "secret") {
		t.Fatalf("response=%#v reset count=%d", response, repository.resetActionURLCount)
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
