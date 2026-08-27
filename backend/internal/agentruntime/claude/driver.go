package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
)

const Version = "2.1.233"

type Driver struct{}

func New(config cliadapter.Config) *cliadapter.Adapter {
	if len(config.Command) == 0 {
		config.Command = []string{"claude"}
	}
	if config.ExpectedVersion == "" {
		config.ExpectedVersion = Version
	}
	return cliadapter.New(Driver{}, config)
}

func (Driver) Name() string { return "claude" }

func (Driver) VersionArgs() []string { return []string{"--version"} }

func (Driver) ParseVersion(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", fmt.Errorf("empty Claude Code version output")
	}
	return fields[0], nil
}

func (Driver) Build(request agentruntime.ExecuteRequest, _ string) (cliadapter.Invocation, error) {
	if len(request.ModelProtocols) > 0 && !request.SupportsModelProtocol("anthropic_messages") {
		return cliadapter.Invocation{}, fmt.Errorf("Claude Code requires the Anthropic Messages protocol")
	}
	args := []string{
		"--print",
		"--bare",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--permission-mode", "bypassPermissions",
		"--dangerously-skip-permissions",
		"--strict-mcp-config",
		"--disable-slash-commands",
		"--no-chrome",
		"--model", request.Model,
	}
	if request.CheckpointRef != "" {
		args = append(args, "--resume", request.CheckpointRef)
	}
	if request.MCPConfigPath != "" {
		args = append(args, "--mcp-config", request.MCPConfigPath)
	}
	return cliadapter.Invocation{Args: args, Stdin: strings.NewReader(request.Instruction)}, nil
}

func (Driver) NewParser(string) cliadapter.Parser { return &parser{} }

type parser struct {
	result cliadapter.ParsedResult
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStderr || len(strings.TrimSpace(string(line))) == 0 {
		return nil, nil
	}
	var envelope struct {
		Type      string `json:"type"`
		Subtype   string `json:"subtype"`
		Result    string `json:"result"`
		SessionID string `json:"session_id"`
		Event     struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		} `json:"event"`
		Message struct {
			Content []struct {
				Type  string         `json:"type"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		} `json:"message"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
		TotalCostUSD float64 `json:"total_cost_usd"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, fmt.Errorf("decode Claude Code JSONL: %w", err)
	}
	switch envelope.Type {
	case "stream_event":
		if envelope.Event.Delta.Type == "text_delta" && envelope.Event.Delta.Text != "" {
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": envelope.Event.Delta.Text}}}, nil
		}
	case "assistant":
		events := make([]cliadapter.ParsedEvent, 0)
		for _, content := range envelope.Message.Content {
			if content.Type == "tool_use" {
				events = append(events, cliadapter.ParsedEvent{
					Kind:    agentruntime.EventCommandRequested,
					Payload: map[string]any{"tool": content.Name, "input": content.Input},
				})
			}
		}
		return events, nil
	case "result":
		p.result.FinalMessage = envelope.Result
		p.result.CheckpointRef = envelope.SessionID
		p.result.Usage = agentruntime.Usage{
			InputTokens: envelope.Usage.InputTokens, OutputTokens: envelope.Usage.OutputTokens,
			CostMicros: int64(envelope.TotalCostUSD * 1_000_000),
		}
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult { return p.result }
