package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestConnectorPublicationAndMultipleAuthorizationSelection(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	administrator, owner := uuid.NewString(), uuid.NewString()
	for _, id := range []string{administrator, owner} {
		if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id).Error; err != nil {
			t.Fatal(err)
		}
	}
	revision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{
		PackageSource: "feishu", Version: "1.0.0", Mode: domain.ConnectorModeCLI,
		PackageSHA256: strings.Repeat("a", 64), RuntimePolicy: []byte(`{"auth_mode":"oauth"}`), ObjectKey: "connectors/feishu/1.0.0/package.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	publication, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if publication.State != domain.ConnectorPublicationAvailable || publication.Version != 1 {
		t.Fatalf("unexpected publication: %#v", publication)
	}
	if _, err := repository.PublishConnectorRevision(ctx, administrator, revision.ID, 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale publication update must conflict: %v", err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "feishu", ActiveRevisionID: revision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "account-a", Scopes: []string{"contact:read"}, CredentialCiphertext: []byte("cipher-a")})
	if err != nil {
		t.Fatal(err)
	}
	installation.Version++
	second, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "account-b", Scopes: []string{"im:write"}, CredentialCiphertext: []byte("cipher-b")})
	if err != nil {
		t.Fatal(err)
	}
	items, err := repository.ListConnectorAuthorizations(ctx, owner, installation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].State != domain.ConnectorAuthorizationActive || items[1].State != domain.ConnectorAuthorizationActive {
		t.Fatalf("multiple active authorizations were not preserved: %#v", items)
	}
	installation.Version++
	selected, err := repository.SelectConnectorAuthorization(ctx, owner, installation.ID, first.ID, installation.Version)
	if err != nil {
		t.Fatal(err)
	}
	if selected.AuthorizationID != first.ID {
		t.Fatalf("selected authorization mismatch: %#v", selected)
	}
	if _, err := repository.SelectConnectorAuthorization(ctx, owner, installation.ID, second.ID, installation.Version); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale authorization selection must conflict: %v", err)
	}
}
