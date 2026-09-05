package agentworkspace

import (
	"context"
	"net/http"

	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	accountrepo "agent-platform/backend/internal/data/account/gormrepo"
	"agent-platform/backend/internal/data/account/keycloak"
	"agent-platform/backend/internal/data/account/tokenverifier"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/data/workspace/modeldiscovery"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
	platformserver "agent-platform/backend/internal/server"
	workspaceservice "agent-platform/backend/internal/service/workspace"
	"agent-platform/backend/internal/skillstore"
	"agent-platform/backend/internal/workspacefs"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewTokenVerifier,
	NewIdentityProvider,
	NewAccountService,
	NewWorkspaceService,
	NewCreditsRepository,
	NewCreditsService,
	NewSecretBox,
	NewWorkspaceFiles,
	NewSkillStore,
	workspaceservice.New,
	workspaceservice.NewAuthenticationFilter,
	NewHTTPHandlers,
	wire.Bind(new(accountapplication.IdentityProvider), new(*keycloak.Provider)),
)

func NewTokenVerifier(ctx context.Context, config platformconfig.Config) (accountapplication.TokenVerifier, error) {
	if config.Authentication.Mode != "oidc" {
		return tokenverifier.Rejecting{}, nil
	}
	return tokenverifier.NewOIDC(ctx, config.Authentication, http.DefaultTransport)
}

func NewIdentityProvider(config platformconfig.Config) (*keycloak.Provider, error) {
	return keycloak.New(config.Accounts, nil)
}

func NewAccountService(ctx context.Context, config platformconfig.Config, database *gormdb.Database, verifier accountapplication.TokenVerifier, provider accountapplication.IdentityProvider) (*accountapplication.Service, error) {
	service, err := accountapplication.New(verifier, accountrepo.New(database.ORM()), provider)
	if err != nil {
		return nil, err
	}
	_, err = service.EnsureAdministrator(ctx, accountdomain.User{
		OIDCSubject: config.Accounts.BootstrapSubject, Username: config.Accounts.BootstrapUsername,
		Email: config.Accounts.BootstrapEmail, DisplayName: config.Accounts.BootstrapDisplayName,
		Administrator: true, Enabled: true,
	})
	return service, err
}

func NewCreditsRepository(database *gormdb.Database) *creditsrepo.Repository {
	return creditsrepo.New(database.ORM())
}

func NewWorkspaceService(database *gormdb.Database, credits *creditsrepo.Repository) (*workspaceapplication.Service, error) {
	return workspaceapplication.New(workspacerepo.New(database.ORM(), credits), modeldiscovery.New(nil))
}

func NewCreditsService(credits *creditsrepo.Repository) (*creditsapplication.Service, error) {
	return creditsapplication.New(credits, nil)
}

func NewSecretBox(config platformconfig.Config) (*secretcrypto.Box, error) {
	return secretcrypto.New(config.Security.DataEncryptionKey)
}

func NewWorkspaceFiles(config platformconfig.Config) (*workspacefs.Store, error) {
	return workspacefs.New(config.Workspace.Root, config.Workspace.KnownHosts)
}

func NewSkillStore(objects objectstore.Provider) (*skillstore.Store, error) {
	return skillstore.New(objects)
}

func NewHTTPHandlers(service *workspaceservice.Service, authentication kratoshttp.FilterFunc) (platformserver.HTTPHandlers, error) {
	return platformserver.NewWorkspaceHTTPHandlers(service, authentication)
}
