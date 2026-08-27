package runtimeexecutor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/sandbox"
)

var errMCPHandshakeObserved = errors.New("MCP handshake observed")

type mcpProbeConfiguration struct {
	URL            *string  `json:"url"`
	Runner         *string  `json:"runner"`
	Package        *string  `json:"package"`
	PackageVersion *string  `json:"package_version"`
	Arguments      []string `json:"arguments"`
	Environment    []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Secret bool   `json:"secret"`
	} `json:"environment"`
}

func (executor *Executor) testMCP(ctx context.Context, job application.ExecutionJob) (result application.ExecutionResult, returnErr error) {
	var configuration mcpProbeConfiguration
	if err := json.Unmarshal(job.MCPServer.Configuration, &configuration); err != nil {
		return result, fmt.Errorf("decode MCP test configuration: %w", err)
	}
	variables := make(map[string]string)
	files := make(map[string][]byte)
	for _, variable := range configuration.Environment {
		if !variable.Secret {
			variables[variable.Name] = variable.Value
		}
	}
	if len(job.MCPServer.SecretCiphertext) > 0 {
		plaintext, err := executor.box.Decrypt(job.MCPServer.SecretCiphertext, "mcp-server:"+job.OwnerID)
		if err != nil {
			return result, fmt.Errorf("decrypt MCP test environment: %w", err)
		}
		defer clear(plaintext)
		var secretValues map[string]string
		if err := json.Unmarshal(plaintext, &secretValues); err != nil {
			return result, fmt.Errorf("decode MCP test environment: %w", err)
		}
		for name, value := range secretValues {
			variables[name] = value
		}
		if token := secretValues["MCP_BEARER_TOKEN"]; token != "" {
			files["mcp/http-headers"] = []byte("Authorization: Bearer " + token + "\n")
		}
	}
	environment, err := executor.materializer.Create(credentials.Request{Ref: job.ID, Variables: variables, Files: files})
	if err != nil {
		return result, err
	}
	defer func() { returnErr = errors.Join(returnErr, environment.Cleanup()) }()

	workspace, err := os.MkdirTemp(executor.config.Workspace.Root, ".mcp-test-*")
	if err != nil {
		return result, fmt.Errorf("create MCP test Workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	if err := prepareSandboxDirectory(workspace, executor.config.Worker.SandboxUID, executor.config.Worker.SandboxGID); err != nil {
		return result, err
	}

	command, runtimeName, observer, err := mcpProbeCommand(configuration, len(files) > 0)
	if err != nil {
		return result, err
	}
	runtimeConfig, ok := executor.config.Worker.Runtimes[runtimeName]
	if !ok || !runtimeConfig.Available {
		return result, fmt.Errorf("MCP test Runtime %s is unavailable", runtimeName)
	}
	runProcess, err := containerprocess.New(containerprocess.Config{
		Image: runtimeConfig.ImageDigest, RuntimeCommand: command[0], DirectCommand: true,
		RunID: job.ID, Runtime: executor.config.Sandbox.Runtime,
		WorkspaceDirectory: workspace, ContainerWorkspace: "/workspace", CredentialDirectory: environment.Directory(),
		PublicEgressNetwork: executor.config.Sandbox.EgressNetwork, ResolverConfigFile: executor.config.Sandbox.ResolverConfig,
		Egress: sandbox.EgressPublic, Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 1 << 30, PIDs: 128, TempBytes: 512 << 20},
		UID: executor.config.Worker.SandboxUID, GID: executor.config.Worker.SandboxGID,
	})
	if err != nil {
		return result, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	initialize := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-03-26\",\"capabilities\":{},\"clientInfo\":{\"name\":\"agent-workspace\",\"version\":\"1\"}}}\n")
	_, err = runProcess(probeCtx, processharness.Spec{
		Command: command, Dir: workspace, Stdin: initialize, Observer: observer,
		MaxOutputBytes: 1 << 20, MaxLineBytes: 256 << 10, GracePeriod: 2 * time.Second,
	}, processharness.NewRedactingSink(environment.Redactor(), discardOutput{}))
	if errors.Is(err, errMCPHandshakeObserved) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("isolated MCP test failed: %w", err)
	}
	return result, nil
}

func mcpProbeCommand(configuration mcpProbeConfiguration, bearerToken bool) ([]string, string, processharness.OutputObserver, error) {
	if configuration.URL != nil {
		command := []string{
			"curl", "--fail-with-body", "--silent", "--show-error", "--max-time", "20",
			"--request", "POST", "--header", "Content-Type: application/json",
			"--header", "Accept: application/json, text/event-stream",
		}
		if bearerToken {
			command = append(command, "--header", "@/run/agent-credentials/mcp/http-headers")
		}
		command = append(command, "--data-binary", "@-", *configuration.URL)
		return command, "codex", nil, nil
	}
	if configuration.Runner == nil || configuration.Package == nil || configuration.PackageVersion == nil {
		return nil, "", nil, fmt.Errorf("incomplete stdio MCP configuration")
	}
	packageAtVersion := *configuration.Package + "@" + *configuration.PackageVersion
	switch *configuration.Runner {
	case "npx":
		return append([]string{"npx", "--yes", packageAtVersion}, configuration.Arguments...), "codex", &mcpHandshakeObserver{}, nil
	case "uvx":
		return append([]string{"uvx", *configuration.Package + "==" + *configuration.PackageVersion}, configuration.Arguments...), "hermes", &mcpHandshakeObserver{}, nil
	default:
		return nil, "", nil, fmt.Errorf("unsupported stdio MCP runner")
	}
}

type mcpHandshakeObserver struct {
	mu     sync.Mutex
	stdout bytes.Buffer
}

func (observer *mcpHandshakeObserver) Observe(_ context.Context, stream processharness.Stream, data []byte) error {
	if stream != processharness.StreamStdout {
		return nil
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.stdout.Len()+len(data) > 256<<10 {
		return fmt.Errorf("MCP handshake exceeds 256 KiB")
	}
	_, _ = observer.stdout.Write(data)
	for _, line := range bytes.Split(observer.stdout.Bytes(), []byte{'\n'}) {
		var response struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   json.RawMessage `json:"error"`
		}
		if json.Unmarshal(bytes.TrimSpace(line), &response) == nil && response.JSONRPC == "2.0" && string(response.ID) == "1" && (len(response.Result) > 0 || len(response.Error) > 0) {
			return errMCPHandshakeObserved
		}
	}
	return nil
}

func prepareSandboxDirectory(path string, uid, gid int) error {
	if err := os.Chmod(path, 0o750); err != nil {
		return fmt.Errorf("protect sandbox Workspace: %w", err)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("set sandbox Workspace owner: %w", err)
	}
	return nil
}
