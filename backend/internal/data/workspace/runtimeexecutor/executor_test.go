package runtimeexecutor

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/containerprocess"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/workspacefs"
)

func TestMaterializeCLIConnectorsVerifiesRuntimeAndProtectsBundle(t *testing.T) {
	provider := memory.New()
	store, err := cliconnector.NewArtifactStore(provider)
	if err != nil {
		t.Fatal(err)
	}
	bundle := runtimeConnectorBundle(t, "node_modules/.bin/tool", "#!/usr/bin/env node\n")
	sum := sha256.Sum256(bundle)
	digest := hex.EncodeToString(sum[:])
	key := "cli-connectors/connector-1/v1/" + digest + ".tgz"
	if err := store.PutImmutable(context.Background(), key, bundle, digest); err != nil {
		t.Fatal(err)
	}
	runtimeDigest := "sha256:" + strings.Repeat("a", 64)
	repository := &stubCLICredentialRepository{}
	executor := &Executor{connectors: store, cliCredentials: repository}
	job := application.ExecutionJob{Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "connector-1", Name: "Tool", Executable: "tool", BundleObjectKey: key, BundleSHA256: digest, RuntimeDigests: []string{runtimeDigest}}}}}
	directory, err := executor.materializeCLIConnectors(context.Background(), job, t.TempDir(), "registry.example/runtime@"+runtimeDigest)
	if err != nil {
		t.Fatal(err)
	}
	if directory == "" {
		t.Fatal("CLI Connector directory is empty")
	}
	executable := filepath.Join(directory, "connector-1", "node_modules", ".bin", "tool")
	info, err := os.Stat(executable)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("CLI Connector executable %q is writable: %o", executable, info.Mode().Perm())
	}
	job.Snapshot.CLIConnectors[0].RuntimeDigests = []string{"sha256:" + strings.Repeat("b", 64)}
	if _, err := executor.materializeCLIConnectors(context.Background(), job, t.TempDir(), "registry.example/runtime@"+runtimeDigest); err == nil {
		t.Fatal("expected an unverified Runtime digest to be rejected")
	}
	repository.runtimeVerified = true
	if _, err := executor.materializeCLIConnectors(context.Background(), job, t.TempDir(), "registry.example/runtime@"+runtimeDigest); err != nil {
		t.Fatalf("current exact-bundle conformance did not recover retained snapshot: %v", err)
	}
	if repository.bundleSHA256 != digest || repository.runtimeDigest != runtimeDigest {
		t.Fatalf("conformance lookup = %#v", repository)
	}
}

func TestMaterializeManagedCLIConnectorUsesBundleRuntimeConformance(t *testing.T) {
	provider := memory.New()
	store, err := cliconnector.NewArtifactStore(provider)
	if err != nil {
		t.Fatal(err)
	}
	bundle := runtimeConnectorBundle(t, "node_modules/.bin/tool", "#!/usr/bin/env node\n")
	sum := sha256.Sum256(bundle)
	bundleDigest := hex.EncodeToString(sum[:])
	key := "cli-connectors/connector-1/v1/" + bundleDigest + ".tgz"
	if err := store.PutImmutable(context.Background(), key, bundle, bundleDigest); err != nil {
		t.Fatal(err)
	}
	selectedDigest := "sha256:" + strings.Repeat("a", 64)
	runtimeDigest := "sha256:" + strings.Repeat("b", 64)
	repository := &stubCLICredentialRepository{bundleVerified: true}
	executor := &Executor{connectors: store, cliCredentials: repository}
	job := application.ExecutionJob{Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{
		ID: "installation-1", InstallationID: "installation-1", Name: "飞书", Executable: "tool",
		BundleObjectKey: key, BundleSHA256: bundleDigest, RuntimeDigests: []string{selectedDigest},
	}}}}
	if _, err := executor.materializeCLIConnectors(context.Background(), job, t.TempDir(), "registry.example/runtime@"+runtimeDigest); err != nil {
		t.Fatalf("exact bundle and Runtime conformance should allow the managed Connector: %v", err)
	}
	if repository.bundleSHA256 != bundleDigest || repository.runtimeDigest != runtimeDigest || repository.definitionID != "" {
		t.Fatalf("managed Connector looked up the wrong conformance identity: %#v", repository)
	}
	repository.bundleVerified = false
	if _, err := executor.materializeCLIConnectors(context.Background(), job, t.TempDir(), "registry.example/runtime@"+runtimeDigest); err == nil || !strings.Contains(err.Error(), "not verified for Runtime") {
		t.Fatalf("missing exact bundle conformance should fail closed, got %v", err)
	}
}

func TestModelRuntimeConfigDoesNotExposeCLIConnectorBundle(t *testing.T) {
	executor := &Executor{config: platformconfig.Config{
		Sandbox: platformconfig.SandboxConfig{Runtime: "runsc", EgressNetwork: "public", ResolverConfig: "/etc/resolv.conf"},
		Worker:  platformconfig.WorkerConfig{SandboxUID: 65532, SandboxGID: 65532},
	}}
	config := executor.containerConfig(application.ExecutionJob{ID: "run-1", Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeClaude}}, platformconfig.RuntimeEngineConfig{ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("a", 64)}, "agent-runtime-warm-test", warmSlot{scratch: "/runtime/scratch"}, "/workspace", "", "/credentials", "")
	if config.ConnectorDirectory != "" {
		t.Fatalf("model Runtime received CLI Connector directory %q", config.ConnectorDirectory)
	}
}

type passthroughCLIEgress struct{}

func (passthroughCLIEgress) Execute(ctx context.Context, _ string, _ []string, run func(context.Context) (cliconnector.Result, error)) (cliconnector.Result, error) {
	return run(ctx)
}

func TestStartCLIConnectorBrokerExposesOnlyProtectedSocketToModelRuntime(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "runtime-cli-broker-")
	if err != nil {
		t.Fatal(err)
	}
	temporaryRoot := root
	t.Cleanup(func() { _ = os.RemoveAll(temporaryRoot) })
	root = filepath.Join(root, ".runtime-containers", strings.Repeat("a", 32), "scratch")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	capabilities, err := json.Marshal([]cliconnector.Capability{{
		ID: "identity", ArgvPrefix: []string{"auth", "status"}, Risk: cliconnector.RiskLow,
		Identities: []cliconnector.Identity{cliconnector.IdentityUser}, EgressHosts: []string{"open.feishu.cn"}, Timeout: time.Minute,
	}})
	if err != nil {
		t.Fatal(err)
	}
	runtimeDigest := "sha256:" + strings.Repeat("a", 64)
	executor := &Executor{cliEgress: passthroughCLIEgress{}, cliCredentials: &stubCLICredentialRepository{runtimeVerified: true}, config: platformconfig.Config{
		Sandbox: platformconfig.SandboxConfig{Runtime: "runsc", EgressNetwork: "public", ResolverConfig: "/etc/resolv.conf"},
		Worker:  platformconfig.WorkerConfig{SandboxUID: os.Getuid(), SandboxGID: os.Getgid()},
	}}
	job := application.ExecutionJob{ID: "run-1", Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{
		ID: "connector-1", Name: "Tool", Executable: "tool", AuthenticationDriver: "none",
		BundleSHA256: strings.Repeat("b", 64), RuntimeDigests: []string{"sha256:" + strings.Repeat("c", 64)}, Capabilities: capabilities, Version: 1,
	}}}}
	runtime := platformconfig.RuntimeEngineConfig{ImageDigest: "registry.example/runtime@" + runtimeDigest}
	server, socket, err := executor.startCLIConnectorBroker(context.Background(), job, 1, runtime, filepath.Join(root, "connectors"), filepath.Join(root, "workspace"), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if socket != filepath.Join(root, "broker", "cli-broker.sock") {
		t.Fatalf("broker socket = %q", socket)
	}
	config := executor.containerConfig(job, runtime, "agent-runtime-warm-test", warmSlot{scratch: root}, filepath.Join(root, "workspace"), "", filepath.Join(root, "credentials"), socket)
	if config.CLIBrokerSocket != socket || config.ConnectorDirectory != "" {
		t.Fatalf("model Runtime broker=%q connector directory=%q", config.CLIBrokerSocket, config.ConnectorDirectory)
	}
}

func runtimeConnectorBundle(t *testing.T, name, content string) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestMaterializeAttachmentsVerifiesAndProtectsCopies(t *testing.T) {
	provider := memory.New()
	content := []byte("attachment contents")
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	key := "attachments/owner-1/11111111-1111-4111-8111-111111111111"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "text/plain", Metadata: map[string]string{"name": "notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	sandboxUID, sandboxGID := os.Getuid(), os.Getgid()
	if os.Geteuid() == 0 {
		sandboxUID, sandboxGID = 65532, 65532
	}
	executor := &Executor{objects: provider, config: platformconfig.Config{Worker: platformconfig.WorkerConfig{SandboxUID: sandboxUID, SandboxGID: sandboxGID}}}
	root := t.TempDir()
	attachmentDirectory := filepath.Join(root, "attachments", "11111111-1111-4111-8111-111111111111")
	if err := os.MkdirAll(attachmentDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	attachments, err := executor.materializeAttachments(context.Background(), application.ExecutionJob{OwnerID: "owner-1", Attachments: []domain.Attachment{{ID: "11111111-1111-4111-8111-111111111111", Name: "notes.txt", ContentType: "text/plain", ObjectKey: key, Size: int64(len(content)), SHA256: sha}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 1 || attachments[0].ContentType != "text/plain" {
		t.Fatalf("attachments = %v", attachments)
	}
	wantRuntimePath := "/workspace/.agent-platform-attachments/11111111-1111-4111-8111-111111111111/notes.txt"
	if attachments[0].Path != wantRuntimePath {
		t.Fatalf("Runtime attachment path = %q, want %q", attachments[0].Path, wantRuntimePath)
	}
	copyPath := filepath.Join(root, "attachments", "11111111-1111-4111-8111-111111111111", "notes.txt")
	got, err := os.ReadFile(copyPath)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("copy = %q, %v", got, err)
	}
	info, err := os.Stat(copyPath)
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("copy mode = %v, %v", info.Mode().Perm(), err)
	}
	directoryInfo, err := os.Stat(attachmentDirectory)
	if err != nil || directoryInfo.Mode().Perm() != 0o750 {
		t.Fatalf("attachment directory mode = %v, %v", directoryInfo.Mode().Perm(), err)
	}
	if stat, ok := directoryInfo.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != sandboxUID || int(stat.Gid) != sandboxGID {
		t.Fatalf("attachment directory owner = %#v, want %d:%d", directoryInfo.Sys(), sandboxUID, sandboxGID)
	}
}

func TestEnvironmentDecryptsGlobalModelCredentialWithItsOriginalScope(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("global-key"), "model-provider:administrator-1")
	if err != nil {
		t.Fatal(err)
	}
	executor := &Executor{box: box}
	job := application.ExecutionJob{OwnerID: "ordinary-user-1", Snapshot: domain.ExecutionSnapshot{ProviderModel: domain.ProviderModelSnapshot{
		Endpoint: "https://models.example.test", APIKeyCiphertext: ciphertext, CredentialOwnerID: "administrator-1",
	}}}

	variables, err := executor.environment(job)
	if err != nil {
		t.Fatal(err)
	}
	if variables["OPENAI_API_KEY"] != "global-key" || variables["ANTHROPIC_API_KEY"] != "global-key" {
		t.Fatalf("global Model Provider credential was not materialized: %#v", variables)
	}
}

func TestCLIEnvironmentResolverInjectsOneRunFeishuCredentials(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ownerID, enablementID, externalID := "owner-1", "enablement-1", "ou_user"
	appID, _ := box.Encrypt([]byte("cli_app"), "feishu-cli-application:"+ownerID)
	appSecret, _ := box.Encrypt([]byte("app-secret"), "feishu-cli-application:"+ownerID)
	token, _ := box.Encrypt([]byte("user-token"), "feishu-cli-authorization-token:"+ownerID+":"+enablementID+":"+externalID)
	repository := &stubCLICredentialRepository{credentials: cliconnector.EncryptedExecutionCredentials{
		AppIDCiphertext: appID, AppSecretCiphertext: appSecret, TokenCiphertext: token, EnablementID: enablementID, ExternalIdentityID: externalID,
	}}
	executor := &Executor{box: box, cliCredentials: repository}
	environment, err := executor.cliEnvironmentResolver(ownerID)(context.Background(), cliconnector.Definition{ID: "connector-1", AuthenticationDriver: "feishu"}, cliconnector.Capability{ID: "calendar", Scopes: []string{"calendar:calendar:read"}}, cliconnector.IdentityUser)
	if err != nil {
		t.Fatal(err)
	}
	if environment["LARKSUITE_CLI_APP_ID"] != "cli_app" || environment["LARKSUITE_CLI_APP_SECRET"] != "app-secret" || environment["LARKSUITE_CLI_USER_ACCESS_TOKEN"] != "user-token" || environment["LARKSUITE_CLI_STRICT_MODE"] != "user" {
		t.Fatalf("CLI environment = %#v", environment)
	}
	if repository.ownerID != ownerID || repository.definitionID != "connector-1" || strings.Join(repository.scopes, " ") != "calendar:calendar:read" {
		t.Fatalf("credential lookup = %#v", repository)
	}
}

type stubCLICredentialRepository struct {
	credentials           cliconnector.EncryptedExecutionCredentials
	managedMaterial       domain.ConnectorAuthorizationMaterial
	ownerID, definitionID string
	identity              cliconnector.Identity
	scopes                []string
	runtimeVerified       bool
	bundleVerified        bool
	bundleSHA256          string
	runtimeDigest         string
}

func (repository *stubCLICredentialRepository) ValidateConnectorPackageCLIInvocation(context.Context, string, string, string, string) error {
	return nil
}

func (repository *stubCLICredentialRepository) ResolveConnectorPackageAuthorization(context.Context, string, string, string, string, string) (domain.ConnectorAuthorizationMaterial, error) {
	return repository.managedMaterial, nil
}

func TestManagedCLIConnectorRejectsMissingTaskScope(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	executor := &Executor{box: box, cliCredentials: &stubCLICredentialRepository{managedMaterial: domain.ConnectorAuthorizationMaterial{Scopes: []string{"im:message"}}}}
	_, err = executor.cliEnvironmentResolver("owner")(context.Background(), cliconnector.Definition{ID: "installation", RevisionID: "revision", ManagedInstallation: true, AuthenticationDriver: "feishu"}, cliconnector.Capability{ID: "task_create", Scopes: []string{"task:task:write"}}, cliconnector.IdentityUser)
	if err == nil || !strings.Contains(err.Error(), "lacks required capability scopes") {
		t.Fatalf("missing task scope was accepted: %v", err)
	}
}

func TestManagedCLIConnectorDeliversCredentialsOnlyToTheirDriver(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ownerID := "owner-1"
	credential, err := box.Encrypt([]byte(`{"token":"provided"}`), "connector-authorization:"+ownerID)
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubCLICredentialRepository{managedMaterial: domain.ConnectorAuthorizationMaterial{CredentialCiphertext: credential, CredentialAAD: "connector-authorization:" + ownerID, CredentialFormat: "json"}}
	executor := &Executor{box: box, cliCredentials: repository}
	definition := cliconnector.Definition{ID: "installation", RevisionID: "revision", ManagedInstallation: true, AuthenticationDriver: "connector_package"}
	environment, err := executor.cliEnvironmentResolver(ownerID)(context.Background(), definition, cliconnector.Capability{ID: "read"}, cliconnector.IdentityUser)
	if err != nil || environment["CONNECTOR_CREDENTIALS_JSON"] != `{"token":"provided"}` || len(environment) != 1 {
		t.Fatalf("provided credential delivery failed: %v", err)
	}
	repository.managedMaterial.CredentialFormat = "access_token"
	if _, err := executor.cliEnvironmentResolver(ownerID)(context.Background(), definition, cliconnector.Capability{ID: "read"}, cliconnector.IdentityUser); err == nil {
		t.Fatal("Feishu access token was accepted by a generic CLI")
	}
	definition.AuthenticationDriver = "unavailable"
	if _, err := executor.cliEnvironmentResolver(ownerID)(context.Background(), definition, cliconnector.Capability{ID: "read"}, cliconnector.IdentityUser); err == nil {
		t.Fatal("unsupported authorization driver received credentials")
	}
}

func TestManagedDingTalkCLIReceivesOnlyShortLivedToken(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	aad := "connector-authorization:owner-1:installation-1:corp:user"
	credential, err := json.Marshal(map[string]string{"access_token": "short-lived-token", "refresh_token": "platform-only-refresh", "client_id": "client-1", "access_expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt(credential, aad)
	if err != nil {
		t.Fatal(err)
	}
	repository := &stubCLICredentialRepository{managedMaterial: domain.ConnectorAuthorizationMaterial{CredentialCiphertext: ciphertext, CredentialAAD: aad, CredentialFormat: "json"}}
	executor := &Executor{box: box, cliCredentials: repository}
	definition := cliconnector.Definition{ID: "installation-1", RevisionID: "revision-1", ManagedInstallation: true, AuthenticationDriver: "dingtalk"}
	environment, err := executor.cliEnvironmentResolver("owner-1")(context.Background(), definition, cliconnector.Capability{ID: "read"}, cliconnector.IdentityUser)
	if err != nil || environment["AGENT_PLATFORM_DWS_ACCESS_TOKEN"] != "short-lived-token" || environment["DO_NOT_TRACK"] != "1" || len(environment) != 2 {
		t.Fatalf("DingTalk credential materialization failed: environment keys %d, error %v", len(environment), err)
	}
	for _, value := range environment {
		if strings.Contains(value, "platform-only-refresh") {
			t.Fatal("refresh token entered the CLI process")
		}
	}
}

func (repository *stubCLICredentialRepository) ResolveCLIConnectorExecutionCredentials(_ context.Context, ownerID, definitionID string, identity cliconnector.Identity, scopes []string) (cliconnector.EncryptedExecutionCredentials, error) {
	repository.ownerID, repository.definitionID, repository.identity, repository.scopes = ownerID, definitionID, identity, append([]string(nil), scopes...)
	return repository.credentials, nil
}

func (repository *stubCLICredentialRepository) HasCLIConnectorRuntimeConformance(_ context.Context, definitionID, bundleSHA256, runtimeDigest string) (bool, error) {
	repository.definitionID, repository.bundleSHA256, repository.runtimeDigest = definitionID, bundleSHA256, runtimeDigest
	return repository.runtimeVerified, nil
}

func (repository *stubCLICredentialRepository) HasConnectorBundleRuntimeConformance(_ context.Context, bundleSHA256, runtimeDigest string) (bool, error) {
	repository.bundleSHA256, repository.runtimeDigest = bundleSHA256, runtimeDigest
	return repository.bundleVerified, nil
}

func TestEnvironmentUsesDeepSeekAnthropicEndpointWithoutChangingOpenAIEndpoint(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("deepseek-key"), "model-provider:administrator-1")
	if err != nil {
		t.Fatal(err)
	}
	executor := &Executor{box: box}
	job := application.ExecutionJob{OwnerID: "ordinary-user-1", Snapshot: domain.ExecutionSnapshot{ProviderModel: domain.ProviderModelSnapshot{
		ProviderType: "deepseek", Endpoint: "https://api.deepseek.com", APIKeyCiphertext: ciphertext, CredentialOwnerID: "administrator-1",
	}}}

	variables, err := executor.environment(job)
	if err != nil {
		t.Fatal(err)
	}
	if got := variables["ANTHROPIC_BASE_URL"]; got != "https://api.deepseek.com/anthropic" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want DeepSeek Anthropic endpoint", got)
	}
	if got := variables["OPENAI_BASE_URL"]; got != "https://api.deepseek.com" {
		t.Fatalf("OPENAI_BASE_URL = %q, want unchanged DeepSeek endpoint", got)
	}
}

func TestEnvironmentUsesZhipuClaudeCodeAuthToken(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte("zhipu-key"), "model-provider:administrator-1")
	if err != nil {
		t.Fatal(err)
	}
	executor := &Executor{box: box}
	job := application.ExecutionJob{OwnerID: "ordinary-user-1", Snapshot: domain.ExecutionSnapshot{ProviderModel: domain.ProviderModelSnapshot{
		ProviderType: "zhipu", Endpoint: "https://open.bigmodel.cn/api/anthropic", APIKeyCiphertext: ciphertext, CredentialOwnerID: "administrator-1",
	}}}

	variables, err := executor.environment(job)
	if err != nil {
		t.Fatal(err)
	}
	if got := variables["ANTHROPIC_AUTH_TOKEN"]; got != "zhipu-key" {
		t.Fatalf("ANTHROPIC_AUTH_TOKEN = %q, want Zhipu API key", got)
	}
	if _, exists := variables["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("ANTHROPIC_API_KEY must be omitted for Zhipu Claude Code")
	}
	if got := variables["ANTHROPIC_BASE_URL"]; got != "https://open.bigmodel.cn/api/anthropic" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q, want configured Zhipu Anthropic endpoint", got)
	}
}

func TestAnthropicBaseURLOnlyDerivesDeepSeekRouteOnce(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		endpoint     string
		want         string
	}{
		{name: "DeepSeek root", providerType: "deepseek", endpoint: "https://api.deepseek.com", want: "https://api.deepseek.com/anthropic"},
		{name: "DeepSeek trailing slash", providerType: "deepseek", endpoint: "https://api.deepseek.com/", want: "https://api.deepseek.com/anthropic"},
		{name: "DeepSeek Anthropic route", providerType: "deepseek", endpoint: "https://api.deepseek.com/anthropic", want: "https://api.deepseek.com/anthropic"},
		{name: "Anthropic unchanged", providerType: "anthropic", endpoint: "https://api.anthropic.com/", want: "https://api.anthropic.com/"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := anthropicBaseURL(test.providerType, test.endpoint); got != test.want {
				t.Fatalf("Anthropic Base URL = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExecuteMakesImageAttachmentReadableAndKeepsWorkspaceWritable(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	job.Snapshot.ExpertTeam = nil
	_, slot, err := executor.warmSlot(job, executor.config.Worker.Runtimes[string(domain.RuntimeCodex)])
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("synthetic image contents")
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	attachmentID := "33333333-3333-4333-8333-333333333333"
	key := "attachments/owner-1/" + attachmentID
	provider, ok := executor.objects.(*memory.Provider)
	if !ok {
		t.Fatalf("Object Store = %T, want *memory.Provider", executor.objects)
	}
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "image/png", Metadata: map[string]string{"name": "photo.png"}}); err != nil {
		t.Fatal(err)
	}
	job.Attachments = []domain.Attachment{{ID: attachmentID, Name: "photo.png", ContentType: "image/png", ObjectKey: key, Size: int64(len(content)), SHA256: sha, Image: true}}

	var hostAttachmentPath string
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if len(request.Attachments) != 1 || request.Attachments[0].ContentType != "image/png" {
				return agentruntime.Result{}, fmt.Errorf("Runtime attachments = %#v", request.Attachments)
			}
			if !strings.HasPrefix(filepath.Clean(request.Attachments[0].Path), "/workspace/.agent-platform-attachments/") {
				return agentruntime.Result{}, fmt.Errorf("Runtime attachment path %q is outside /workspace", request.Attachments[0].Path)
			}
			relative, err := filepath.Rel(containerprocess.RuntimeAttachmentDirectory(runtimeWorkspaceDirectory), request.Attachments[0].Path)
			if err != nil {
				return agentruntime.Result{}, fmt.Errorf("map Runtime attachment path %q: %w", request.Attachments[0].Path, err)
			}
			if relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return agentruntime.Result{}, fmt.Errorf("Runtime attachment path %q escapes its mount", request.Attachments[0].Path)
			}
			hostAttachmentPath = filepath.Join(slot.scratch, "attachments", relative)
			got, err := os.ReadFile(hostAttachmentPath)
			if err != nil {
				return agentruntime.Result{}, fmt.Errorf("read Runtime attachment: %w", err)
			}
			if !bytes.Equal(got, content) {
				return agentruntime.Result{}, fmt.Errorf("Runtime attachment content = %q", got)
			}
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "result.txt"), []byte("workspace is writable"), 0o600); err != nil {
				return agentruntime.Result{}, fmt.Errorf("write Runtime Workspace: %w", err)
			}
			publishSuccessfulRuntime(t, events, request.RunID, "done")
			return agentruntime.Result{FinalMessage: "done"}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalMessage != "done" {
		t.Fatalf("final message = %q", result.FinalMessage)
	}
	commitSuccessfulResult(t, result)
	workspaceResult, err := os.ReadFile(filepath.Join(persistent, "result.txt"))
	if err != nil || string(workspaceResult) != "workspace is writable" {
		t.Fatalf("persisted Workspace result = %q, %v", workspaceResult, err)
	}
	if _, err := os.Stat(hostAttachmentPath); !os.IsNotExist(err) {
		t.Fatalf("ephemeral attachment was not cleaned up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(persistent, ".agent-platform-attachments")); !os.IsNotExist(err) {
		t.Fatalf("reserved attachment mountpoint was persisted: %v", err)
	}
}

func TestExecuteChannelSecretsFilterOutputWithoutEnteringRuntime(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	job.Snapshot.ExpertTeam = nil
	secret := "channel-only-secret-value"
	job.AdditionalRedactionValues = [][]byte{[]byte(secret)}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, config cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			encoded, _ := json.Marshal(request)
			if strings.Contains(string(encoded), secret) {
				t.Fatal("channel credential entered Runtime request")
			}
			_, slot, err := executor.warmSlot(job, executor.config.Worker.Runtimes[string(domain.RuntimeCodex)])
			if err != nil {
				t.Fatal(err)
			}
			err = filepath.WalkDir(slot.credentials, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !entry.IsDir() {
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if bytes.Contains(data, []byte(secret)) {
						t.Fatal("channel credential entered credential mount")
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "report.txt"), []byte(secret), 0600); err != nil {
				t.Fatal(err)
			}
			publishSuccessfulRuntime(t, events, request.RunID, secret)
			return agentruntime.Result{FinalMessage: secret}, nil
		}}, nil
	}
	progress := &recordingProgress{}
	result, err := executor.Execute(context.Background(), job, progress)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.FinalMessage, secret) || !strings.Contains(result.FinalMessage, "[REDACTED]") {
		t.Fatalf("final result not filtered: %q", result.FinalMessage)
	}
	for _, event := range progress.events {
		encoded, _ := json.Marshal(event)
		if bytes.Contains(encoded, []byte(secret)) {
			t.Fatal("progress leaked channel secret")
		}
	}
	commitSuccessfulResult(t, result)
	data, err := os.ReadFile(filepath.Join(persistent, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(secret)) {
		t.Fatal("promoted Workspace leaked channel secret")
	}
}

func TestBuildInstructionAppliesPresetAndCustomPersonality(t *testing.T) {
	tests := []struct {
		name        string
		personality string
		guidance    string
		want        []string
	}{
		{name: "preset", personality: "direct_efficient", guidance: "Prefer bullet points.", want: []string{"Be direct, concise", "Prefer bullet points.", "Do the work"}},
		{name: "custom", personality: "custom", guidance: "Answer like a patient tutor.", want: []string{"Answer like a patient tutor.", "Do the work"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instruction := buildInstruction(application.ExecutionJob{
				Instruction: "Do the work",
				Snapshot:    domain.ExecutionSnapshot{Personality: test.personality, PersonalityInstructions: test.guidance},
			}, nil)
			for _, value := range test.want {
				if !strings.Contains(instruction, value) {
					t.Fatalf("instruction %q does not contain %q", instruction, value)
				}
			}
		})
	}
}

func TestBuildInstructionUsesExecutionInstructionNotCapabilityIntroduction(t *testing.T) {
	job := application.ExecutionJob{Instruction: "Current task", Snapshot: domain.ExecutionSnapshot{Expert: &domain.ExpertSnapshot{Name: "Architect", CapabilityIntroduction: "Public marketing copy", ExecutionInstruction: "Inspect boundaries before proposing changes."}}}
	got := buildInstruction(job, nil)
	if !strings.Contains(got, "Inspect boundaries before proposing changes.") {
		t.Fatalf("instruction does not contain Expert Execution Instruction: %q", got)
	}
	if strings.Contains(got, "Public marketing copy") {
		t.Fatalf("instruction contains display-only Capability Introduction: %q", got)
	}
}

func TestBuildInstructionDescribesOnlyReviewedCLIConnectorForms(t *testing.T) {
	capabilities, err := json.Marshal([]cliconnector.Capability{{ID: "identity", ArgvPrefix: []string{"auth", "status"}, Identities: []cliconnector.Identity{cliconnector.IdentityUser}}})
	if err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Instruction: "Check identity", Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "connector-1", Name: "Feishu CLI", Capabilities: capabilities}}}}
	got := buildInstruction(job, nil)
	want := "agent-cli --connector connector-1 --capability identity --identity user [--target <target>] -- auth status"
	if !strings.Contains(got, want) || strings.Contains(got, "/opt/agent-platform/connector") {
		t.Fatalf("CLI Connector instruction = %q", got)
	}
}

func TestBuildInstructionRequiresTargetForHighRiskCLICommand(t *testing.T) {
	capabilities, err := json.Marshal([]cliconnector.Capability{
		{ID: "page-read", ArgvPrefix: []string{"pages", "get"}, Risk: cliconnector.RiskLow, Identities: []cliconnector.Identity{cliconnector.IdentityUser}},
		{ID: "page-create", ArgvPrefix: []string{"pages", "create"}, Risk: cliconnector.RiskHigh, Identities: []cliconnector.Identity{cliconnector.IdentityUser}},
	})
	if err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Instruction: "Create a test page", Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "notion-1", Name: "Notion", PackageObjectKey: "packages/notion", Capabilities: capabilities}}}}
	instruction := buildInstruction(job, nil)
	for _, want := range []string{
		"--capability page-read --identity user [--target <target>] -- pages get",
		"--capability page-create --identity user --target <target> -- pages create",
		"High-risk commands require a non-empty --target before the -- separator",
	} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("missing reviewed command guidance %q", want)
		}
	}
}

func TestManagedWeComLargeCatalogUsesCompactSkillInstruction(t *testing.T) {
	capabilities := make([]cliconnector.Capability, 90)
	for i := range capabilities {
		capabilities[i] = cliconnector.Capability{ID: fmt.Sprintf("command_%d", i), ArgvPrefix: []string{"doc", "search"}, Identities: []cliconnector.Identity{cliconnector.IdentityUser}}
	}
	encoded, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Instruction: "Find a document", Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "wecom-1", Name: "企业微信", Executable: "wecom-workspace", AuthenticationDriver: "connector_package", PackageObjectKey: "packages/wecom", Capabilities: encoded}}}}
	instruction := buildInstruction(job, nil)
	if !strings.Contains(instruction, "/run/agent-credentials/connector-skills/wecom-1/SKILL.md") || !strings.Contains(instruction, "capabilities.json") || !strings.Contains(instruction, "--identity <reviewed-identity>") || strings.Contains(instruction, "command_89") {
		t.Fatalf("WeCom large-catalog instruction = %q", instruction)
	}
}

func TestSelectedFeishuConnectorProvidesSkillBeforeCLIUse(t *testing.T) {
	capabilities, err := json.Marshal([]cliconnector.Capability{{ID: "im_messages_send", ArgvPrefix: []string{"im", "+messages-send"}, Identities: []cliconnector.Identity{cliconnector.IdentityUser}}})
	if err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Instruction: "Send a message", Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "feishu-1", Name: "Feishu", AuthenticationDriver: "feishu", Capabilities: capabilities}}}}
	files, _, _, err := (&Executor{}).extensionFiles(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	skill := string(files["connector-skills/feishu-1/SKILL.md"])
	if !strings.Contains(skill, "im +chat-search --query") || !strings.Contains(skill, "im +messages-send --chat-id") {
		t.Fatalf("Feishu Skill omits the group message flow: %q", skill)
	}
	if len(files["connector-skills/feishu-1/references/lark-task/SKILL.md"]) == 0 || len(files["connector-skills/feishu-1/references/lark-mail/SKILL.md"]) == 0 {
		t.Fatal("pinned Feishu domain references were not mounted")
	}
	instruction := buildInstruction(job, nil)
	if !strings.Contains(instruction, "im +messages-send --chat-id") || strings.Index(instruction, "Feishu CLI Skill") > strings.Index(instruction, "Available isolated CLI Connectors") {
		t.Fatalf("Feishu Skill was not loaded before CLI forms: %q", instruction)
	}
}

func TestManagedFeishuLargeCatalogUsesCompactSkillInstruction(t *testing.T) {
	capabilities := make([]cliconnector.Capability, 41)
	for i := range capabilities {
		capabilities[i] = cliconnector.Capability{ID: fmt.Sprintf("command_%d", i), ArgvPrefix: []string{"docs", "+create"}, Identities: []cliconnector.Identity{cliconnector.IdentityUser}}
	}
	encoded, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "feishu-1", Name: "Feishu", AuthenticationDriver: "feishu", PackageObjectKey: "connectors/feishu/package.zip", Capabilities: encoded}}}}
	instruction := buildInstruction(job, nil)
	if !strings.Contains(instruction, "capabilities.json") || !strings.Contains(instruction, "SKILL.md") || strings.Contains(instruction, "command_40") {
		t.Fatalf("managed Feishu instruction is not compact: %q", instruction)
	}
}

func TestSelectedManagedCLIConnectorMountsFrozenPackageSkills(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entries := map[string]string{
		"connector-meta.json":  `{"source":"example-cli","version":"1.0.0","type":"cli","name":"Example","description":"Example CLI","examples_zh":["发送"],"examples_en":["Send"],"minPlatformVersion":"1.0.0","auth_mode":"none"}`,
		"icon.svg":             `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"cli.json":             `{"runtime":{"kind":"node","version":"22.14.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"executable":"example","commands":{"init":{"argv":["setup"]},"auth":{"argv":["auth"]},"status":{"argv":["status"]},"unAuth":{"argv":["logout"]}},"status_match":{"json_path":"$.authenticated","equals":true},"egress_hosts":["api.example.com"],"timeout_seconds":60,"capabilities":[{"id":"send","argv_prefix":["message","send"],"risk":"high","identities":["user"],"egress_hosts":["api.example.com"],"timeout_seconds":30}]}`,
		"skills/send/SKILL.md": "---\nname: example-send\ndisplay_name: Send\ndescription: Send messages\nversion: 1.0.0\nauthor: Example\n---\n\n# Send\nUse the reviewed send command.\n",
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	provider := memory.New()
	digest := sha256.Sum256(archive.Bytes())
	sha := hex.EncodeToString(digest[:])
	key := "connectors/example-cli/1.0.0/" + sha + ".zip"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(archive.Bytes()), objectstore.PutOptions{Size: int64(archive.Len()), SHA256: sha, ContentType: "application/zip"}); err != nil {
		t.Fatal(err)
	}
	connector := domain.CLIConnectorSnapshot{ID: "installed-1", Name: "Example", PackageObjectKey: key, PackageSHA256: sha}
	job := application.ExecutionJob{Snapshot: domain.ExecutionSnapshot{CLIConnectors: []domain.CLIConnectorSnapshot{connector}}}
	files, _, _, err := (&Executor{objects: provider}).extensionFiles(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["connector-skills/installed-1/SKILL.md"], []byte(entries["skills/send/SKILL.md"])) || !bytes.Equal(files["connector-skills/installed-1/skills/send/SKILL.md"], []byte(entries["skills/send/SKILL.md"])) {
		t.Fatalf("mounted Connector Skills = %#v", files)
	}
	if !strings.Contains(buildInstruction(job, nil), "first read /run/agent-credentials/connector-skills/installed-1/SKILL.md") {
		t.Fatal("managed Connector Skill was not prioritized")
	}
	job.Snapshot.CLIConnectors[0].PackageSHA256 = strings.Repeat("b", 64)
	if _, _, _, err := (&Executor{objects: provider}).extensionFiles(context.Background(), job); err == nil {
		t.Fatal("expected a changed frozen package digest to be rejected")
	}
}

func TestBuildInstructionUsesOnlyVisibleStructuredExpertGuidanceInOrder(t *testing.T) {
	expert := &domain.ExpertSnapshot{
		Name:               "Architect",
		Introduction:       "Display-only introduction",
		CoreCapability:     "Design service boundaries.",
		OperatingProcedure: "Inspect evidence before deciding.",
		OutputStandard:     "Return a decision and verification plan.",
		Cautions:           "Do not claim unverified capabilities.",
		ExpertiseTags:      []string{"display-only-tag"},
	}
	got := buildInstruction(application.ExecutionJob{Instruction: "Current task", Snapshot: domain.ExecutionSnapshot{Expert: expert}}, nil)
	want := []string{"Core Capability:\nDesign service boundaries.", "Operating Procedure:\nInspect evidence before deciding.", "Output Standard:\nReturn a decision and verification plan.", "Cautions:\nDo not claim unverified capabilities."}
	position := -1
	for _, section := range want {
		next := strings.Index(got, section)
		if next <= position {
			t.Fatalf("structured guidance is missing or out of order: %q", got)
		}
		position = next
	}
	if strings.Contains(got, expert.Introduction) || strings.Contains(got, expert.ExpertiseTags[0]) {
		t.Fatalf("display-only Expert metadata entered instruction: %q", got)
	}
}

func TestTeamMemberJobPassesOnlyPriorFinalResultsAndOwnExtensions(t *testing.T) {
	base := application.ExecutionJob{Instruction: "Solve the task", CheckpointRef: "native-session", Snapshot: domain.ExecutionSnapshot{ExpertTeam: &domain.ExpertTeamSnapshot{ID: "team-1"}}}
	member := domain.ExpertMemberSnapshot{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-2", Name: "Builder", ExecutionInstruction: "Implement it."}, Position: 2, MCPServers: []domain.MCPServerSnapshot{{ID: "mcp-2"}}, Skills: []domain.SkillSnapshot{{ID: "skill-2"}}, CLIConnectors: []domain.CLIConnectorSnapshot{{ID: "cli-2"}}}
	got := teamMemberJob(base, member, []domain.ExpertStage{{ExpertName: "Architect", FinalText: "Use a hexagonal boundary."}})
	if got.CheckpointRef != "" || got.Snapshot.Expert == nil || got.Snapshot.Expert.ID != "expert-2" {
		t.Fatalf("team member execution identity = %#v", got)
	}
	if len(got.Snapshot.MCPServers) != 1 || got.Snapshot.MCPServers[0].ID != "mcp-2" || len(got.Snapshot.Skills) != 1 || got.Snapshot.Skills[0].ID != "skill-2" || len(got.Snapshot.CLIConnectors) != 1 || got.Snapshot.CLIConnectors[0].ID != "cli-2" {
		t.Fatalf("team member extensions = %#v / %#v / %#v", got.Snapshot.MCPServers, got.Snapshot.Skills, got.Snapshot.CLIConnectors)
	}
	if !strings.Contains(got.Instruction, "Architect:\nUse a hexagonal boundary.") {
		t.Fatalf("team member instruction = %q", got.Instruction)
	}
}

func TestEventSinkStreamsIntermediateDeltasWithoutPublishingAnOfficialTerminalEvent(t *testing.T) {
	recorder := &recordingProgress{}
	sink := &eventSink{runID: "run-1", job: application.ExecutionJob{ID: "run-1"}, progress: recorder, suppressMessages: true}
	for _, kind := range []agentruntime.EventKind{agentruntime.EventMessageCompleted, agentruntime.EventRuntimeCompleted, agentruntime.EventMessageDelta, agentruntime.EventCommandCompleted} {
		if err := sink.Publish(context.Background(), agentruntime.Event{RunID: "run-1", Kind: kind, OccurredAt: time.Now(), Payload: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(recorder.events) != 2 || recorder.events[0].Type != string(agentruntime.EventMessageDelta) || recorder.events[1].Type != string(agentruntime.EventCommandCompleted) {
		t.Fatalf("published events = %#v", recorder.events)
	}
}

func TestExecuteExpertTeamRunsInOrderAndCommitsOnlyTheFinalResult(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	var calls []string
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		position := len(calls) + 1
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			calls = append(calls, request.Instruction)
			if position == 1 {
				if err := os.WriteFile(filepath.Join(request.WorkspacePath, "shared.txt"), []byte("stage one"), 0o600); err != nil {
					return agentruntime.Result{}, err
				}
			} else {
				body, err := os.ReadFile(filepath.Join(request.WorkspacePath, "shared.txt"))
				if err != nil || string(body) != "stage one" || !strings.Contains(request.Instruction, "First Expert:\nfirst result") {
					return agentruntime.Result{}, errors.New("second Expert did not receive shared context")
				}
			}
			message := []string{"first result", "final result"}[position-1]
			publishSuccessfulRuntime(t, events, request.RunID, message)
			return agentruntime.Result{FinalMessage: message}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || !strings.Contains(calls[0], "First Expert instruction") || !strings.Contains(calls[1], "Second Expert instruction") {
		t.Fatalf("execution order = %#v", calls)
	}
	if result.FinalMessage != "final result" || len(result.ExpertStages) != 2 || result.ExpertStages[0].FinalText != "first result" || result.ExpertStages[1].FinalText != "final result" {
		t.Fatalf("execution result = %#v", result)
	}
	commitSuccessfulResult(t, result)
	body, err := os.ReadFile(filepath.Join(persistent, "shared.txt"))
	if err != nil || string(body) != "stage one" {
		t.Fatalf("committed Workspace = %q, %v", body, err)
	}
}

func TestExecuteExpertTeamUsesEachStagesRuntimeAndModel(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	executor.config.Worker.Runtimes["claude"] = executor.config.Worker.Runtimes["codex"]
	claudeSecret, err := executor.box.Encrypt([]byte("claude-key"), "model-provider:owner-1")
	if err != nil {
		t.Fatal(err)
	}
	codexSecret, err := executor.box.Encrypt([]byte("codex-key"), "model-provider:owner-1")
	if err != nil {
		t.Fatal(err)
	}
	job.Snapshot = domain.ExecutionSnapshot{SchemaVersion: 2, WorkspacePath: job.Snapshot.WorkspacePath, Stages: []domain.ExecutionStageSnapshot{
		{Position: 1, Expert: &domain.ExpertSnapshot{ID: "expert-1", Name: "First Expert", ExecutionInstruction: "First Expert instruction", Version: 2}, RuntimeEngine: domain.RuntimeClaude, ProviderModel: domain.ProviderModelSnapshot{ID: "model-claude", ModelID: "claude-model", Endpoint: "https://claude.example.test", APIKeyCiphertext: claudeSecret}},
		{Position: 2, Expert: &domain.ExpertSnapshot{ID: "expert-2", Name: "Second Expert", ExecutionInstruction: "Second Expert instruction", Version: 3}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ID: "model-codex", ModelID: "codex-model", Endpoint: "https://codex.example.test", APIKeyCiphertext: codexSecret}},
	}}
	var containerConfigs []containerprocess.Config
	var credentialKeys []string
	executor.checkout = func(context.Context, string) (runtimeLease, error) {
		return &recordingLease{start: func(config containerprocess.Config) {
			containerConfigs = append(containerConfigs, config)
			key, readErr := os.ReadFile(filepath.Join(config.CredentialDirectory, "env", "OPENAI_API_KEY"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			credentialKeys = append(credentialKeys, string(key))
		}}, nil
	}
	var engines []domain.RuntimeEngine
	var models []string
	executor.newAdapter = func(engine domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		engines = append(engines, engine)
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			models = append(models, request.Model)
			publishSuccessfulRuntime(t, events, request.RunID, request.Model)
			return agentruntime.Result{FinalMessage: request.Model}, nil
		}}, nil
	}

	if _, err := executor.Execute(context.Background(), job, &recordingProgress{}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(engines, []domain.RuntimeEngine{domain.RuntimeClaude, domain.RuntimeCodex}) || !reflect.DeepEqual(models, []string{"claude-model", "codex-model"}) {
		t.Fatalf("stage runtime/model calls = %#v / %#v", engines, models)
	}
	if len(containerConfigs) != 2 || containerConfigs[0].RuntimeCommand != "claude" || containerConfigs[1].RuntimeCommand != "codex" || containerConfigs[0].Image != containerConfigs[1].Image {
		t.Fatalf("stage container configs = %#v", containerConfigs)
	}
	if !reflect.DeepEqual(credentialKeys, []string{"claude-key", "codex-key"}) {
		t.Fatalf("stage credential identities = %#v", credentialKeys)
	}
}

func TestExecuteAnonymousStageProducesTerminalAuditRecord(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Snapshot.ExpertTeam = nil
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			publishSuccessfulRuntime(t, events, request.RunID, "done")
			return agentruntime.Result{FinalMessage: "done"}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ExpertStages) != 1 || result.ExpertStages[0].State != "succeeded" || result.ExpertStages[0].Position != 1 || result.ExpertStages[0].Total != 1 {
		t.Fatalf("anonymous terminal stage = %#v", result.ExpertStages)
	}
}

func TestExecuteUsesAuthoritativeMarkdownWithoutDisplayOrLegacyGuidance(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Snapshot.ExpertTeam = nil
	var expert domain.ExpertSnapshot
	if err := json.Unmarshal([]byte(`{"id":"expert","name":"Reviewer","guidance":"# Rules\n\nPreserve **Markdown**.\n","introduction":"DISPLAY_ONLY","core_capability":"STALE_FORM"}`), &expert); err != nil {
		t.Fatal(err)
	}
	job.Snapshot.Expert = &expert
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if !strings.Contains(request.Instruction, "# Rules\n\nPreserve **Markdown**.\n") || strings.Contains(request.Instruction, "DISPLAY_ONLY") || strings.Contains(request.Instruction, "STALE_FORM") {
				t.Fatalf("instruction=%q", request.Instruction)
			}
			publishSuccessfulRuntime(t, events, request.RunID, "done")
			return agentruntime.Result{FinalMessage: "done"}, nil
		}}, nil
	}
	if _, err := executor.Execute(context.Background(), job, &recordingProgress{}); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteTreatsInvalidRuntimeUsageAsFrozenFallback(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Timezone = "UTC"
	job.Snapshot.ExpertTeam = nil
	job.Snapshot.ProviderModel.ID = "model-1"
	job.Snapshot.ProviderModel.ProviderType = "openai"
	job.Snapshot.ProviderModel.Protocols = []string{"openai_responses"}
	repository := &fallbackCreditRepository{}
	creditService, err := creditsapplication.New(repository, func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.EnableCredits(creditService); err != nil {
		t.Fatal(err)
	}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			publishSuccessfulRuntime(t, events, request.RunID, "done")
			return agentruntime.Result{FinalMessage: "done", Usage: agentruntime.Usage{InputTokens: -1, Reported: true}}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CreditSettlements) != 1 || result.CreditSettlements[0].Amount != int64(creditsdomain.DefaultFallback) || !result.CreditSettlements[0].Estimated {
		t.Fatalf("fallback settlement = %#v", result.CreditSettlements)
	}
	if result.CreditConsumption == nil || len(result.CreditConsumption.Stages) != 1 || result.CreditConsumption.Stages[0].UsageReported || result.CreditConsumption.Stages[0].InputTokens != 0 {
		t.Fatalf("fallback Stage audit = %#v", result.CreditConsumption)
	}
	if repository.aborted {
		t.Fatal("valid fallback settlement aborted its execution lease")
	}
}

func TestExecuteDoesNotChargeWhenRuntimeFailsBeforeModelInvocation(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Timezone = "UTC"
	job.Snapshot.ExpertTeam = nil
	job.Snapshot.ProviderModel.ID = "model-1"
	job.Snapshot.ProviderModel.ProviderType = "openai"
	job.Snapshot.ProviderModel.Protocols = []string{"openai_responses"}
	repository := &fallbackCreditRepository{}
	creditService, err := creditsapplication.New(repository, func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.EnableCredits(creditService); err != nil {
		t.Fatal(err)
	}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error) {
			return agentruntime.Result{}, errors.New("process did not start")
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err == nil {
		t.Fatal("expected pre-invocation Runtime failure")
	}
	if len(result.CreditSettlements) != 0 || result.CreditConsumption != nil {
		t.Fatalf("pre-invocation failure was charged: %#v", result)
	}
	if !repository.aborted {
		t.Fatal("pre-invocation failure did not release its Credit admission")
	}
}

func TestExecuteDoesNotChargeWhenRuntimeFailsWithoutResponseOrUsage(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Timezone = "UTC"
	job.Snapshot.ExpertTeam = nil
	job.Snapshot.ProviderModel.ID = "model-1"
	job.Snapshot.ProviderModel.ProviderType = "openai"
	job.Snapshot.ProviderModel.Protocols = []string{"openai_responses"}
	repository := &fallbackCreditRepository{}
	creditService, err := creditsapplication.New(repository, func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.EnableCredits(creditService); err != nil {
		t.Fatal(err)
	}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error) {
			return agentruntime.Result{ModelInvocationStarted: true}, errors.New("runtime exited without a response")
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err == nil {
		t.Fatal("expected Runtime failure")
	}
	if len(result.CreditSettlements) != 0 || result.CreditConsumption != nil {
		t.Fatalf("response-less failure was charged: %#v", result)
	}
	if !repository.aborted {
		t.Fatal("response-less failure did not release its Credit admission")
	}
}

func TestExecuteChargesMeasuredUsageWhenRuntimeFailsAfterUsageReport(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Timezone = "UTC"
	job.Snapshot.ExpertTeam = nil
	job.Snapshot.ProviderModel.ID = "model-1"
	job.Snapshot.ProviderModel.ProviderType = "openai"
	job.Snapshot.ProviderModel.Protocols = []string{"openai_responses"}
	repository := &fallbackCreditRepository{}
	creditService, err := creditsapplication.New(repository, func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if err := executor.EnableCredits(creditService); err != nil {
		t.Fatal(err)
	}
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			publishRuntimeFailure(t, events, request.RunID, "failed after usage")
			return agentruntime.Result{Usage: agentruntime.Usage{InputTokens: 10_000, Reported: true}}, errors.New("failed after usage")
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err == nil {
		t.Fatal("expected Runtime failure")
	}
	if len(result.CreditSettlements) != 1 || !result.CreditSettlements[0].UsageKnown || result.CreditSettlements[0].Estimated {
		t.Fatalf("measured failure settlement = %#v", result.CreditSettlements)
	}
	if repository.aborted {
		t.Fatal("measured failure usage was released instead of settled")
	}
}

func TestExecuteTeamStagesAndReusesIndependentNativeState(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Kind, job.SessionID = application.JobSession, "session-1"
	executor.config.Worker.Runtimes["codex"] = platformconfig.RuntimeEngineConfig{Available: true, NativeResume: true, ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("0", 64), CLIVersion: "test"}
	model := job.Snapshot.ProviderModel
	model.ID = "model-1"
	job.Snapshot = domain.ExecutionSnapshot{SchemaVersion: 2, Stages: []domain.ExecutionStageSnapshot{
		{Position: 1, Expert: &domain.ExpertSnapshot{ID: "expert-1", Name: "First", Version: 2}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: model},
		{Position: 2, Expert: &domain.ExpertSnapshot{ID: "expert-2", Name: "Second", Version: 3}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: model},
	}}
	startIndex := 0
	executor.checkout = func(context.Context, string) (runtimeLease, error) {
		return &recordingLease{start: func(config containerprocess.Config) {
			startIndex++
			if config.NativeStateDirectory == "" {
				t.Fatal("team stage has no isolated native state directory")
			}
			if err := os.WriteFile(filepath.Join(config.NativeStateDirectory, "sessions", "state.txt"), []byte(fmt.Sprintf("stage-%d", startIndex)), 0o600); err != nil {
				t.Fatal(err)
			}
		}}, nil
	}
	var checkpoints []string
	call := 0
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			call++
			checkpoints = append(checkpoints, request.CheckpointRef)
			publishSuccessfulRuntime(t, events, request.RunID, "done")
			return agentruntime.Result{FinalMessage: "done", CheckpointRef: fmt.Sprintf("checkpoint-%d", call)}, nil
		}}, nil
	}

	first, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.StageCheckpointRefs, map[int]string{1: "checkpoint-1", 2: "checkpoint-2"}) {
		t.Fatalf("stage checkpoints = %#v", first.StageCheckpointRefs)
	}
	if first.SuccessCommit == nil {
		t.Fatal("team native state has no deferred success commit")
	}
	if err := first.SuccessCommit.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := first.SuccessCommit.Cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, stage := range job.Snapshot.Stages {
		path, pathErr := workspacefs.NativeExpertSessionStatePath(executor.config.Workspace.Root, job.OwnerID, job.SessionID, stage.Expert.ID, stage.Expert.Version, "codex")
		if pathErr != nil {
			t.Fatal(pathErr)
		}
		if _, statErr := os.Stat(filepath.Join(path, "state.txt")); statErr != nil {
			t.Fatalf("stage %d native state was not promoted: %v", stage.Position, statErr)
		}
	}

	job.StageCheckpointRefs = first.StageCheckpointRefs
	call, startIndex, checkpoints = 0, 0, nil
	second, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(checkpoints, []string{"checkpoint-1", "checkpoint-2"}) {
		t.Fatalf("resumed checkpoints = %#v", checkpoints)
	}
	if second.SuccessCommit == nil {
		t.Fatal("resumed native state has no deferred success commit")
	}
	if err := second.SuccessCommit.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := second.SuccessCommit.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteExpertTeamFailsFastAndRollsBackWorkspace(t *testing.T) {
	executor, job, persistent := newTeamTestExecutor(t)
	adapterCalls := 0
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		adapterCalls++
		position := adapterCalls
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "uncommitted.txt"), []byte("partial"), 0o600); err != nil {
				return agentruntime.Result{}, err
			}
			if position == 1 {
				publishSuccessfulRuntime(t, events, request.RunID, "first result")
				return agentruntime.Result{FinalMessage: "first result"}, nil
			}
			publishRuntimeFailure(t, events, request.RunID, "second failed")
			return agentruntime.Result{}, &agentruntime.Error{Code: agentruntime.ErrorAuthenticationFailed, Message: "provider rejected test-key", Cause: errors.New("second failed test-key")}
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err == nil || !strings.Contains(err.Error(), "second failed") {
		t.Fatalf("error = %v", err)
	}
	if adapterCalls != 2 || len(result.ExpertStages) != 2 || result.ExpertStages[1].State != "failed" {
		t.Fatalf("fail-fast result = %#v, calls = %d", result, adapterCalls)
	}
	if strings.Contains(err.Error(), "test-key") || strings.Contains(result.ExpertStages[1].Error, "test-key") || !strings.Contains(result.ExpertStages[1].Error, "[REDACTED]") {
		t.Fatalf("Runtime error was not redacted: result=%#v error=%v", result.ExpertStages[1], err)
	}
	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorAuthenticationFailed {
		t.Fatalf("Runtime error code = %q", got)
	}
	if _, err := os.Stat(filepath.Join(persistent, "uncommitted.txt")); !os.IsNotExist(err) {
		t.Fatalf("failed Workspace changes were committed: %v", err)
	}
}

func TestExecuteExpertTeamUsesOneOverallTimeout(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	executor.executionTTL = 20 * time.Millisecond
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(ctx context.Context, _ agentruntime.ExecuteRequest, _ agentruntime.EventSink) (agentruntime.Result, error) {
			<-ctx.Done()
			return agentruntime.Result{}, ctx.Err()
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if !errors.Is(err, context.DeadlineExceeded) || len(result.ExpertStages) != 1 || result.ExpertStages[0].State != "cancelled" {
		t.Fatalf("timeout result = %#v, error = %v", result, err)
	}
}

func TestSessionExecutionCapturesOnlyFinalFilesAsArtifacts(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	job.Kind = application.JobSession
	job.WorkflowID = ""
	job.SessionID = "33333333-3333-4333-8333-333333333333"
	job.AssistantMessageID = 42
	job.Snapshot.ExpertTeam = nil
	executor.checkout = func(context.Context, string) (runtimeLease, error) { return &recordingLease{}, nil }
	executor.newAdapter = func(_ domain.RuntimeEngine, _ cliadapter.Config) (agentruntime.Adapter, error) {
		return &recordingAdapter{execute: func(_ context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
			if err := os.MkdirAll(filepath.Join(request.WorkspacePath, "node_modules", "example"), 0o700); err != nil {
				return agentruntime.Result{}, err
			}
			if err := os.MkdirAll(filepath.Join(request.WorkspacePath, "assets"), 0o700); err != nil {
				return agentruntime.Result{}, err
			}
			for name, content := range map[string]string{
				"node_modules/example/package.json": "{}",
				"assets/font.ttf":                   "temporary font",
				"create_report.js":                  "generate report",
			} {
				if err := os.WriteFile(filepath.Join(request.WorkspacePath, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
					return agentruntime.Result{}, err
				}
			}
			if err := os.WriteFile(filepath.Join(request.WorkspacePath, "report.md"), []byte("generated report"), 0o600); err != nil {
				return agentruntime.Result{}, err
			}
			publishSuccessfulRuntime(t, events, request.RunID, "Created `/workspace/report.md`.")
			return agentruntime.Result{FinalMessage: "Created `/workspace/report.md`."}, nil
		}}, nil
	}

	result, err := executor.Execute(context.Background(), job, &recordingProgress{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Name != "report.md" || result.Artifacts[0].TextPreview != "generated report" {
		t.Fatalf("Session Artifacts = %#v", result.Artifacts)
	}
	wantPrefix := "artifacts/owner-1/sessions/33333333-3333-4333-8333-333333333333/"
	if !strings.HasPrefix(result.Artifacts[0].ObjectKey, wantPrefix) {
		t.Fatalf("Session Artifact Object Key = %q, want prefix %q", result.Artifacts[0].ObjectKey, wantPrefix)
	}
}

func TestPersistChangedFilesSeparatesFinalArtifactFromWorkspace(t *testing.T) {
	executor, job, _ := newTeamTestExecutor(t)
	workspace := t.TempDir()
	path := filepath.Join(workspace, "report.md")
	if err := os.WriteFile(path, []byte("generated report"), 0o600); err != nil {
		t.Fatal(err)
	}

	artifacts, err := executor.persistChangedFiles(context.Background(), job, workspace, map[string]string{}, "Created `/workspace/report.md`.", credentials.NewRedactor())
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].Name != "report.md" {
		t.Fatalf("artifacts = %#v", artifacts)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("final Artifact remains in project Workspace: %v", err)
	}
}

func newTeamTestExecutor(t *testing.T) (*Executor, application.ExecutionJob, string) {
	t.Helper()
	root, credentialsRoot := t.TempDir(), t.TempDir()
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	secret, err := box.Encrypt([]byte("test-key"), "model-provider:owner-1")
	if err != nil {
		t.Fatal(err)
	}
	warm, err := containerprocess.NewWarmManager("docker", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	config := platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: root},
		Worker: platformconfig.WorkerConfig{CredentialTempRoot: credentialsRoot, SandboxUID: os.Getuid(), SandboxGID: os.Getgid(), Runtimes: map[string]platformconfig.RuntimeEngineConfig{
			"codex": {Available: true, ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("0", 64), CLIVersion: "test"},
		}},
	}
	executor, err := New(config, box, memory.New(), warm)
	if err != nil {
		t.Fatal(err)
	}
	workspacePath := "owners/owner-1/workflows/workflow-1"
	persistent := filepath.Join(root, filepath.FromSlash(workspacePath))
	if err := os.MkdirAll(persistent, 0o750); err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{
		ID: "11111111-1111-4111-8111-111111111111", Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "22222222-2222-4222-8222-222222222222", Instruction: "Solve this task",
		Snapshot: domain.ExecutionSnapshot{
			RuntimeEngine: domain.RuntimeCodex, WorkspacePath: workspacePath,
			ProviderModel: domain.ProviderModelSnapshot{ModelID: "test-model", Endpoint: "https://example.test/v1", APIKeyCiphertext: secret},
			ExpertTeam: &domain.ExpertTeamSnapshot{ID: "team-1", Members: []domain.ExpertMemberSnapshot{
				{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-1", Name: "First Expert", ExecutionInstruction: "First Expert instruction"}, Position: 1},
				{ExpertSnapshot: domain.ExpertSnapshot{ID: "expert-2", Name: "Second Expert", ExecutionInstruction: "Second Expert instruction"}, Position: 2},
			}},
		},
	}
	return executor, job, persistent
}

type recordingLease struct {
	start func(containerprocess.Config)
}

func (lease *recordingLease) Start(_ context.Context, config containerprocess.Config) (cliadapter.RunProcess, error) {
	if lease.start != nil {
		lease.start(config)
	}
	return nil, nil
}
func (*recordingLease) Release(context.Context) error { return nil }

type recordingAdapter struct {
	execute func(context.Context, agentruntime.ExecuteRequest, agentruntime.EventSink) (agentruntime.Result, error)
}

func (*recordingAdapter) Describe(context.Context) (agentruntime.Descriptor, error) {
	return agentruntime.Descriptor{Name: "recording", Version: "test", Capabilities: map[agentruntime.Capability]bool{agentruntime.CapabilityStreaming: true}}, nil
}
func (adapter *recordingAdapter) Execute(ctx context.Context, request agentruntime.ExecuteRequest, events agentruntime.EventSink) (agentruntime.Result, error) {
	return adapter.execute(ctx, request, events)
}

func publishSuccessfulRuntime(t *testing.T, sink agentruntime.EventSink, runID, message string) {
	t.Helper()
	events := []agentruntime.Event{
		{RunID: runID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
		{RunID: runID, Sequence: 2, Kind: agentruntime.EventMessageDelta, OccurredAt: time.Now(), Payload: []byte(`{"delta":"` + message + `"}`)},
		{RunID: runID, Sequence: 3, Kind: agentruntime.EventMessageCompleted, OccurredAt: time.Now(), Payload: []byte(`{"message":"` + message + `"}`)},
		{RunID: runID, Sequence: 4, Kind: agentruntime.EventRuntimeCompleted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
	}
	for _, event := range events {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

func publishRuntimeFailure(t *testing.T, sink agentruntime.EventSink, runID, message string) {
	t.Helper()
	for _, event := range []agentruntime.Event{
		{RunID: runID, Sequence: 1, Kind: agentruntime.EventRuntimeStarted, OccurredAt: time.Now(), Payload: []byte(`{}`)},
		{RunID: runID, Sequence: 2, Kind: agentruntime.EventRuntimeFailed, OccurredAt: time.Now(), Payload: []byte(`{"error":"` + message + `"}`)},
	} {
		if err := sink.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
}

type recordingProgress struct{ events []application.ExecutionEvent }

func commitSuccessfulResult(t *testing.T, result application.ExecutionResult) {
	t.Helper()
	if result.SuccessCommit == nil {
		t.Fatal("successful Workflow result has no deferred commit")
	}
	if err := result.SuccessCommit.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := result.SuccessCommit.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func (recorder *recordingProgress) RecordProgress(_ context.Context, _ application.ExecutionJob, event application.ExecutionEvent) error {
	recorder.events = append(recorder.events, event)
	return nil
}

type fallbackCreditRepository struct {
	creditsapplication.Repository
	aborted bool
}

func (*fallbackCreditRepository) ResolveRate(context.Context, creditsdomain.ModelRateKey) (creditsdomain.ModelCreditRate, error) {
	return creditsdomain.ModelCreditRate{RevisionID: "default-1", InputMultiplierMicros: creditsdomain.MultiplierScale, OutputMultiplierMicros: creditsdomain.MultiplierScale, Fallback: creditsdomain.DefaultFallback}, nil
}

func (*fallbackCreditRepository) Admit(_ context.Context, admission creditsdomain.Admission) (creditsdomain.Admission, error) {
	return admission, nil
}

func (repository *fallbackCreditRepository) Abort(context.Context, creditsdomain.Admission) error {
	repository.aborted = true
	return nil
}

func TestSanitizeNativeStateRemovesTransientConfigAndRedactsSessionFiles(t *testing.T) {
	root := t.TempDir()
	secret := "native-state-secret-canary"
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("token = \""+secret+"\""), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(root, "sessions", "2026", "08")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(sessions, "rollout.jsonl")
	if err := os.WriteFile(rollout, []byte(`{"message":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sanitizeNativeState(root, credentials.NewRedactor([]byte(secret))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("transient config remains: %v", err)
	}
	content, err := os.ReadFile(rollout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), secret) || !strings.Contains(string(content), "[REDACTED]") {
		t.Fatalf("session state was not redacted: %s", content)
	}
}

func TestFirstWorkflowCommitCreatesOwnerDirectory(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "first-workflow-commit-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	uid, gid := os.Getuid(), os.Getgid()
	if os.Geteuid() == 0 {
		uid, gid = 65532, 65532
	}
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: root},
		Worker:    platformconfig.WorkerConfig{SandboxUID: uid, SandboxGID: gid},
	}}
	job := application.ExecutionJob{Kind: application.JobWorkflow, Snapshot: domain.ExecutionSnapshot{WorkspacePath: "workflows/new-owner/new-workflow"}}
	workspace, persistent, _, err := executor.stageWorkspaceAt(job, filepath.Join(root, ".runtime-containers", "slot", "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "result.txt"), []byte("first result"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := preparePersistentWorkspaceTree(workspace, uid, gid); err != nil {
		t.Fatal(err)
	}
	commit := &nativeStateCommit{promotions: []nativeStatePromotion{{temporary: workspace, persistent: persistent}}, temporaryRoots: []string{workspace}}
	if err := commit.Commit(); err != nil {
		t.Fatalf("first Workflow Workspace commit: %v", err)
	}
	for _, directory := range []string{filepath.Join(root, "workflows"), filepath.Dir(persistent)} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatal(err)
		}
		owner := info.Sys().(*syscall.Stat_t)
		if int(owner.Uid) != uid || int(owner.Gid) != gid || info.Mode().Perm() != 0o700 {
			t.Fatalf("persistent parent owner/mode = %d:%d %o, want %d:%d 700", owner.Uid, owner.Gid, info.Mode().Perm(), uid, gid)
		}
	}
	content, err := os.ReadFile(filepath.Join(persistent, "result.txt"))
	if err != nil || string(content) != "first result" {
		t.Fatalf("persistent Workspace content = %q, error = %v", content, err)
	}
	if os.Geteuid() == 0 {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		input, err := os.Open(binary)
		if err != nil {
			t.Fatal(err)
		}
		accessibleBinary := filepath.Join(root, "runtimeexecutor.test")
		output, err := os.OpenFile(accessibleBinary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
		if err != nil {
			_ = input.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(output, input)
		if err := errors.Join(copyErr, input.Close(), output.Close()); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(accessibleBinary, "-test.run=^TestFirstWorkflowPlatformAccessHelper$")
		command.Env = append(os.Environ(), "TEST_FIRST_WORKFLOW_ROOT="+root)
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("platform UID cannot manage first Workflow Workspace: %v\n%s", err, output)
		}
	}
	if err := commit.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(persistent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback left new persistent Workspace: %v", err)
	}
	if err := commit.Cleanup(); err != nil {
		t.Fatal(err)
	}
}

func TestFirstWorkflowPlatformAccessHelper(t *testing.T) {
	root := os.Getenv("TEST_FIRST_WORKFLOW_ROOT")
	if root == "" {
		return
	}
	store, err := workspacefs.New(root, "")
	if err != nil {
		t.Fatal(err)
	}
	path := "workflows/new-owner/new-workflow"
	if _, _, err := store.List(context.Background(), path, ""); err != nil {
		t.Fatalf("list first Workspace: %v", err)
	}
	if err := store.Clear(context.Background(), path); err != nil {
		t.Fatalf("clear first Workspace: %v", err)
	}
}

func TestStateParentRejectsSymlinkAndPreservesExistingOwnership(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := createOwnedStateParent(filepath.Join(root, "link", "new"), os.Getuid(), os.Getgid()); err == nil {
		t.Fatal("persistent parent followed a symbolic link")
	}
	if err := os.Chmod(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := createOwnedStateParent(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("existing parent permissions changed: %v, %v", info, err)
	}
}

func TestNativeStateCommitCanRollbackAllPromotedMembers(t *testing.T) {
	root := t.TempDir()
	commit := &nativeStateCommit{}
	for _, name := range []string{"first", "second"} {
		persistent := filepath.Join(root, name, "persistent")
		temporaryRoot := filepath.Join(root, name, "temporary")
		temporary := filepath.Join(temporaryRoot, "sessions")
		if err := os.MkdirAll(persistent, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(temporary, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(persistent, "state"), []byte("old-"+name), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temporary, "state"), []byte("new-"+name), 0o600); err != nil {
			t.Fatal(err)
		}
		commit.promotions = append(commit.promotions, nativeStatePromotion{temporary: temporary, persistent: persistent})
		commit.temporaryRoots = append(commit.temporaryRoots, temporaryRoot)
	}

	if err := commit.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := commit.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first", "second"} {
		content, err := os.ReadFile(filepath.Join(root, name, "persistent", "state"))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "old-"+name {
			t.Fatalf("%s state after rollback = %q", name, content)
		}
	}
}

func TestPreparePersistentWorkspaceTreeMakesMergedFilesReadableByThePlatformUser(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "output")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "result.txt")
	if err := os.WriteFile(file, []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := preparePersistentWorkspaceTree(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{root: 0o700, directory: 0o700, file: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode = %o, want %o", path, got, want)
		}
	}
}

func TestPrepareRuntimeScratchCreatesAnInitiallyMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot", "scratch")
	if err := prepareRuntimeScratch(path, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o750 {
		t.Fatalf("Runtime scratch directory = mode %o, directory=%t", info.Mode().Perm(), info.IsDir())
	}
	attachmentInfo, err := os.Stat(filepath.Join(path, "attachments"))
	if err != nil {
		t.Fatal(err)
	}
	if !attachmentInfo.IsDir() || attachmentInfo.Mode().Perm() != 0o750 {
		t.Fatalf("Runtime attachment directory = mode %o, directory=%t", attachmentInfo.Mode().Perm(), attachmentInfo.IsDir())
	}
}

func TestAnonymousWorkflowConversationRestoresNativeState(t *testing.T) {
	root := t.TempDir()
	persistent, err := workspacefs.NativeRunConversationStatePath(root, "owner-1", "conversation-1", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(persistent, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(persistent, "rollout.jsonl"), []byte("saved session"), 0o600); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: root},
		Worker:    platformconfig.WorkerConfig{SandboxUID: os.Getuid(), SandboxGID: os.Getgid()},
	}}
	job := application.ExecutionJob{
		Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "workflow-1", ConversationID: "conversation-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}
	temporary := filepath.Join(root, ".runtime-containers", "slot", "native-state")
	staged, gotPersistent, err := executor.nativeStateDirectoriesAt(job, platformconfig.RuntimeEngineConfig{NativeResume: true}, temporary)
	if err != nil {
		t.Fatal(err)
	}
	if staged != temporary || gotPersistent != persistent {
		t.Fatalf("native state directories = %q, %q; want %q, %q", staged, gotPersistent, temporary, persistent)
	}
	contents, err := os.ReadFile(filepath.Join(staged, "sessions", "rollout.jsonl"))
	if err != nil || string(contents) != "saved session" {
		t.Fatalf("restored native state = %q, %v", contents, err)
	}
}

func TestResolveRuntimeCheckpointFallsBackWhenWorkflowNativeStateIsMissing(t *testing.T) {
	nativeState := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nativeState, "sessions"), 0o750); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := resolveRuntimeCheckpoint(application.JobWorkflow, "thread-1", nativeState)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint != "" {
		t.Fatalf("checkpoint = %q, want a fresh Workflow invocation", checkpoint)
	}

	rollout := filepath.Join(nativeState, "sessions", "2026", "09", "rollout.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rollout, []byte("saved session"), 0o600); err != nil {
		t.Fatal(err)
	}
	checkpoint, err = resolveRuntimeCheckpoint(application.JobWorkflow, "thread-1", nativeState)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint != "thread-1" {
		t.Fatalf("checkpoint = %q, want persisted checkpoint", checkpoint)
	}
}

func TestPrepareRuntimeAttachmentMountpointRejectsWorkspaceCollision(t *testing.T) {
	workspace := t.TempDir()
	reserved := filepath.Join(workspace, ".agent-platform-attachments")
	if err := os.WriteFile(reserved, []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := prepareRuntimeAttachmentMountpoint(workspace, os.Getuid(), os.Getgid()); err == nil || !strings.Contains(err.Error(), "reserved Runtime attachment path") {
		t.Fatalf("collision error = %v", err)
	}
	contents, err := os.ReadFile(reserved)
	if err != nil || string(contents) != "user data" {
		t.Fatalf("reserved Workspace entry was changed: %q, %v", contents, err)
	}
}

func TestWarmSlotIsStablePerResourceAndRuntime(t *testing.T) {
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: "/workspaces"},
		Worker:    platformconfig.WorkerConfig{CredentialTempRoot: "/credentials"},
	}}
	runtime := platformconfig.RuntimeEngineConfig{ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("a", 64)}
	first, slot, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	repeat, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	workflow, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "workflow-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if first != repeat || first == workflow {
		t.Fatalf("warm container names: first=%q repeat=%q workflow=%q", first, repeat, workflow)
	}
	if !strings.Contains(slot.workspace, strings.TrimPrefix(first, "agent-runtime-warm-")) || !strings.HasPrefix(slot.credentials, "/credentials/") {
		t.Fatalf("warm slot paths = %+v", slot)
	}
	for _, kind := range []application.JobKind{application.JobSession, application.JobWorkflow} {
		job := application.ExecutionJob{Kind: kind, OwnerID: "owner-1", SessionID: "session-1", WorkflowID: "workflow-1", ConversationID: "conversation-1", Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex, SelectionKey: "with-skill"}}
		before, _, err := executor.warmSlot(job, runtime)
		if err != nil {
			t.Fatal(err)
		}
		job.Snapshot.SelectionKey = "without-skill"
		after, _, err := executor.warmSlot(job, runtime)
		if err != nil {
			t.Fatal(err)
		}
		if before == after {
			t.Fatal("removed resources reused warm Runtime state")
		}
	}

}

func TestSelectedManagedMCPConnectorMountsFrozenPackageSkills(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	entries := map[string]string{
		"connector-meta.json":  `{"source":"example-mcp","version":"1.0.0","type":"mcp","name":"Example","description":"Example CLI","examples_zh":["发送"],"examples_en":["Send"],"minPlatformVersion":"1.0.0","auth_mode":"none"}`,
		"icon.svg":             `<svg xmlns="http://www.w3.org/2000/svg"></svg>`,
		"mcp.json":             `{"transport":"streamable_http","url":"https://mcp.example.com","egress_hosts":["mcp.example.com"],"timeout_seconds":60}`,
		"skills/send/SKILL.md": "---\nname: example-send\ndisplay_name: Send\ndescription: Send messages\nversion: 1.0.0\nauthor: Example\n---\n\n# Send\nUse the reviewed send command.\n",
	}
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	provider := memory.New()
	digest := sha256.Sum256(archive.Bytes())
	sha := hex.EncodeToString(digest[:])
	key := "connectors/example-mcp/1.0.0/" + sha + ".zip"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(archive.Bytes()), objectstore.PutOptions{Size: int64(archive.Len()), SHA256: sha, ContentType: "application/zip"}); err != nil {
		t.Fatal(err)
	}
	connector := domain.MCPServerSnapshot{ID: "installed-1", Name: "Example", Transport: "streamable_http", Configuration: json.RawMessage(`{"url":"https://mcp.example.com","egress_hosts":["mcp.example.com"]}`), PackageObjectKey: key, PackageSHA256: sha}
	job := application.ExecutionJob{Snapshot: domain.ExecutionSnapshot{MCPServers: []domain.MCPServerSnapshot{connector}, ProviderModel: domain.ProviderModelSnapshot{ModelID: "model", Endpoint: "https://models.example.test", ProviderType: "anthropic", Protocols: []string{"anthropic_messages"}}}}
	files, _, _, err := (&Executor{objects: provider}).extensionFiles(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(files["connector-skills/installed-1/SKILL.md"], []byte(entries["skills/send/SKILL.md"])) || !bytes.Equal(files["connector-skills/installed-1/skills/send/SKILL.md"], []byte(entries["skills/send/SKILL.md"])) {
		t.Fatalf("mounted Connector Skills = %#v", files)
	}
	if !strings.Contains(buildInstruction(job, nil), "first read /run/agent-credentials/connector-skills/installed-1/SKILL.md") {
		t.Fatal("managed Connector Skill was not prioritized")
	}
	job.Snapshot.MCPServers[0].PackageSHA256 = strings.Repeat("b", 64)
	if _, _, _, err := (&Executor{objects: provider}).extensionFiles(context.Background(), job); err == nil {
		t.Fatal("expected a changed frozen package digest to be rejected")
	}
}
