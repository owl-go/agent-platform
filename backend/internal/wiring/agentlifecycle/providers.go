package agentlifecycle

import (
	"agent-platform/backend/internal/biz/agentlifecycle/application"
	"agent-platform/backend/internal/data/agentlifecycle/draftvalidator"
	"agent-platform/backend/internal/data/agentlifecycle/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database) *application.Service {
	return application.New(gormrepo.New(database.ORM()), draftvalidator.New(database.ORM()))
}
