package agentlifecycle

import (
	"agent-platform/backend/internal/biz/agentlifecycle/application"
	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
	"agent-platform/backend/internal/data/agentlifecycle/draftvalidator"
	"agent-platform/backend/internal/data/agentlifecycle/gormrepo"
	"agent-platform/backend/internal/data/controlplane/releasedependency"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, bindings *sourceapplication.BindingService) *application.Service {
	drafts := draftvalidator.New(database.ORM())
	return application.New(gormrepo.New(database.ORM()), drafts, releasedependency.New(database.ORM(), drafts, bindings))
}
