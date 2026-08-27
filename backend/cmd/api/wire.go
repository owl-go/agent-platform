//go:build wireinject

package main

import (
	"context"
	"log/slog"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"
	platformserver "agent-platform/backend/internal/server"
	platformservice "agent-platform/backend/internal/service/platform"
	agentworkspacewiring "agent-platform/backend/internal/wiring/agentworkspace"
	platformwiring "agent-platform/backend/internal/wiring/platform"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

func initializeAPI(context.Context, platformconfig.Config, *gormdb.Database, *slog.Logger) (*kratos.App, error) {
	wire.Build(
		agentworkspacewiring.ProviderSet,
		platformwiring.NewObjectStore,
		platformservice.New,
		platformserver.NewHTTPServerFromConfig,
		platformserver.NewAPIApp,
		wire.Bind(new(platformservice.ReadinessChecker), new(*gormdb.Database)),
	)
	return nil, nil
}
