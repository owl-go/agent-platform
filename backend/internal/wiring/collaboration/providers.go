package collaboration

import (
	"agent-platform/backend/internal/biz/collaboration/application"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/collaboration/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, transactions *gormtx.Manager) *application.Service {
	return application.NewWithLaunchCoordinator(gormrepo.New(database.ORM()), bizworkflow.NewLaunch(transactions))
}
