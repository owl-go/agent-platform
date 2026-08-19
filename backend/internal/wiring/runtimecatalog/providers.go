package runtimecatalog

import (
	"agent-platform/backend/internal/biz/runtimecatalog/application"
	"agent-platform/backend/internal/data/runtimecatalog/evidenceverifier"
	"agent-platform/backend/internal/data/runtimecatalog/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewService)

func NewService(database *gormdb.Database, objects objectstore.Provider) *application.Service {
	return application.NewWithEvidenceVerifier(gormrepo.New(database.ORM()), evidenceverifier.New(objects))
}
