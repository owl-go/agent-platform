package workerprocess

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	artifactapplication "agent-platform/backend/internal/biz/artifact/application"
	"agent-platform/backend/internal/biz/execution/application"
	retentionapplication "agent-platform/backend/internal/biz/retention/application"
	retentiondomain "agent-platform/backend/internal/biz/retention/domain"
	webhookapplication "agent-platform/backend/internal/biz/webhook/application"
	"agent-platform/backend/internal/credentials"
	artifactgorm "agent-platform/backend/internal/data/artifact/gormrepo"
	"agent-platform/backend/internal/data/artifact/outputsink"
	"agent-platform/backend/internal/data/controlplane/gormuow"
	"agent-platform/backend/internal/data/controlplane/runtimeapproval"
	"agent-platform/backend/internal/data/execution/runtimeprocessor"
	"agent-platform/backend/internal/data/retention/dockervolume"
	retentiongorm "agent-platform/backend/internal/data/retention/gormrepo"
	webhookgorm "agent-platform/backend/internal/data/webhook/gormrepo"
	"agent-platform/backend/internal/data/webhook/httpdelivery"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/aliyunoss"
	minioadapter "agent-platform/backend/internal/objectstore/minio"
	"agent-platform/backend/internal/objectstore/providerfactory"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/sandbox"
	secretfilesystem "agent-platform/backend/internal/secretstore/filesystem"
	workerserver "agent-platform/backend/internal/server/worker"

	kratos "github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/google/wire"
	"gorm.io/gorm"
)

var ProviderSet = wire.NewSet(NewServers, NewApp)

func NewServers(database *gormdb.Database, runs *application.Service, config platformconfig.Config) ([]transport.Server, error) {
	const fatalFailureThreshold = 10
	servers := make([]transport.Server, 0, 5)
	state := workerserver.NewState()

	reconcileProcess := workerserver.FatalAfterConsecutiveFailures(func(ctx context.Context) (bool, error) {
		result, err := runs.ReconcileExpired(ctx, config.Worker.MaxAttempts)
		return result.Rescheduled > 0 || result.Failed > 0, err
	}, fatalFailureThreshold)
	reconcile, err := workerserver.NewLoopWithState("reconcile", config.Worker.ReconcileInterval.Value(), reconcileProcess, state)
	if err != nil {
		return nil, err
	}
	servers = append(servers, reconcile)

	if config.Worker.ExecutionEnabled {
		execution, err := newExecutionWorker(database.ORM(), runs, config)
		if err != nil {
			return nil, err
		}
		loop, err := workerserver.NewLoopWithState("execution", config.Worker.PollInterval.Value(), workerserver.FatalAfterConsecutiveFailures(execution.ProcessNext, fatalFailureThreshold), state)
		if err != nil {
			return nil, err
		}
		servers = append(servers, loop)
	}
	if config.Webhook.Enabled {
		dispatcher, err := newWebhookDispatcher(database.ORM(), config.Webhook)
		if err != nil {
			return nil, err
		}
		loop, err := workerserver.NewLoopWithState("webhook", config.Webhook.PollInterval.Value(), workerserver.FatalAfterConsecutiveFailures(dispatcher.ProcessNext, fatalFailureThreshold), state)
		if err != nil {
			return nil, err
		}
		servers = append(servers, loop)
	}
	if config.Retention.Enabled {
		retention, err := newRetentionService(database.ORM(), config)
		if err != nil {
			return nil, err
		}
		retentionProcess := workerserver.FatalAfterConsecutiveFailures(func(ctx context.Context) (bool, error) {
			result, err := retention.Sweep(ctx)
			found := result.Artifacts > 0 || result.Workspaces > 0 || result.RunEvents > 0 || result.AuditEvents > 0 || result.IdempotencyKey > 0
			return found, err
		}, fatalFailureThreshold)
		loop, err := workerserver.NewLoopWithState("retention", config.Retention.SweepInterval.Value(), retentionProcess, state)
		if err != nil {
			return nil, err
		}
		servers = append(servers, loop)
	}
	management, err := workerserver.NewManagementServer(config.Worker.ManagementAddress, database, state)
	if err != nil {
		return nil, err
	}
	return append([]transport.Server{management}, servers...), nil
}

func NewApp(ctx context.Context, config platformconfig.Config, logger *slog.Logger, servers []transport.Server) *kratos.App {
	return kratos.New(
		kratos.Context(ctx),
		kratos.Name("agent-platform-worker"),
		kratos.Version("dev"),
		kratos.Logger(logger),
		kratos.StopTimeout(config.Worker.ShutdownTimeout.Value()),
		kratos.Server(servers...),
	)
}

func newRetentionService(database *gorm.DB, config platformconfig.Config) (*retentionapplication.Service, error) {
	provider, err := newObjectStore(config.ObjectStore)
	if err != nil {
		return nil, err
	}
	workspaceRemover, err := dockervolume.New(sandbox.CLIExecutor{})
	if err != nil {
		return nil, err
	}
	return retentionapplication.NewWithWorkspaceRemover(retentiongorm.New(database), provider, workspaceRemover, retentiondomain.Policy{
		BatchSize: config.Retention.BatchSize, RunEventPeriod: config.Retention.RunEventPeriod.Value(),
		ArtifactPeriod: config.Retention.ArtifactPeriod.Value(), WorkspacePeriod: config.Retention.WorkspacePeriod.Value(), AuditPeriod: config.Retention.AuditPeriod.Value(),
		IdempotencyGrace: config.Retention.IdempotencyGrace.Value(),
	})
}

func newWebhookDispatcher(database *gorm.DB, config platformconfig.WebhookConfig) (*webhookapplication.Dispatcher, error) {
	client := &http.Client{
		Timeout: config.RequestTimeout.Value(),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	sender, err := httpdelivery.New(client, []byte(config.SigningSecret))
	if err != nil {
		return nil, err
	}
	return webhookapplication.NewDispatcher(webhookgorm.New(database), sender, webhookapplication.Config{
		LeaseDuration: config.LeaseDuration.Value(), RetryBase: config.RetryBase.Value(),
		RetryMaximum: config.RetryMaximum.Value(), MaxAttempts: config.MaxAttempts,
	})
}

func newExecutionWorker(database *gorm.DB, runs *application.Service, config platformconfig.Config) (*application.Worker, error) {
	if err := ensurePrivateDirectory(config.Worker.CredentialTempRoot); err != nil {
		return nil, err
	}
	resolver, err := secretfilesystem.New(config.Worker.SecretStoreRoot)
	if err != nil {
		return nil, err
	}
	provider, err := newObjectStore(config.ObjectStore)
	if err != nil {
		return nil, err
	}
	artifactService, err := artifactapplication.New(artifactgorm.New(database), provider, config.Retention.ArtifactPeriod.Value())
	if err != nil {
		return nil, err
	}
	factory, err := runtimeprocessor.NewContainerFactory(runtimeprocessor.ContainerFactoryConfig{
		AdapterVersion: config.Worker.AdapterVersion, SandboxRuntime: config.Sandbox.Runtime,
		PublicEgressNetwork: config.Sandbox.EgressNetwork, ResolverConfigFile: config.Sandbox.ResolverConfig,
		UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID, OutputSinkFactory: outputsink.FactoryWithRecorder(provider, artifactService),
	})
	if err != nil {
		return nil, err
	}
	writes := gormuow.New(database)
	if config.Webhook.Enabled {
		writes = gormuow.NewWithWebhook(database, config.Webhook.TargetURL)
	}
	approvals, err := runtimeapproval.New(database, writes)
	if err != nil {
		return nil, err
	}
	processor, err := runtimeprocessor.New(runs, resolver, credentials.Materializer{
		Root:  config.Worker.CredentialTempRoot,
		Owner: &credentials.Owner{UID: config.Worker.SandboxUID, GID: config.Worker.SandboxGID},
	}, factory, approvals)
	if err != nil {
		return nil, err
	}
	return application.NewWorker(runs, processor, application.WorkerConfig{
		WorkerID: config.Worker.ID, LeaseDuration: config.Worker.LeaseDuration.Value(),
		RenewInterval: config.Worker.RenewInterval.Value(),
	})
}

func newObjectStore(config platformconfig.ObjectStoreConfig) (objectstore.Provider, error) {
	return providerfactory.New(providerfactory.Config{
		Provider: providerfactory.Name(config.Provider),
		MinIO: minioadapter.Config{
			Endpoint: config.MinIO.Endpoint, AccessKey: config.MinIO.AccessKey, SecretKey: config.MinIO.SecretKey,
			Bucket: config.MinIO.Bucket, Secure: config.MinIO.Secure,
		},
		AliyunOSS: aliyunoss.Config{
			Endpoint: config.AliyunOSS.Endpoint, AccessKey: config.AliyunOSS.AccessKeyID,
			SecretKey: config.AliyunOSS.AccessKeySecret, Bucket: config.AliyunOSS.Bucket, Prefix: config.AliyunOSS.Prefix,
		},
	})
}

func ensurePrivateDirectory(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("credential temporary root must be absolute")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create credential temporary root: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("credential temporary root must be a non-symlink directory")
	}
	return os.Chmod(path, 0o700)
}
