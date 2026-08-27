//go:build wireinject

package main

import (
	"context"
	"log/slog"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"
	platformwiring "agent-platform/backend/internal/wiring/platform"
	workerwiring "agent-platform/backend/internal/wiring/workspaceworker"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

func initializeWorker(context.Context, platformconfig.Config, *gormdb.Database, *slog.Logger) (*kratos.App, error) {
	wire.Build(
		workerwiring.ProviderSet,
		platformwiring.NewObjectStore,
	)
	return nil, nil
}
