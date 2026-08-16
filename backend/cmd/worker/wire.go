//go:build wireinject

package main

import (
	"context"
	"log/slog"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"
	executionwiring "agent-platform/backend/internal/wiring/execution"
	workerwiring "agent-platform/backend/internal/wiring/workerprocess"
	workflowwiring "agent-platform/backend/internal/wiring/workflow"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

func initializeWorker(context.Context, platformconfig.Config, *gormdb.Database, *slog.Logger) (*kratos.App, error) {
	wire.Build(
		workflowwiring.ProviderSet,
		executionwiring.ProviderSet,
		workerwiring.ProviderSet,
	)
	return nil, nil
}
