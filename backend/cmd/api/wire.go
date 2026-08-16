//go:build wireinject

package main

import (
	"context"
	"log/slog"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/platformconfig"
	platformserver "agent-platform/backend/internal/server"
	platformservice "agent-platform/backend/internal/service/platform"
	agentwiring "agent-platform/backend/internal/wiring/agentlifecycle"
	apiwiring "agent-platform/backend/internal/wiring/apiprocess"
	approvalwiring "agent-platform/backend/internal/wiring/approval"
	artifactwiring "agent-platform/backend/internal/wiring/artifact"
	auditwiring "agent-platform/backend/internal/wiring/audit"
	collaborationwiring "agent-platform/backend/internal/wiring/collaboration"
	executionwiring "agent-platform/backend/internal/wiring/execution"
	identitywiring "agent-platform/backend/internal/wiring/identity"
	modelwiring "agent-platform/backend/internal/wiring/modelcatalog"
	platformwiring "agent-platform/backend/internal/wiring/platform"
	runtimewiring "agent-platform/backend/internal/wiring/runtimecatalog"
	sourcewiring "agent-platform/backend/internal/wiring/sourcecontrol"
	workflowwiring "agent-platform/backend/internal/wiring/workflow"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

func initializeAPI(context.Context, platformconfig.Config, *gormdb.Database, *slog.Logger) (*kratos.App, error) {
	wire.Build(
		workflowwiring.ProviderSet,
		executionwiring.ProviderSet,
		identitywiring.ProviderSet,
		approvalwiring.ProviderSet,
		runtimewiring.ProviderSet,
		modelwiring.ProviderSet,
		sourcewiring.ProviderSet,
		agentwiring.ProviderSet,
		collaborationwiring.ProviderSet,
		auditwiring.ProviderSet,
		artifactwiring.ProviderSet,
		platformwiring.APIProviderSet,
		apiwiring.ProviderSet,
		platformservice.New,
		platformserver.NewHTTPServerFromConfig,
		platformserver.NewAPIApp,
		wire.Bind(new(platformservice.ReadinessChecker), new(*gormdb.Database)),
	)
	return nil, nil
}
