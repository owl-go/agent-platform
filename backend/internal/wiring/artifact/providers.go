package artifact

import (
	"agent-platform/backend/internal/biz/artifact/application"
	"agent-platform/backend/internal/data/artifact/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, objects objectstore.Provider, config platformconfig.Config) (*application.Service, error) {
	return application.New(gormrepo.New(database.ORM()), objects, config.Retention.ArtifactPeriod.Value())
}
