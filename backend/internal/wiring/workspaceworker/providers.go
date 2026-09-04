package workspaceworker

import (
	"context"
	"log/slog"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/data/workspace/runtimeexecutor"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
	workerserver "agent-platform/backend/internal/server/worker"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewWarmManager, NewWorker, NewServers, NewApp)

func NewWarmManager(config platformconfig.Config) (*containerprocess.WarmManager, error) {
	return containerprocess.NewWarmManager("docker", config.Worker.RuntimeIdleTimeout.Value())
}

func NewWorker(database *gormdb.Database, config platformconfig.Config, objects objectstore.Provider, warm *containerprocess.WarmManager) (*workspaceapplication.Worker, error) {
	box, err := secretcrypto.New(config.Security.DataEncryptionKey)
	if err != nil {
		return nil, err
	}
	executor, err := runtimeexecutor.New(config, box, objects, warm)
	if err != nil {
		return nil, err
	}
	credits, err := creditsapplication.New(creditsrepo.New(database.ORM()), nil)
	if err != nil {
		return nil, err
	}
	if err := executor.EnableCredits(credits); err != nil {
		return nil, err
	}
	return workspaceapplication.NewWorker(workspacerepo.New(database.ORM()), executor)
}

func NewServers(database *gormdb.Database, worker *workspaceapplication.Worker, warm *containerprocess.WarmManager, config platformconfig.Config) ([]transport.Server, error) {
	state := workerserver.NewState()
	interval := config.Worker.PollInterval.Value()
	if interval <= 0 {
		interval = 2 * time.Second
	}
	loop, err := workerserver.NewLoopWithState("agent-workspace-execution", interval, workerserver.FatalAfterConsecutiveFailures(worker.ProcessNext, 10), state)
	if err != nil {
		return nil, err
	}
	reaper, err := workerserver.NewLoopWithState("warm-runtime-container-reaper", time.Minute, func(ctx context.Context) (bool, error) {
		removed, reapErr := warm.Reap(ctx)
		return removed > 0, reapErr
	}, state)
	if err != nil {
		return nil, err
	}
	management, err := workerserver.NewManagementServer(config.Worker.ManagementAddress, database, state)
	if err != nil {
		return nil, err
	}
	return []transport.Server{management, loop, reaper}, nil
}

func NewApp(ctx context.Context, config platformconfig.Config, logger *slog.Logger, servers []transport.Server) *kratos.App {
	return kratos.New(
		kratos.Context(ctx), kratos.Name("agent-workspace-worker"), kratos.Version("dev"), kratos.Logger(logger),
		kratos.StopTimeout(config.Worker.ShutdownTimeout.Value()), kratos.Server(servers...),
	)
}
