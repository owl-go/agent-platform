package workspace

import (
	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapp "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapp "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"net/http"
	"testing"
)

type renewalHTTPRepo struct {
	workspaceapp.Repository
	connectorPackageRepository
	locked   bool
	released int
}

func (r *renewalHTTPRepo) LockConnectorAuthorizationRefresh(context.Context, string, string, string) (func(), bool, error) {
	return func() { r.released++ }, r.locked, nil
}
func (r *renewalHTTPRepo) ListConnectorAuthorizations(context.Context, string, string) ([]domain.ConnectorAuthorization, error) {
	return []domain.ConnectorAuthorization{{ID: "grant", Version: 2}}, nil
}
func TestManualConnectorRefreshRejectsStaleVersionBeforeProvider(t *testing.T) {
	for _, locked := range []bool{true, false} {
		repo := &renewalHTTPRepo{locked: locked}
		app, err := workspaceapp.New(repo)
		if err != nil {
			t.Fatal(err)
		}
		svc := &Service{accounts: &accountapp.Service{}, workspace: app}
		ctx := accountapp.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "owner"})
		_, err = svc.RefreshConnectorAuthorization(ctx, &workspacev1.RefreshConnectorAuthorizationRequest{InstallationId: "installation", AuthorizationId: "grant", ExpectedVersion: 1})
		if kratoserrors.Code(err) != http.StatusPreconditionFailed {
			t.Fatalf("stale/concurrent refresh accepted: %v", err)
		}
		if locked && repo.released != 1 || !locked && repo.released != 0 {
			t.Fatal("refresh lock cleanup incorrect")
		}
		// No provider, Cipher or policy repository is configured: proceeding past
		// this boundary would panic and could consume a rotated refresh token.
	}
}
