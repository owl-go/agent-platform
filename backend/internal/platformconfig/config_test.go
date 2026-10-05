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

func TestMessageChannelsConfigurationIsOptInAndBounded(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	if config.MessageChannels.Enabled {
		t.Fatal("channels enabled by default")
	}
	for _, test := range []struct {
		name, extra string
		valid       bool
	}{
		{"enabled HTTPS origin", "enabled: true\n  callback_base_url: https://workspace.example.test\n  max_connections: 64\n  max_pending_messages: 1000\n  max_sender_messages_per_minute: 10\n  max_text_bytes: 10000\n  max_send_attempts: 8\n  send_interval: 3s", true},
		{"approved private bridge", "approved_endpoints:\n    - url: https://bridge.internal:8443/signal\n      allow_private_network: true", true},
		{"unapproved HTTP bridge", "approved_endpoints:\n    - url: http://bridge.internal", false},
		{"wildcard bridge", "approved_endpoints:\n    - url: https://*.internal", false},
		{"userinfo bridge", "approved_endpoints:\n    - url: https://user@bridge.internal", false},
		{"query bridge", "approved_endpoints:\n    - url: https://bridge.internal?token=secret", false},
		{"duplicate bridge", "approved_endpoints:\n    - url: https://bridge.internal\n    - url: https://bridge.internal", false},
		{"HTTP", "enabled: true\n  callback_base_url: http://workspace.example.test", false},
		{"callback path", "enabled: true\n  callback_base_url: https://workspace.example.test/path", false},
		{"connection budget", "enabled: true\n  callback_base_url: https://workspace.example.test\n  max_connections: 1001", false},
		{"backlog budget", "max_pending_messages: -1", false},
		{"sender budget", "max_sender_messages_per_minute: 101", false},
		{"body budget", "max_text_bytes: 10001", false},
		{"attempt budget", "max_send_attempts: 33", false},
		{"send interval", "send_interval: 1s", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			loaded, err := Load(writeConfig(t, validYAML("postgres://database/platform")+"message_channels:\n  "+test.extra+"\n"))
			if err == nil {
				err = loaded.ValidateAPI()
			}
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", test.valid, err)
			}
		})
	}
}

func TestLoadExpandsEnvironmentInsideRuntimeMap(t *testing.T) {
	t.Setenv("TEST_RUNTIME_IMAGE", "registry.example/agent-platform/claude@sha256:"+strings.Repeat("a", 64))
	fixture := strings.Replace(validYAML("postgres://database/platform"), "  sandbox_gid: 65532", `  sandbox_gid: 65532
  runtimes:
    claude:
      available: true
      image_digest: "${TEST_RUNTIME_IMAGE}"
      cli_version: "2.1.233"`, 1)

	config, err := Load(writeConfig(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker() rejected an expanded Runtime map: %v", err)
	}
	if got := config.Worker.Runtimes["claude"].ImageDigest; got != os.Getenv("TEST_RUNTIME_IMAGE") {
		t.Fatalf("Runtime image digest = %q, want expanded environment value", got)
	}
}

func TestOIDCAuthenticationConfigurationIsStrictAndFailClosed(t *testing.T) {
	config, err := Load(writeConfig(t, validOIDCYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	if err := config.ValidateAPI(); err != nil {
		t.Fatalf("ValidateAPI() rejected valid OIDC configuration: %v", err)
	}
	if config.Authentication.DiscoveryTimeout.Value() != 5*time.Second || config.Authentication.JWKSTimeout.Value() != 3*time.Second {
		t.Fatalf("unexpected OIDC configuration: %+v", config.Authentication)
	}
	loopback := config
	loopback.Authentication.Issuer = "http://127.0.0.1:18091/realms/agent-platform"
	loopback.Authentication.RedirectURI = "http://127.0.0.1:18092/auth/callback"
	loopback.Authentication.LogoutRedirectURI = "http://127.0.0.1:18092"
	if err := loopback.ValidateAPI(); err != nil {
		t.Fatalf("ValidateAPI() rejected loopback OIDC development URLs: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*AuthenticationConfig)
	}{
		{name: "missing issuer", mutate: func(value *AuthenticationConfig) { value.Issuer = "" }},
		{name: "insecure issuer", mutate: func(value *AuthenticationConfig) { value.Issuer = "http://identity.example.test" }},
		{name: "issuer query", mutate: func(value *AuthenticationConfig) { value.Issuer = "https://identity.example.test?issuer=other" }},
		{name: "missing audience", mutate: func(value *AuthenticationConfig) { value.Audience = "" }},
		{name: "missing client", mutate: func(value *AuthenticationConfig) { value.ClientID = "" }},
		{name: "unsafe redirect", mutate: func(value *AuthenticationConfig) { value.RedirectURI = "http://app.example.test/callback" }},
		{name: "unsafe logout redirect", mutate: func(value *AuthenticationConfig) { value.LogoutRedirectURI = "http://app.example.test" }},
		{name: "missing signing algorithms", mutate: func(value *AuthenticationConfig) { value.SigningAlgorithms = nil }},
		{name: "unsupported signing algorithm", mutate: func(value *AuthenticationConfig) { value.SigningAlgorithms = []string{"none"} }},
		{name: "zero discovery timeout", mutate: func(value *AuthenticationConfig) { value.DiscoveryTimeout = 0 }},
		{name: "zero JWKS timeout", mutate: func(value *AuthenticationConfig) { value.JWKSTimeout = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := config.Authentication
			candidate.SigningAlgorithms = append([]string(nil), config.Authentication.SigningAlgorithms...)
			test.mutate(&candidate)
			invalid := config
			invalid.Authentication = candidate
			if err := invalid.ValidateAPI(); err == nil {
				t.Fatal("ValidateAPI() accepted unsafe OIDC configuration")
			}
		})
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
	config.Worker.PollInterval = 0
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted zero polling interval")
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
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker: %v", err)
	}
	config.Worker.PollInterval = Duration(6 * time.Second)
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted polling slower than the responsiveness guard")
	}
	config.Worker.PollInterval = Duration(time.Second)
	config.Worker.RuntimeIdleTimeout = 0
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a zero warm Runtime idle timeout")
	}
	config.Worker.RuntimeIdleTimeout = Duration(time.Hour)
	config.Sandbox.EgressSubnet = ""
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a missing Egress subnet")
	}
	config.Sandbox.EgressSubnet = "172.30.0.0/24"
	config.Sandbox.ResolverAddresses = []string{"127.0.0.1"}
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a private Resolver address")
	}
	config.Sandbox.ResolverAddresses = []string{"1.1.1.1"}
	config.Sandbox.EgressControllerSocket = "/run/agent-platform/other.sock"
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a non-reserved Egress controller socket")
	}
}

func TestCLIBuilderConfigurationRequiresPinnedIsolatedInputs(t *testing.T) {
	config, err := Load(writeConfig(t, validYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	config.Worker.Runtimes = map[string]RuntimeEngineConfig{"codex": {Available: true, ImageDigest: "registry.example/codex@sha256:" + strings.Repeat("b", 64), CLIVersion: "test"}}
	config.Worker.CLIBuilder = CLIBuilderConfig{Enabled: true, ImageDigest: "registry.example/cli-builder@sha256:" + strings.Repeat("a", 64), EgressNetwork: "agent-npm-egress", Timeout: Duration(10 * time.Minute)}
	if err := config.ValidateWorker(); err != nil {
		t.Fatalf("ValidateWorker rejected CLI Builder: %v", err)
	}
	config.Worker.CLIBuilder.ImageDigest = "cli-builder:latest"
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted a mutable CLI Builder image")
	}
	config.Worker.CLIBuilder.ImageDigest = "registry.example/cli-builder@sha256:" + strings.Repeat("a", 64)
	config.Worker.CLIBuilder.Timeout = Duration(31 * time.Minute)
	if err := config.ValidateWorker(); err == nil {
		t.Fatal("ValidateWorker accepted an excessive CLI build timeout")
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
accounts:
  keycloak_base_url: https://identity.example.test
  realm: agent-workspace
  admin_client_id: agent-workspace-admin
  admin_client_secret: test-client-secret
  bootstrap_subject: bootstrap-admin-subject
  bootstrap_username: platform-admin
  bootstrap_email: admin@example.test
  bootstrap_display_name: Platform Administrator
workspace:
  root: /var/lib/agent-workspace/workspaces
  known_hosts: /etc/agent-workspace/known_hosts
security:
  data_encryption_key: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
worker:
  management_address: "127.0.0.1:9090"
  shutdown_timeout: 10s
  poll_interval: 1s
  runtime_idle_timeout: 30m
  credential_temp_root: /var/lib/agent-platform/run-credentials
  sandbox_uid: 65532
  sandbox_gid: 65532
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
  egress_subnet: 172.30.0.0/24
  egress_controller_socket: /run/agent-platform/egress-controller.sock
  resolver_config: /etc/agent-platform/sandbox-resolv.conf
  resolver_addresses: [223.5.5.5, 1.1.1.1]
`) + "\n"
}

func validOIDCYAML(dsn string) string {
	return strings.Replace(validYAML(dsn), "authentication:\n  mode: deny_all", `authentication:
  mode: oidc
  issuer: https://identity.example.test
  audience: agent-platform-api
  client_id: agent-platform-web
  redirect_uri: https://app.example.test/auth/callback
  logout_redirect_uri: https://app.example.test
  signing_algorithms: [RS256]
  discovery_timeout: 5s
  jwks_timeout: 3s`, 1)
}

func TestRegistrationBrokerIsOptionalAndRequiresCompleteHTTPSConfiguration(t *testing.T) {
	config, err := Load(writeConfig(t, validOIDCYAML("postgres://database/platform")))
	if err != nil {
		t.Fatal(err)
	}
	if err = config.ValidateAPI(); err != nil {
		t.Fatal("optional broker changed existing deployments", err)
	}
	good := RegistrationBrokerConfig{PublicURL: "https://workspace.test", ClientSecret: strings.Repeat("s", 32), SigningKey: "fixture-key"}
	config.Accounts.RegistrationBroker = good
	if err = config.ValidateAPI(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		mutate func(*Config)
	}{
		{"HTTP", func(c *Config) { c.Accounts.RegistrationBroker.PublicURL = "http://workspace.test" }},
		{"credentials in origin", func(c *Config) { c.Accounts.RegistrationBroker.PublicURL = "https://user@workspace.test" }},
		{"origin path", func(c *Config) { c.Accounts.RegistrationBroker.PublicURL = "https://workspace.test/path" }},
		{"origin query", func(c *Config) { c.Accounts.RegistrationBroker.PublicURL = "https://workspace.test?token=secret" }},
		{"missing key", func(c *Config) { c.Accounts.RegistrationBroker.SigningKey = "" }},
		{"short client secret", func(c *Config) { c.Accounts.RegistrationBroker.ClientSecret = "short" }},
		{"non OIDC", func(c *Config) { c.Authentication.Mode = "deny_all" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := config
			test.mutate(&candidate)
			if candidate.ValidateAPI() == nil {
				t.Fatal("invalid broker configuration accepted")
			}
		})
	}
}
