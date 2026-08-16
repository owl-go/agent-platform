package sourcecontrol

import (
	"agent-platform/backend/internal/biz/sourcecontrol/application"
	"agent-platform/backend/internal/data/sourcecontrol/bindingvalidator"
	"agent-platform/backend/internal/data/sourcecontrol/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService, NewBindingService)

func NewService(database *gormdb.Database) *application.Service {
	return application.New(gormrepo.New(database.ORM()))
}

func NewBindingService(database *gormdb.Database) *application.BindingService {
	return application.NewBindingService(gormrepo.New(database.ORM()), bindingvalidator.New(database.ORM()))
}
