package workspaceworker

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/cliconnector"
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
	creditsRepository := creditsrepo.New(database.ORM())
	repository := workspacerepo.New(database.ORM(), creditsRepository)
	executor, err := runtimeexecutor.New(config, box, objects, warm)
	if err != nil {
		return nil, err
	}
	egress, err := cliconnector.NewUnixEgressGate(cliconnector.UnixEgressConfig{
		SocketPath: config.Sandbox.EgressControllerSocket, EgressNetwork: config.Sandbox.EgressNetwork,
		NetworkCIDR: config.Sandbox.EgressSubnet, ResolverAddresses: config.Sandbox.ResolverAddresses,
	}, nil)
	if err != nil {
		return nil, err
	}
	if err := executor.EnableCLIConnectors(egress); err != nil {
		return nil, err
	}
	if err := executor.EnableCLIApprovals(repository); err != nil {
		return nil, err
	}
	credits, err := creditsapplication.New(creditsRepository, nil)
	if err != nil {
		return nil, err
	}
	if err := executor.EnableCredits(credits); err != nil {
		return nil, err
	}
	connectorBuilder, err := newCLIConnectorBuilder(config, objects)
	if err != nil {
		return nil, err
	}
	return workspaceapplication.NewWorker(repository, executor, connectorBuilder)
}

func newCLIConnectorBuilder(config platformconfig.Config, objects objectstore.Provider) (*cliconnector.Builder, error) {
	if !config.Worker.CLIBuilder.Enabled {
		return nil, nil
	}
	buildEnvironment, err := cliconnector.NewDockerBuildEnvironment(cliconnector.DockerBuildConfig{
		DockerCommand: "docker", Runtime: config.Sandbox.Runtime, ImageDigest: config.Worker.CLIBuilder.ImageDigest,
		EgressNetwork: config.Worker.CLIBuilder.EgressNetwork, ResolverConfig: config.Sandbox.ResolverConfig,
		UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID, Timeout: config.Worker.CLIBuilder.Timeout.Value(),
	}, nil)
	if err != nil {
		return nil, err
	}
	packages, err := cliconnector.NewIsolatedPackageBuilder(buildEnvironment)
	if err != nil {
		return nil, err
	}
	store, err := cliconnector.NewArtifactStore(objects)
	if err != nil {
		return nil, err
	}
	runtimeImages := make(map[string]string)
	for _, runtime := range config.Worker.Runtimes {
		if !runtime.Available {
			continue
		}
		_, digest, ok := strings.Cut(runtime.ImageDigest, "@")
		if !ok {
			return nil, fmt.Errorf("available Runtime image has no RepoDigest")
		}
		runtimeImages[digest] = runtime.ImageDigest
	}
	conformanceTimeout := config.Worker.CLIBuilder.Timeout.Value()
	if conformanceTimeout > 5*time.Minute {
		conformanceTimeout = 5 * time.Minute
	}
	conformance, err := cliconnector.NewDockerConformance(cliconnector.DockerConformanceConfig{DockerCommand: "docker", Runtime: config.Sandbox.Runtime, RuntimeImages: runtimeImages, UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID, Timeout: conformanceTimeout}, nil)
	if err != nil {
		return nil, err
	}
	runtimeDigests := make([]string, 0, len(runtimeImages))
	for digest := range runtimeImages {
		runtimeDigests = append(runtimeDigests, digest)
	}
	slices.Sort(runtimeDigests)
	return &cliconnector.Builder{Packages: packages, Store: store, Conformance: conformance, RuntimeDigests: runtimeDigests}, nil
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
