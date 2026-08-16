package modelcatalog

import (
	"agent-platform/backend/internal/biz/modelcatalog/application"
	"agent-platform/backend/internal/data/modelcatalog/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database) *application.Service {
	return application.New(gormrepo.New(database.ORM()))
}
