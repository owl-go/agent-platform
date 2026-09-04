package runtimeexecutor

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/openclaw"
	"agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"

	"github.com/pelletier/go-toml/v2"
)

type storedMCPConfiguration struct {
	URL            *string                               `json:"url"`
	Runner         *string                               `json:"runner"`
	Package        *string                               `json:"package"`
	PackageVersion *string                               `json:"package_version"`
	Arguments      []string                              `json:"arguments"`
	Environment    []workspacedomain.EnvironmentVariable `json:"environment"`
}

type nativeMCPServer struct {
	Command string            `json:"command,omitempty" toml:"command,omitempty"`
	Args    []string          `json:"args,omitempty" toml:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty" toml:"env,omitempty"`
	URL     string            `json:"url,omitempty" toml:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty" toml:"-"`
}

type codexMCPServer struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
	URL               string            `toml:"url,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
}

func (executor *Executor) nativeMCPFiles(job application.ExecutionJob) (map[string][]byte, map[string]string, [][]byte, error) {
	files := make(map[string][]byte)
	variables := make(map[string]string)
	var redactValues [][]byte
	if len(job.Snapshot.MCPServers) == 0 {
		return files, variables, redactValues, nil
	}
	claudeServers := make(map[string]nativeMCPServer)
	codexServers := make(map[string]codexMCPServer)
	hermesServers := make(map[string]nativeMCPServer)
	openClawServers := make(map[string]openclaw.MCPServer)
	for _, server := range job.Snapshot.MCPServers {
		var configuration storedMCPConfiguration
		if err := json.Unmarshal(server.Configuration, &configuration); err != nil {
			return nil, nil, nil, fmt.Errorf("decode MCP Server %q: %w", server.Name, err)
		}
		secretValues := make(map[string]string)
		if len(server.SecretCiphertext) > 0 {
			plaintext, err := executor.box.Decrypt(server.SecretCiphertext, "mcp-server:"+job.OwnerID)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("decrypt MCP Server %q secrets: %w", server.Name, err)
			}
			if err := json.Unmarshal(plaintext, &secretValues); err != nil {
				clear(plaintext)
				return nil, nil, nil, fmt.Errorf("decode MCP Server %q secrets: %w", server.Name, err)
			}
			clear(plaintext)
			for _, value := range secretValues {
				redactValues = append(redactValues, []byte(value))
			}
		}
		environment := make(map[string]string)
		for _, variable := range configuration.Environment {
			if variable.Secret {
				environment[variable.Name] = secretValues[variable.Name]
			} else {
				environment[variable.Name] = variable.Value
			}
		}
		if server.Transport == "streamable_http" {
			if configuration.URL == nil {
				return nil, nil, nil, fmt.Errorf("MCP Server %q is missing its URL", server.Name)
			}
			native := nativeMCPServer{URL: *configuration.URL}
			codex := codexMCPServer{URL: *configuration.URL}
			openClaw := openclaw.MCPServer{Enabled: true, URL: *configuration.URL, Transport: "streamable-http"}
			if token := secretValues["MCP_BEARER_TOKEN"]; token != "" {
				native.Headers = map[string]string{"Authorization": "Bearer " + token}
				openClaw.Headers = native.Headers
				variableName := mcpTokenVariable(server.ID)
				variables[variableName] = token
				codex.BearerTokenEnvVar = variableName
			}
			claudeServers[server.Name] = native
			codexServers[server.Name] = codex
			hermesServers[server.Name] = native
			openClawServers[server.Name] = openClaw
			continue
		}
		if configuration.Runner == nil || configuration.Package == nil || configuration.PackageVersion == nil {
			return nil, nil, nil, fmt.Errorf("MCP Server %q has an incomplete stdio configuration", server.Name)
		}
		command := *configuration.Runner
		arguments := mcpPackageArguments(command, *configuration.Package, *configuration.PackageVersion, configuration.Arguments)
		native := nativeMCPServer{Command: command, Args: arguments, Env: environment}
		claudeServers[server.Name] = native
		codexServers[server.Name] = codexMCPServer{Command: command, Args: arguments, Env: environment}
		hermesServers[server.Name] = native
		openClawServers[server.Name] = openclaw.MCPServer{Enabled: true, Command: command, Args: arguments, Env: environment}
	}
	claudeConfig, err := json.Marshal(map[string]any{"mcpServers": claudeServers})
	if err != nil {
		return nil, nil, nil, err
	}
	codexConfig, err := toml.Marshal(struct {
		MCPServers map[string]codexMCPServer `toml:"mcp_servers"`
	}{MCPServers: codexServers})
	if err != nil {
		return nil, nil, nil, err
	}
	hermesConfig, err := json.Marshal(map[string]any{"mcp_servers": hermesServers})
	if err != nil {
		return nil, nil, nil, err
	}
	openClawConfig, err := openclaw.EncodeRuntimeConfig(agentruntime.ExecuteRequest{
		Model: job.Snapshot.ProviderModel.ModelID, ModelEndpoint: job.Snapshot.ProviderModel.Endpoint,
		ModelProvider: job.Snapshot.ProviderModel.ProviderType, ModelProtocols: job.Snapshot.ProviderModel.Protocols,
	}, openClawServers)
	if err != nil {
		return nil, nil, nil, err
	}
	files["extensions/claude-mcp.json"] = claudeConfig
	files["extensions/codex-config.toml"] = codexConfig
	files["runtime-home/.hermes/config.yaml"] = hermesConfig
	files["extensions/openclaw.json"] = openClawConfig
	return files, variables, redactValues, nil
}

func mcpPackageArguments(runner, packageName, version string, arguments []string) []string {
	prefix := []string{"--yes", packageName + "@" + version}
	if runner == "uvx" {
		prefix = []string{packageName + "==" + version}
	}
	return append(prefix, arguments...)
}

func mcpTokenVariable(identifier string) string {
	value := strings.ToUpper(strings.ReplaceAll(identifier, "-", ""))
	if len(value) > 16 {
		value = value[:16]
	}
	return "AGENT_MCP_" + value + "_TOKEN"
}
