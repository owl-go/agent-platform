package identity

import (
	"context"
	"net/http"

	"agent-platform/backend/internal/biz/identity/application"
	"agent-platform/backend/internal/data/identity/gormrepo"
	"agent-platform/backend/internal/data/identity/tokenverifier"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewAccessService)

func NewAccessService(config platformconfig.Config, database *gormdb.Database) (*application.AccessService, error) {
	verifier := application.TokenVerifier(tokenverifier.NewRejecting())
	if config.Authentication.Mode == "oidc" {
		configured, err := tokenverifier.NewOIDC(context.Background(), config.Authentication, http.DefaultTransport)
		if err != nil {
			return nil, err
		}
		verifier = configured
	}
	return application.NewAccessService(verifier, gormrepo.New(database.ORM()))
}
