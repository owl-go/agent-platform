package openclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
)

const Version = "2026.7.1-2"

var providerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

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
	model := strings.ToLower(request.Model)
	for _, forbidden := range []string{"claude-cli", "codex-cli", "cli-backend"} {
		if strings.Contains(model, forbidden) {
			return cliadapter.Invocation{}, fmt.Errorf("OpenClaw nested CLI backend %q is forbidden", request.Model)
		}
	}
	provider, modelID, found := strings.Cut(model, "/")
	if !found || !providerPattern.MatchString(provider) || strings.TrimSpace(modelID) == "" {
		return cliadapter.Invocation{}, fmt.Errorf("OpenClaw model must use provider/model format")
	}
	promptPath := filepath.Join(scratchDirectory, "instruction.txt")
	if err := os.WriteFile(promptPath, []byte(request.Instruction), 0o600); err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("write OpenClaw instruction: %w", err)
	}
	configPath := request.MCPConfigPath
	if configPath == "" {
		configPath = filepath.Join(scratchDirectory, "openclaw.json")
	}
	config := struct {
		Plugins struct {
			Allow            []string                   `json:"allow"`
			BundledDiscovery string                     `json:"bundledDiscovery"`
			Slots            map[string]string          `json:"slots"`
			Entries          map[string]map[string]bool `json:"entries"`
		} `json:"plugins"`
	}{}
	config.Plugins.Allow = []string{provider}
	config.Plugins.BundledDiscovery = "allowlist"
	config.Plugins.Slots = map[string]string{"memory": "none"}
	config.Plugins.Entries = map[string]map[string]bool{provider: {"enabled": true}}
	if request.MCPConfigPath == "" {
		encoded, err := json.Marshal(config)
		if err != nil {
			return cliadapter.Invocation{}, fmt.Errorf("encode OpenClaw runtime config: %w", err)
		}
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
		"--model", request.Model,
		"--timeout", "0",
		"--json",
	}}, nil
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
