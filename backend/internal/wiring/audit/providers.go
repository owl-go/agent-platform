package audit

import (
	"agent-platform/backend/internal/biz/audit/application"
	"agent-platform/backend/internal/data/audit/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database) *application.Service {
	return application.New(gormrepo.New(database.ORM()))
}
