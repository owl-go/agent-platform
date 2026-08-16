package identity

import (
	"agent-platform/backend/internal/biz/identity/application"
	"agent-platform/backend/internal/data/identity/gormrepo"
	"agent-platform/backend/internal/data/identity/tokenverifier"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewAccessService)

func NewAccessService(database *gormdb.Database) (*application.AccessService, error) {
	return application.NewAccessService(tokenverifier.NewRejecting(), gormrepo.New(database.ORM()))
}
