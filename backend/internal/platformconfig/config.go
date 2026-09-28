package platformconfig

import (
	"fmt"
	"io"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "config/platform.yaml"

var immutableImageDigest = regexp.MustCompile(`^[^\s@]+@sha256:[0-9a-f]{64}$`)

type Config struct {
	API            APIConfig            `yaml:"api"`
	Authentication AuthenticationConfig `yaml:"authentication"`
	Accounts       AccountsConfig       `yaml:"accounts"`
	Workspace      WorkspaceConfig      `yaml:"workspace"`
	Security       SecurityConfig       `yaml:"security"`
	Worker         WorkerConfig         `yaml:"worker"`
	Database       DatabaseConfig       `yaml:"database"`
	ObjectStore    ObjectStoreConfig    `yaml:"object_store"`
	Sandbox        SandboxConfig        `yaml:"sandbox"`
}

type AccountsConfig struct {
	KeycloakBaseURL      string `yaml:"keycloak_base_url"`
	Realm                string `yaml:"realm"`
	AdminClientID        string `yaml:"admin_client_id"`
	AdminClientSecret    string `yaml:"admin_client_secret"`
	BootstrapSubject     string `yaml:"bootstrap_subject"`
	BootstrapUsername    string `yaml:"bootstrap_username"`
	BootstrapEmail       string `yaml:"bootstrap_email"`
	BootstrapDisplayName string `yaml:"bootstrap_display_name"`
}

type WorkspaceConfig struct {
	Root       string `yaml:"root"`
	KnownHosts string `yaml:"known_hosts"`
}

type SecurityConfig struct {
	DataEncryptionKey string `yaml:"data_encryption_key"`
}

type AuthenticationConfig struct {
	Mode              string   `yaml:"mode"`
	Issuer            string   `yaml:"issuer"`
	Audience          string   `yaml:"audience"`
	ClientID          string   `yaml:"client_id"`
	RedirectURI       string   `yaml:"redirect_uri"`
	LogoutRedirectURI string   `yaml:"logout_redirect_uri"`
	SigningAlgorithms []string `yaml:"signing_algorithms"`
	DiscoveryTimeout  Duration `yaml:"discovery_timeout"`
	JWKSTimeout       Duration `yaml:"jwks_timeout"`
}

type APIConfig struct {
	Address           string   `yaml:"address"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

type WorkerConfig struct {
	ManagementAddress  string                         `yaml:"management_address"`
	ShutdownTimeout    Duration                       `yaml:"shutdown_timeout"`
	PollInterval       Duration                       `yaml:"poll_interval"`
	RuntimeIdleTimeout Duration                       `yaml:"runtime_idle_timeout"`
	CredentialTempRoot string                         `yaml:"credential_temp_root"`
	SandboxUID         int                            `yaml:"sandbox_uid"`
	SandboxGID         int                            `yaml:"sandbox_gid"`
	Runtimes           map[string]RuntimeEngineConfig `yaml:"runtimes"`
	CLIBuilder         CLIBuilderConfig               `yaml:"cli_builder"`
}

type CLIBuilderConfig struct {
	Enabled       bool     `yaml:"enabled"`
	ImageDigest   string   `yaml:"image_digest"`
	EgressNetwork string   `yaml:"egress_network"`
	Timeout       Duration `yaml:"timeout"`
}

type RuntimeEngineConfig struct {
	Available    bool   `yaml:"available"`
	NativeResume bool   `yaml:"native_resume"`
	ImageDigest  string `yaml:"image_digest"`
	CLIVersion   string `yaml:"cli_version"`
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
	Runtime                string   `yaml:"runtime"`
	EgressNetwork          string   `yaml:"egress_network"`
	EgressSubnet           string   `yaml:"egress_subnet"`
	EgressControllerSocket string   `yaml:"egress_controller_socket"`
	ResolverConfig         string   `yaml:"resolver_config"`
	ResolverAddresses      []string `yaml:"resolver_addresses"`
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
			if value.IsNil() {
				return
			}
			value = value.Elem()
		}
		switch value.Kind() {
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				expand(value.Field(index))
			}
		case reflect.Map:
			for _, key := range value.MapKeys() {
				item := reflect.New(value.Type().Elem()).Elem()
				item.Set(value.MapIndex(key))
				expand(item)
				value.SetMapIndex(key, item)
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				expand(value.Index(index))
			}
		case reflect.Interface:
			if !value.IsNil() {
				item := reflect.New(value.Elem().Type()).Elem()
				item.Set(value.Elem())
				expand(item)
				value.Set(item)
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
	if err := config.Accounts.Validate(); err != nil {
		return err
	}
	if !filepath.IsAbs(config.Workspace.Root) || filepath.Clean(config.Workspace.Root) == string(filepath.Separator) {
		return fmt.Errorf("workspace.root must be an absolute, non-root path")
	}
	if strings.TrimSpace(config.Security.DataEncryptionKey) == "" {
		return fmt.Errorf("security.data_encryption_key is required")
	}
	return nil
}

func (config AccountsConfig) Validate() error {
	if err := validateHTTPSURL("accounts.keycloak_base_url", config.KeycloakBaseURL, true); err != nil {
		return err
	}
	if strings.TrimSpace(config.Realm) == "" || strings.TrimSpace(config.AdminClientID) == "" || strings.TrimSpace(config.AdminClientSecret) == "" {
		return fmt.Errorf("accounts realm and Keycloak Admin client credentials are required")
	}
	if strings.TrimSpace(config.BootstrapSubject) == "" || strings.TrimSpace(config.BootstrapUsername) == "" || strings.TrimSpace(config.BootstrapEmail) == "" || strings.TrimSpace(config.BootstrapDisplayName) == "" {
		return fmt.Errorf("accounts bootstrap Administrator identity is required")
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
	if strings.TrimSpace(config.Audience) == "" || strings.TrimSpace(config.ClientID) == "" {
		return fmt.Errorf("authentication audience and client_id are required in oidc mode")
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
	egressSubnet, err := netip.ParsePrefix(config.Sandbox.EgressSubnet)
	if err != nil || !egressSubnet.Addr().Is4() || !egressSubnet.Addr().IsPrivate() || egressSubnet.Bits() < 16 || egressSubnet.Bits() > 30 {
		return fmt.Errorf("sandbox.egress_subnet must be an explicit private IPv4 subnet between /16 and /30")
	}
	if !filepath.IsAbs(config.Sandbox.EgressControllerSocket) || filepath.Base(config.Sandbox.EgressControllerSocket) != "egress-controller.sock" {
		return fmt.Errorf("sandbox.egress_controller_socket must be an absolute reserved socket path")
	}
	if !filepath.IsAbs(config.Sandbox.ResolverConfig) {
		return fmt.Errorf("sandbox.resolver_config must be an absolute path")
	}
	if len(config.Sandbox.ResolverAddresses) == 0 {
		return fmt.Errorf("sandbox.resolver_addresses requires at least one public IPv4 address")
	}
	for _, value := range config.Sandbox.ResolverAddresses {
		address, parseErr := netip.ParseAddr(value)
		if parseErr != nil || !publicResolverIPv4(address) {
			return fmt.Errorf("sandbox.resolver_addresses must contain only public IPv4 addresses")
		}
	}
	if strings.TrimSpace(config.Worker.ManagementAddress) == "" {
		return fmt.Errorf("worker.management_address is required")
	}
	if config.Worker.ShutdownTimeout.Value() <= 0 {
		return fmt.Errorf("worker.shutdown_timeout must be positive")
	}
	if config.Worker.PollInterval.Value() <= 0 || config.Worker.PollInterval.Value() > 5*time.Second {
		return fmt.Errorf("worker.poll_interval must be positive and no greater than 5s")
	}
	if config.Worker.RuntimeIdleTimeout.Value() <= 0 || config.Worker.RuntimeIdleTimeout.Value() > 24*time.Hour {
		return fmt.Errorf("worker.runtime_idle_timeout must be positive and no greater than 24h")
	}
	if !filepath.IsAbs(config.Worker.CredentialTempRoot) {
		return fmt.Errorf("worker.credential_temp_root must be absolute")
	}
	if config.Worker.SandboxUID <= 0 || config.Worker.SandboxGID <= 0 {
		return fmt.Errorf("Worker Sandbox UID and GID must be non-root")
	}
	for name, runtime := range config.Worker.Runtimes {
		if name != "claude" && name != "codex" && name != "hermes" && name != "openclaw" && name != "pi" {
			return fmt.Errorf("worker.runtimes contains unsupported Runtime %q", name)
		}
		if runtime.Available && (!immutableImageDigest.MatchString(runtime.ImageDigest) || strings.TrimSpace(runtime.CLIVersion) == "") {
			return fmt.Errorf("available Runtime %q requires an immutable image digest and CLI version", name)
		}
	}
	if config.Worker.CLIBuilder.Enabled {
		builder := config.Worker.CLIBuilder
		if !immutableImageDigest.MatchString(builder.ImageDigest) {
			return fmt.Errorf("worker.cli_builder.image_digest must be an immutable repository digest")
		}
		if strings.TrimSpace(builder.EgressNetwork) == "" {
			return fmt.Errorf("worker.cli_builder.egress_network is required")
		}
		if builder.Timeout.Value() <= 0 || builder.Timeout.Value() > 30*time.Minute {
			return fmt.Errorf("worker.cli_builder.timeout must be positive and no greater than 30m")
		}
		availableRuntime := false
		for _, runtime := range config.Worker.Runtimes {
			availableRuntime = availableRuntime || runtime.Available
		}
		if !availableRuntime {
			return fmt.Errorf("worker.cli_builder requires at least one available Runtime")
		}
	}
	return nil
}

func publicResolverIPv4(address netip.Addr) bool {
	if !address.IsValid() || !address.Is4() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
	} {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
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
