package openclaw

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/agentruntime/cliadapter"
	"agent-platform/internal/agentruntime/processharness"
)

const Version = "2026.7.1-2"

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
	promptPath := filepath.Join(scratchDirectory, "instruction.txt")
	if err := os.WriteFile(promptPath, []byte(request.Instruction), 0o600); err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("write OpenClaw instruction: %w", err)
	}
	return cliadapter.Invocation{Args: []string{
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
	loaded bool
	result cliadapter.ParsedResult
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStdout {
		p.stdout = append(p.stdout, string(line))
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
	if err := json.Unmarshal([]byte(strings.Join(p.stdout, "\n")), &response); err != nil {
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
