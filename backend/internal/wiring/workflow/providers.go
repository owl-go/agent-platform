package workflow

import (
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewManager)

func NewManager(database *gormdb.Database) *gormtx.Manager {
	return gormtx.New(database.ORM())
}
