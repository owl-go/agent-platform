package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/objectstore/memory"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

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

func TestUserCannotListAggregateCLIConnectorHealth(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "user-1"})
	_, err := service.ListCLIConnectorHealth(ctx, &workspacev1.ListCLIConnectorHealthRequest{})
	if code := kratoserrors.Code(err); code != http.StatusForbidden {
		t.Fatalf("User aggregate health code = %d, want %d", code, http.StatusForbidden)
	}
}
