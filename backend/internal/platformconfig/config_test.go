package platformconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadExpandsEnvironmentAndValidatesAPI(t *testing.T) {
	t.Setenv("TEST_DATABASE_DSN", `postgres://platform:p@ss:#word@database/platform`)
	path := writeConfig(t, validYAML("${TEST_DATABASE_DSN}"))

	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateAPI(); err != nil {
		t.Fatal(err)
	}
	if config.Database.DSN != `postgres://platform:p@ss:#word@database/platform` || config.API.ShutdownTimeout.Value() != 10*time.Second {
		t.Fatalf("unexpected configuration: %+v", config)
	}
}

func TestLoadRejectsUnknownFieldsAndMissingEnvironment(t *testing.T) {
	for _, fixture := range []string{
		validYAML("${MISSING_DATABASE_DSN}"),
		validYAML("postgres://database/platform") + "unknown: true\n",
		validYAML("postgres://database/platform") + "---\napi: {}\n",
	} {
		path := writeConfig(t, fixture)
		if _, err := Load(path); err == nil {
			t.Fatalf("Load accepted invalid YAML:\n%s", fixture)
		}
	}
}

func TestValidationRejectsUnsafeDefaults(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Worker.MaxAttempts = 0
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted zero attempts")
	}
	config.Database.MaxIdleConnections = config.Database.MaxOpenConnections + 1
	if err := config.ValidateAPI(); err == nil {
		t.Fatal("ValidateAPI accepted invalid connection pool")
	}
}

func TestWorkerValidationDoesNotDependOnAPIConfiguration(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.API = APIConfig{}
	config.Authentication = AuthenticationConfig{}
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker depended on API-only configuration: %v", err)
	}
}

func TestAPIValidationDoesNotDependOnWorkerConfiguration(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Worker = WorkerConfig{}
	config.Sandbox = SandboxConfig{}
	if err := config.ValidateAPI(); err != nil {
		t.Fatalf("ValidateAPI depended on Worker-only configuration: %v", err)
	}
}

func TestWorkerExecutionConfigurationIsFailClosed(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Worker.ExecutionEnabled = true
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted incomplete execution configuration")
	}
	config.Worker.ID = "worker-a"
	config.Worker.AdapterVersion = "adapter-1"
	config.Worker.PollInterval = Duration(time.Second)
	config.Worker.LeaseDuration = Duration(30 * time.Second)
	config.Worker.RenewInterval = Duration(5 * time.Second)
	config.Worker.SecretStoreRoot = "/var/lib/agent-platform/secrets"
	config.Worker.CredentialTempRoot = "/var/lib/agent-platform/run-credentials"
	config.Worker.SandboxUID = 65532
	config.Worker.SandboxGID = 65532
	config.Retention.ArtifactPeriod = Duration(90 * 24 * time.Hour)
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
	config.Worker.RenewInterval = Duration(6 * time.Second)
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted cancellation polling slower than the SLO guard")
	}
	config.Worker.RenewInterval = Duration(5 * time.Second)
	config.Worker.LeaseDuration = Duration(61 * time.Second)
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a lease longer than the recovery guard")
	}
}

func TestWebhookConfigurationIsFailClosed(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Webhook.Enabled = true
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted incomplete Webhook configuration")
	}
	config.Webhook.PollInterval = Duration(time.Second)
	config.Webhook.RequestTimeout = Duration(5 * time.Second)
	config.Webhook.LeaseDuration = Duration(10 * time.Second)
	config.Webhook.RetryBase = Duration(time.Second)
	config.Webhook.RetryMaximum = Duration(time.Minute)
	config.Webhook.MaxAttempts = 5
	config.Webhook.SigningSecret = "0123456789abcdef0123456789abcdef"
	config.Webhook.TargetURL = "https://hooks.example.test/agent-platform"
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
}

func TestRetentionConfigurationIsFailClosed(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Retention.Enabled = true
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted incomplete Retention configuration")
	}
	config.Retention.SweepInterval = Duration(time.Hour)
	config.Retention.BatchSize = 500
	config.Retention.RunEventPeriod = Duration(90 * 24 * time.Hour)
	config.Retention.ArtifactPeriod = Duration(90 * 24 * time.Hour)
	config.Retention.WorkspacePeriod = Duration(30 * 24 * time.Hour)
	config.Retention.AuditPeriod = Duration(365 * 24 * time.Hour)
	config.Retention.IdempotencyGrace = Duration(time.Hour)
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
	config.Retention.AuditPeriod = Duration(30 * 24 * time.Hour)
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted Audit retention shorter than Run Event retention")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "platform.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validYAML(dsn string) string {
	return strings.TrimSpace(`
api:
  address: ":8080"
  read_header_timeout: 5s
  idle_timeout: 60s
  shutdown_timeout: 10s
authentication:
  mode: deny_all
worker:
  management_address: "127.0.0.1:9090"
  shutdown_timeout: 10s
  reconcile_interval: 5s
  max_attempts: 3
retention:
  enabled: false
  artifact_period: 2160h
database:
  dsn: "`+dsn+`"
  max_open_connections: 20
  max_idle_connections: 10
  connection_max_idle: 5m
  connection_max_lifetime: 30m
object_store:
  provider: minio
  minio:
    endpoint: minio:9000
    access_key: access
    secret_key: secret
    bucket: agent-platform
    secure: false
  aliyun_oss: {}
sandbox:
  runtime: runsc
  egress_network: agent-public-egress
  resolver_config: /etc/agent-platform/sandbox-resolv.conf
`) + "\n"
}
