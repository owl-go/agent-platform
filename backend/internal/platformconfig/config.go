package platformconfig

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "config/platform.yaml"

type Config struct {
	API            APIConfig            `yaml:"api"`
	Authentication AuthenticationConfig `yaml:"authentication"`
	Worker         WorkerConfig         `yaml:"worker"`
	Webhook        WebhookConfig        `yaml:"webhook"`
	Retention      RetentionConfig      `yaml:"retention"`
	Database       DatabaseConfig       `yaml:"database"`
	ObjectStore    ObjectStoreConfig    `yaml:"object_store"`
	Sandbox        SandboxConfig        `yaml:"sandbox"`
}

type AuthenticationConfig struct {
	Mode              string   `yaml:"mode"`
	Issuer            string   `yaml:"issuer"`
	Audience          string   `yaml:"audience"`
	ClientID          string   `yaml:"client_id"`
	OrganizationClaim string   `yaml:"organization_claim"`
	RedirectURI       string   `yaml:"redirect_uri"`
	LogoutRedirectURI string   `yaml:"logout_redirect_uri"`
	SigningAlgorithms []string `yaml:"signing_algorithms"`
	DiscoveryTimeout  Duration `yaml:"discovery_timeout"`
	JWKSTimeout       Duration `yaml:"jwks_timeout"`
}

type RetentionConfig struct {
	Enabled          bool     `yaml:"enabled"`
	SweepInterval    Duration `yaml:"sweep_interval"`
	BatchSize        int      `yaml:"batch_size"`
	RunEventPeriod   Duration `yaml:"run_event_period"`
	ArtifactPeriod   Duration `yaml:"artifact_period"`
	WorkspacePeriod  Duration `yaml:"workspace_period"`
	AuditPeriod      Duration `yaml:"audit_period"`
	IdempotencyGrace Duration `yaml:"idempotency_grace"`
}

type WebhookConfig struct {
	Enabled        bool     `yaml:"enabled"`
	PollInterval   Duration `yaml:"poll_interval"`
	RequestTimeout Duration `yaml:"request_timeout"`
	LeaseDuration  Duration `yaml:"lease_duration"`
	RetryBase      Duration `yaml:"retry_base"`
	RetryMaximum   Duration `yaml:"retry_maximum"`
	MaxAttempts    int      `yaml:"max_attempts"`
	SigningSecret  string   `yaml:"signing_secret"`
	TargetURL      string   `yaml:"target_url"`
}

type APIConfig struct {
	Address           string   `yaml:"address"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

type WorkerConfig struct {
	ManagementAddress  string   `yaml:"management_address"`
	ShutdownTimeout    Duration `yaml:"shutdown_timeout"`
	ReconcileInterval  Duration `yaml:"reconcile_interval"`
	MaxAttempts        int      `yaml:"max_attempts"`
	ExecutionEnabled   bool     `yaml:"execution_enabled"`
	ID                 string   `yaml:"id"`
	PollInterval       Duration `yaml:"poll_interval"`
	LeaseDuration      Duration `yaml:"lease_duration"`
	RenewInterval      Duration `yaml:"renew_interval"`
	AdapterVersion     string   `yaml:"adapter_version"`
	SecretStoreRoot    string   `yaml:"secret_store_root"`
	CredentialTempRoot string   `yaml:"credential_temp_root"`
	SandboxUID         int      `yaml:"sandbox_uid"`
	SandboxGID         int      `yaml:"sandbox_gid"`
}

type DatabaseConfig struct {
	DSN                string   `yaml:"dsn"`
	MaxOpenConnections int      `yaml:"max_open_connections"`
	MaxIdleConnections int      `yaml:"max_idle_connections"`
	ConnectionMaxIdle  Duration `yaml:"connection_max_idle"`
	ConnectionMaxLife  Duration `yaml:"connection_max_lifetime"`
}

type ObjectStoreConfig struct {
	Provider string `yaml:"provider"`
	MinIO    struct {
		Endpoint  string `yaml:"endpoint"`
		AccessKey string `yaml:"access_key"`
		SecretKey string `yaml:"secret_key"`
		Bucket    string `yaml:"bucket"`
		Secure    bool   `yaml:"secure"`
	} `yaml:"minio"`
	AliyunOSS struct {
		Endpoint        string `yaml:"endpoint"`
		AccessKeyID     string `yaml:"access_key_id"`
		AccessKeySecret string `yaml:"access_key_secret"`
		Bucket          string `yaml:"bucket"`
		Prefix          string `yaml:"prefix"`
	} `yaml:"aliyun_oss"`
}

type SandboxConfig struct {
	Runtime        string `yaml:"runtime"`
	EgressNetwork  string `yaml:"egress_network"`
	ResolverConfig string `yaml:"resolver_config"`
}

type Duration time.Duration

func (duration *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a string")
	}
	parsed, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", node.Value, err)
	}
	*duration = Duration(parsed)
	return nil
}

func (duration Duration) Value() time.Duration {
	return time.Duration(duration)
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, fmt.Errorf("configuration path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration %q: %w", path, err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode configuration %q: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("decode configuration %q: multiple YAML documents are not allowed", path)
		}
		return Config{}, fmt.Errorf("decode trailing configuration %q: %w", path, err)
	}
	if err := expandEnvironment(&config); err != nil {
		return Config{}, fmt.Errorf("expand configuration %q: %w", path, err)
	}
	return config, nil
}

func expandEnvironment(config *Config) error {
	missingSet := make(map[string]struct{})
	var expand func(reflect.Value)
	expand = func(value reflect.Value) {
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				expand(value.Field(index))
			}
		case reflect.String:
			value.SetString(os.Expand(value.String(), func(name string) string {
				replacement, found := os.LookupEnv(name)
				if !found {
					missingSet[name] = struct{}{}
				}
				return replacement
			}))
		}
	}
	expand(reflect.ValueOf(config))
	missing := make([]string, 0, len(missingSet))
	for name := range missingSet {
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("environment variables are unset: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (config Config) ValidateAPI() error {
	if err := config.validateShared(); err != nil {
		return err
	}
	if strings.TrimSpace(config.API.Address) == "" {
		return fmt.Errorf("api.address is required")
	}
	if config.API.ReadHeaderTimeout.Value() <= 0 || config.API.IdleTimeout.Value() <= 0 || config.API.ShutdownTimeout.Value() <= 0 {
		return fmt.Errorf("API timeouts must be positive")
	}
	if err := config.Authentication.Validate(); err != nil {
		return err
	}
	if err := config.validateWebhook(); err != nil {
		return err
	}
	return nil
}

func (config AuthenticationConfig) Validate() error {
	switch config.Mode {
	case "deny_all":
		return nil
	case "oidc":
	default:
		return fmt.Errorf("authentication.mode must be deny_all or oidc")
	}
	if err := validateHTTPSURL("authentication.issuer", config.Issuer, true); err != nil {
		return err
	}
	if strings.TrimSpace(config.Audience) == "" || strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.OrganizationClaim) == "" {
		return fmt.Errorf("authentication audience, client_id, and organization_claim are required in oidc mode")
	}
	registeredClaims := map[string]bool{
		"iss": true, "sub": true, "aud": true, "exp": true, "nbf": true,
		"iat": true, "auth_time": true, "nonce": true, "acr": true, "amr": true, "azp": true,
	}
	if registeredClaims[config.OrganizationClaim] {
		return fmt.Errorf("authentication.organization_claim must be a dedicated Organization claim")
	}
	if err := validateHTTPSURL("authentication.redirect_uri", config.RedirectURI, true); err != nil {
		return err
	}
	if err := validateHTTPSURL("authentication.logout_redirect_uri", config.LogoutRedirectURI, true); err != nil {
		return err
	}
	if config.DiscoveryTimeout.Value() <= 0 || config.DiscoveryTimeout.Value() > 30*time.Second || config.JWKSTimeout.Value() <= 0 || config.JWKSTimeout.Value() > 30*time.Second {
		return fmt.Errorf("authentication discovery_timeout and jwks_timeout must be positive and no greater than 30s")
	}
	if len(config.SigningAlgorithms) == 0 {
		return fmt.Errorf("authentication.signing_algorithms is required in oidc mode")
	}
	allowed := map[string]bool{
		"RS256": true, "RS384": true, "RS512": true,
		"PS256": true, "PS384": true, "PS512": true,
		"ES256": true, "ES384": true, "ES512": true,
		"EdDSA": true,
	}
	seen := make(map[string]struct{}, len(config.SigningAlgorithms))
	for _, algorithm := range config.SigningAlgorithms {
		if !allowed[algorithm] {
			return fmt.Errorf("authentication signing algorithm %q is not allowed", algorithm)
		}
		if _, duplicate := seen[algorithm]; duplicate {
			return fmt.Errorf("authentication signing algorithm %q is duplicated", algorithm)
		}
		seen[algorithm] = struct{}{}
	}
	return nil
}

func validateHTTPSURL(field, value string, allowLoopbackHTTP bool) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an absolute HTTPS URL without user info, query, or fragment", field)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	hostname := parsed.Hostname()
	if allowLoopbackHTTP && parsed.Scheme == "http" && (hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1") {
		return nil
	}
	return fmt.Errorf("%s must use HTTPS except for a loopback redirect", field)
}

func (config Config) ValidateWorker() error {
	if err := config.validateShared(); err != nil {
		return err
	}
	if config.Sandbox.Runtime != "runsc" {
		return fmt.Errorf("sandbox.runtime must be runsc")
	}
	if config.Sandbox.EgressNetwork == "" {
		return fmt.Errorf("sandbox.egress_network is required")
	}
	if !filepath.IsAbs(config.Sandbox.ResolverConfig) {
		return fmt.Errorf("sandbox.resolver_config must be an absolute path")
	}
	if strings.TrimSpace(config.Worker.ManagementAddress) == "" {
		return fmt.Errorf("worker.management_address is required")
	}
	if config.Worker.ShutdownTimeout.Value() <= 0 {
		return fmt.Errorf("worker.shutdown_timeout must be positive")
	}
	if config.Worker.ReconcileInterval.Value() <= 0 || config.Worker.ReconcileInterval.Value() > 30*time.Second {
		return fmt.Errorf("worker.reconcile_interval must be positive and no greater than 30s")
	}
	if config.Worker.MaxAttempts <= 0 || config.Worker.MaxAttempts > 3 {
		return fmt.Errorf("worker.max_attempts must be between 1 and 3")
	}
	if config.Worker.ExecutionEnabled {
		if strings.TrimSpace(config.Worker.ID) == "" || strings.TrimSpace(config.Worker.AdapterVersion) == "" {
			return fmt.Errorf("worker.id and worker.adapter_version are required when execution is enabled")
		}
		if config.Worker.PollInterval.Value() <= 0 || config.Worker.LeaseDuration.Value() <= 0 || config.Worker.RenewInterval.Value() <= 0 || config.Worker.RenewInterval.Value() >= config.Worker.LeaseDuration.Value() {
			return fmt.Errorf("Worker polling and lease intervals are invalid")
		}
		if config.Worker.RenewInterval.Value() > 5*time.Second {
			return fmt.Errorf("worker.renew_interval must not exceed 5s so Run cancellation starts within 10s")
		}
		if config.Worker.PollInterval.Value() > 5*time.Second || config.Worker.LeaseDuration.Value() > time.Minute {
			return fmt.Errorf("worker.poll_interval must not exceed 5s and worker.lease_duration must not exceed 1m")
		}
		if config.Retention.ArtifactPeriod.Value() <= 0 {
			return fmt.Errorf("retention.artifact_period is required when execution is enabled")
		}
		if !filepath.IsAbs(config.Worker.SecretStoreRoot) || !filepath.IsAbs(config.Worker.CredentialTempRoot) {
			return fmt.Errorf("Worker Secret Store and credential temporary roots must be absolute")
		}
		if config.Worker.SandboxUID <= 0 || config.Worker.SandboxGID <= 0 {
			return fmt.Errorf("Worker Sandbox UID and GID must be non-root")
		}
	}
	if err := config.validateWebhook(); err != nil {
		return err
	}
	if err := config.validateRetention(); err != nil {
		return err
	}
	return nil
}

func (config Config) validateRetention() error {
	if !config.Retention.Enabled {
		return nil
	}
	if config.Retention.SweepInterval.Value() <= 0 || config.Retention.BatchSize <= 0 || config.Retention.BatchSize > 10_000 {
		return fmt.Errorf("Retention sweep interval and batch size are invalid")
	}
	if config.Retention.RunEventPeriod.Value() <= 0 || config.Retention.ArtifactPeriod.Value() <= 0 || config.Retention.WorkspacePeriod.Value() <= 0 || config.Retention.AuditPeriod.Value() <= 0 || config.Retention.IdempotencyGrace.Value() < 0 {
		return fmt.Errorf("Retention periods are invalid")
	}
	if config.Retention.AuditPeriod.Value() < config.Retention.RunEventPeriod.Value() {
		return fmt.Errorf("Retention Audit period cannot be shorter than Run Event period")
	}
	return nil
}

func (config Config) validateWebhook() error {
	if !config.Webhook.Enabled {
		return nil
	}
	if config.Webhook.PollInterval.Value() <= 0 || config.Webhook.RequestTimeout.Value() <= 0 || config.Webhook.LeaseDuration.Value() <= config.Webhook.RequestTimeout.Value() {
		return fmt.Errorf("Webhook polling, request, and lease durations are invalid")
	}
	if config.Webhook.RetryBase.Value() <= 0 || config.Webhook.RetryMaximum.Value() < config.Webhook.RetryBase.Value() || config.Webhook.MaxAttempts <= 0 {
		return fmt.Errorf("Webhook retry configuration is invalid")
	}
	if len(config.Webhook.SigningSecret) < 32 {
		return fmt.Errorf("webhook.signing_secret must contain at least 32 bytes when Webhook delivery is enabled")
	}
	target, err := url.ParseRequestURI(config.Webhook.TargetURL)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil {
		return fmt.Errorf("webhook.target_url must be an HTTPS URL without user info when Webhook delivery is enabled")
	}
	return nil
}

func (config Config) validateShared() error {
	if err := config.Database.Validate(); err != nil {
		return err
	}
	switch config.ObjectStore.Provider {
	case "minio":
		if config.ObjectStore.MinIO.Endpoint == "" || config.ObjectStore.MinIO.AccessKey == "" ||
			config.ObjectStore.MinIO.SecretKey == "" || config.ObjectStore.MinIO.Bucket == "" {
			return fmt.Errorf("MinIO object store configuration is incomplete")
		}
	case "aliyun_oss":
		if config.ObjectStore.AliyunOSS.Endpoint == "" || config.ObjectStore.AliyunOSS.AccessKeyID == "" ||
			config.ObjectStore.AliyunOSS.AccessKeySecret == "" || config.ObjectStore.AliyunOSS.Bucket == "" {
			return fmt.Errorf("Aliyun OSS object store configuration is incomplete")
		}
	default:
		return fmt.Errorf("object_store.provider must be minio or aliyun_oss")
	}
	if config.Retention.ArtifactPeriod.Value() <= 0 {
		return fmt.Errorf("retention.artifact_period must be positive")
	}
	return nil
}

func (config DatabaseConfig) Validate() error {
	if strings.TrimSpace(config.DSN) == "" {
		return fmt.Errorf("database.dsn is required")
	}
	if config.MaxOpenConnections <= 0 || config.MaxIdleConnections < 0 || config.MaxIdleConnections > config.MaxOpenConnections {
		return fmt.Errorf("database connection limits are invalid")
	}
	if config.ConnectionMaxIdle.Value() <= 0 || config.ConnectionMaxLife.Value() <= 0 {
		return fmt.Errorf("database connection lifetimes must be positive")
	}
	return nil
}
