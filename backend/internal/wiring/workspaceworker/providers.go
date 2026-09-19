package workspaceworker

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	aicreationapplication "agent-platform/backend/internal/biz/aicreation/application"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/cliconnector"
	creditsrepo "agent-platform/backend/internal/data/credits/gormrepo"
	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/data/workspace/runtimeexecutor"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"agent-platform/backend/internal/knowledgebase/ingestion"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
	workerserver "agent-platform/backend/internal/server/worker"
	aicreationwiring "agent-platform/backend/internal/wiring/aicreation"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewWarmManager, NewWorker, NewServers, NewApp)

func NewWarmManager(config platformconfig.Config) (*containerprocess.WarmManager, error) {
	return containerprocess.NewWarmManager("docker", config.Worker.RuntimeIdleTimeout.Value())
}

type Worker struct {
	workspace  *workspaceapplication.Worker
	aicreation *aicreationapplication.Service
	ingestion  *ingestion.Processor
}

func (worker *Worker) ProcessNext(ctx context.Context) (bool, error) {
	if worker.ingestion != nil {
		worked, err := worker.ingestion.ProcessNext(ctx)
		if err != nil || worked {
			return worked, err
		}
	}
	worked, err := worker.workspace.ProcessNext(ctx)
	if err != nil || worked {
		return worked, err
	}
	return worker.aicreation.ProcessNext(ctx)
}

func (worker *Worker) CleanupExpiredAIContent(ctx context.Context) (bool, error) {
	removed, err := worker.aicreation.CleanupExpired(ctx)
	return removed > 0, err
}

func NewWorker(database *gormdb.Database, config platformconfig.Config, objects objectstore.Provider, warm *containerprocess.WarmManager) (*Worker, error) {
	box, err := secretcrypto.New(config.Security.DataEncryptionKey)
	if err != nil {
		return nil, err
	}
	creditsRepository := creditsrepo.New(database.ORM())
	repository := workspacerepo.New(database.ORM(), creditsRepository)
	if err := repository.EnsureSystemSkills(context.Background(), objects); err != nil {
		return nil, err
	}
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
	if err := executor.EnableCLICredentials(repository); err != nil {
		return nil, err
	}
	var knowledgeProcessor *ingestion.Processor
	if strings.TrimSpace(config.AnythingLLM.Endpoint) != "" {
		provider, providerErr := anythingllm.NewClient(config.AnythingLLM.Endpoint, config.AnythingLLM.APIKey, config.AnythingLLM.Timeout.Value())
		if providerErr != nil {
			return nil, providerErr
		}
		if err := executor.EnableKnowledgeRetrieval(provider); err != nil {
			return nil, err
		}
		knowledgeProcessor, err = ingestion.New(repository, objects, provider)
		if err != nil {
			return nil, err
		}
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
	workspaceWorker, err := workspaceapplication.NewWorker(repository, executor, connectorBuilder)
	if err != nil {
		return nil, err
	}
	aicreation, err := aicreationwiring.NewApplication(database, creditsRepository, credits, box, objects)
	if err != nil {
		return nil, err
	}
	return &Worker{workspace: workspaceWorker, aicreation: aicreation, ingestion: knowledgeProcessor}, nil
}

func newCLIConnectorBuilder(config platformconfig.Config, objects objectstore.Provider) (*cliconnector.Builder, error) {
	if !config.Worker.CLIBuilder.Enabled {
		return nil, nil
	}
	buildEnvironment, err := cliconnector.NewDockerBuildEnvironment(cliconnector.DockerBuildConfig{
		DockerCommand: "docker", Runtime: config.Sandbox.Runtime, ImageDigest: config.Worker.CLIBuilder.ImageDigest,
		EgressNetwork: config.Worker.CLIBuilder.EgressNetwork, ResolverConfig: config.Sandbox.ResolverConfig,
		TempRoot: filepath.Join(config.Worker.CredentialTempRoot, "cli-build"),
		UID:      config.Worker.SandboxUID, GID: config.Worker.SandboxGID, Timeout: config.Worker.CLIBuilder.Timeout.Value(),
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
	conformance, err := cliconnector.NewDockerConformance(cliconnector.DockerConformanceConfig{DockerCommand: "docker", Runtime: config.Sandbox.Runtime, TempRoot: filepath.Join(config.Worker.CredentialTempRoot, "cli-conformance"), RuntimeImages: runtimeImages, UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID, Timeout: conformanceTimeout}, nil)
	if err != nil {
		return nil, err
	}
	runtimeDigests := make([]string, 0, len(runtimeImages))
	for digest := range runtimeImages {
		runtimeDigests = append(runtimeDigests, digest)
	}
	slices.Sort(runtimeDigests)
	return &cliconnector.Builder{Packages: packages, Uploads: cliconnector.ZIPPackageBuilder{}, Store: store, Sources: store, Conformance: conformance, RuntimeDigests: runtimeDigests}, nil
}

func NewServers(database *gormdb.Database, worker *Worker, warm *containerprocess.WarmManager, config platformconfig.Config) ([]transport.Server, error) {
	state := workerserver.NewState()
	interval := config.Worker.PollInterval.Value()
	if interval <= 0 {
		interval = 2 * time.Second
	}
	const executionConcurrency = 5
	loops := make([]*workerserver.Loop, 0, executionConcurrency)
	for index := 0; index < executionConcurrency; index++ {
		name := "agent-workspace-execution"
		if index > 0 {
			name = fmt.Sprintf("agent-workspace-execution-%d", index+1)
		}
		loop, loopErr := workerserver.NewLoopWithState(name, interval, workerserver.FatalAfterConsecutiveFailures(worker.ProcessNext, 10), state)
		if loopErr != nil {
			return nil, loopErr
		}
		loops = append(loops, loop)
	}
	reaper, err := workerserver.NewLoopWithState("warm-runtime-container-reaper", time.Minute, func(ctx context.Context) (bool, error) {
		removed, reapErr := warm.Reap(ctx)
		return removed > 0, reapErr
	}, state)
	if err != nil {
		return nil, err
	}
	contentReaper, err := workerserver.NewLoopWithState("ai-creation-content-reaper", time.Minute, workerserver.FatalAfterConsecutiveFailures(worker.CleanupExpiredAIContent, 10), state)
	if err != nil {
		return nil, err
	}
	management, err := workerserver.NewManagementServer(config.Worker.ManagementAddress, database, state)
	if err != nil {
		return nil, err
	}
	servers := make([]transport.Server, 0, len(loops)+3)
	for _, loop := range loops {
		servers = append(servers, loop)
	}
	return append(servers, management, reaper, contentReaper), nil
}

func NewApp(ctx context.Context, config platformconfig.Config, logger *slog.Logger, servers []transport.Server) *kratos.App {
	return kratos.New(
		kratos.Context(ctx), kratos.Name("agent-workspace-worker"), kratos.Version("dev"), kratos.Logger(logger),
		kratos.StopTimeout(config.Worker.ShutdownTimeout.Value()), kratos.Server(servers...),
	)
}
