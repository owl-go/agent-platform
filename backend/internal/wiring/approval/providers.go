package approval

import (
	"agent-platform/backend/internal/biz/approval/application"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/approval/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, transactions *gormtx.Manager) *application.Service {
	return application.New(gormrepo.New(database.ORM()), bizworkflow.NewApproval(transactions))
}
