package openclaw

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
)

const Version = "2026.7.1-2"

const providerName = "agent-workspace"

type Driver struct{}

func New(config cliadapter.Config) *cliadapter.Adapter {
	if len(config.Command) == 0 {
		config.Command = []string{"openclaw"}
	}
	if config.ExpectedVersion == "" {
		config.ExpectedVersion = Version
	}
	return cliadapter.New(Driver{}, config)
}

func (Driver) Name() string { return "openclaw" }

func (Driver) VersionArgs() []string { return []string{"--version"} }

func (Driver) ParseVersion(output string) (string, error) {
	for _, field := range strings.Fields(output) {
		candidate := strings.TrimPrefix(strings.TrimSpace(field), "v")
		if candidate == Version {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unexpected OpenClaw version output %q", output)
}

func (Driver) Build(request agentruntime.ExecuteRequest, scratchDirectory string) (cliadapter.Invocation, error) {
	if request.CheckpointRef != "" {
		return cliadapter.Invocation{}, fmt.Errorf("OpenClaw native resume is not verified")
	}
	model, err := modelReference(request.Model)
	if err != nil {
		return cliadapter.Invocation{}, err
	}
	promptPath := filepath.Join(scratchDirectory, "instruction.txt")
	if err := os.WriteFile(promptPath, []byte(request.Instruction), 0o600); err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("write OpenClaw instruction: %w", err)
	}
	encoded, err := EncodeRuntimeConfig(request, nil)
	if err != nil {
		return cliadapter.Invocation{}, err
	}
	configPath := request.MCPConfigPath
	if configPath == "" {
		configPath = filepath.Join(scratchDirectory, "openclaw.json")
		if err := os.WriteFile(configPath, append(encoded, '\n'), 0o600); err != nil {
			return cliadapter.Invocation{}, fmt.Errorf("write OpenClaw runtime config: %w", err)
		}
	}
	return cliadapter.Invocation{Env: []string{"OPENCLAW_CONFIG_PATH=" + configPath}, Args: []string{
		"agent",
		"--local",
		"--agent", "main",
		"--session-key", request.RunID,
		"--message-file", promptPath,
		"--model", model,
		"--timeout", "0",
		"--json",
	}}, nil
}

type MCPServer struct {
	Enabled   bool              `json:"enabled"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Transport string            `json:"transport,omitempty"`
}

type providerConfiguration struct {
	BaseURL string               `json:"baseUrl"`
	APIKey  string               `json:"apiKey"`
	API     string               `json:"api"`
	Models  []modelConfiguration `json:"models"`
}

type modelConfiguration struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Input []string `json:"input"`
}

// EncodeRuntimeConfig keeps OpenClaw's model and MCP configuration in one per-run file.
func EncodeRuntimeConfig(request agentruntime.ExecuteRequest, servers map[string]MCPServer) ([]byte, error) {
	model, err := modelReference(request.Model)
	if err != nil {
		return nil, err
	}
	protocol, apiKey, defaultEndpoint, err := modelProtocol(request.ModelProtocols)
	if err != nil {
		return nil, err
	}
	endpoint, err := modelEndpoint(request.ModelEndpoint, defaultEndpoint)
	if err != nil {
		return nil, err
	}
	config := struct {
		Models struct {
			Mode      string                           `json:"mode"`
			Providers map[string]providerConfiguration `json:"providers"`
		} `json:"models"`
		Agents struct {
			Defaults struct {
				Model struct {
					Primary string `json:"primary"`
				} `json:"model"`
				Models map[string]map[string]map[string]string `json:"models"`
			} `json:"defaults"`
		} `json:"agents"`
		Plugins struct {
			Allow            []string                   `json:"allow"`
			BundledDiscovery string                     `json:"bundledDiscovery"`
			Slots            map[string]string          `json:"slots"`
			Entries          map[string]map[string]bool `json:"entries"`
		} `json:"plugins"`
		MCP *struct {
			Servers map[string]MCPServer `json:"servers"`
		} `json:"mcp,omitempty"`
	}{}
	config.Models.Mode = "replace"
	modelID := strings.TrimSpace(request.Model)
	config.Models.Providers = map[string]providerConfiguration{providerName: {
		BaseURL: endpoint, APIKey: apiKey, API: protocol,
		Models: []modelConfiguration{{ID: modelID, Name: modelID, Input: []string{"text"}}},
	}}
	config.Agents.Defaults.Model.Primary = model
	config.Agents.Defaults.Models = map[string]map[string]map[string]string{
		model: {"agentRuntime": {"id": "openclaw"}},
	}
	config.Plugins.Allow = []string{}
	config.Plugins.BundledDiscovery = "allowlist"
	config.Plugins.Slots = map[string]string{"memory": "none"}
	config.Plugins.Entries = map[string]map[string]bool{}
	if len(servers) > 0 {
		config.MCP = &struct {
			Servers map[string]MCPServer `json:"servers"`
		}{Servers: servers}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode OpenClaw runtime config: %w", err)
	}
	return encoded, nil
}

func modelReference(value string) (string, error) {
	model := strings.TrimSpace(value)
	if model == "" {
		return "", fmt.Errorf("OpenClaw model is required")
	}
	lower := strings.ToLower(model)
	for _, forbidden := range []string{"claude-cli", "codex-cli", "cli-backend"} {
		if strings.Contains(lower, forbidden) {
			return "", fmt.Errorf("OpenClaw nested CLI backend %q is forbidden", value)
		}
	}
	return providerName + "/" + model, nil
}

func modelEndpoint(value, defaultValue string) (string, error) {
	if value == "" {
		value = defaultValue
	}
	endpoint, err := url.Parse(value)
	scheme := ""
	if endpoint != nil {
		scheme = strings.ToLower(endpoint.Scheme)
	}
	if err != nil || endpoint == nil || (scheme != "http" && scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", fmt.Errorf("OpenClaw model endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
	}
	return endpoint.String(), nil
}

func modelProtocol(protocols []string) (string, string, string, error) {
	protocol := "openai_responses"
	if len(protocols) > 0 {
		protocol = protocols[0]
	}
	switch protocol {
	case "openai_responses":
		return "openai-responses", "${OPENAI_API_KEY}", "https://api.openai.com/v1", nil
	case "openai_chat":
		return "openai-completions", "${OPENAI_API_KEY}", "https://api.openai.com/v1", nil
	case "anthropic_messages":
		return "anthropic-messages", "${ANTHROPIC_API_KEY}", "https://api.anthropic.com", nil
	case "gemini":
		return "google-generative-ai", "${OPENAI_API_KEY}", "https://generativelanguage.googleapis.com/v1beta", nil
	default:
		return "", "", "", fmt.Errorf("OpenClaw requires OpenAI Responses, OpenAI Chat, Anthropic Messages, or Gemini protocol")
	}
}

func (Driver) NewParser(string) cliadapter.Parser { return &parser{} }

type parser struct {
	stdout []string
	stderr []string
	loaded bool
	result cliadapter.ParsedResult
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStdout {
		p.stdout = append(p.stdout, string(line))
		var response struct {
			Payloads []struct {
				Text string `json:"text"`
			} `json:"payloads"`
		}
		if json.Unmarshal(line, &response) == nil {
			events := make([]cliadapter.ParsedEvent, 0, len(response.Payloads))
			for _, payload := range response.Payloads {
				if payload.Text != "" {
					events = append(events, cliadapter.ParsedEvent{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": payload.Text}})
				}
			}
			return events, nil
		}
	} else if value := strings.TrimSpace(string(line)); value != "" {
		if len(value) > 4096 {
			value = value[len(value)-4096:]
		}
		p.stderr = append(p.stderr, value)
		if len(p.stderr) > 16 {
			p.stderr = p.stderr[len(p.stderr)-16:]
		}
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult {
	if p.loaded {
		return p.result
	}
	p.loaded = true
	var response struct {
		Payloads []struct {
			Text string `json:"text"`
		} `json:"payloads"`
		Meta struct {
			AgentMeta struct {
				SessionID string `json:"sessionId"`
			} `json:"agentMeta"`
		} `json:"meta"`
	}
	stdout := strings.Join(p.stdout, "\n")
	if strings.TrimSpace(stdout) == "" {
		diagnostic := strings.Join(p.stderr, "\n")
		lower := strings.ToLower(diagnostic)
		if strings.Contains(lower, "authentication failed") || strings.Contains(lower, "invalid_api_key") || strings.Contains(lower, "status=401") {
			p.result.Error = &agentruntime.Error{
				Code: agentruntime.ErrorAuthenticationFailed, Message: "OpenClaw model authentication failed",
				Cause: fmt.Errorf("OpenClaw returned no JSON result: %s", diagnostic),
			}
			return p.result
		}
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		p.result.Error = fmt.Errorf("decode OpenClaw JSON output: %w", err)
		return p.result
	}
	messages := make([]string, 0, len(response.Payloads))
	for _, payload := range response.Payloads {
		if payload.Text != "" {
			messages = append(messages, payload.Text)
		}
	}
	p.result.FinalMessage = strings.Join(messages, "\n\n")
	p.result.CheckpointRef = response.Meta.AgentMeta.SessionID
	return p.result
}
