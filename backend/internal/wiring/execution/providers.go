package execution

import (
	"agent-platform/backend/internal/biz/execution/application"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/execution/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, transactions *gormtx.Manager) *application.Service {
	return application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(transactions))
}
