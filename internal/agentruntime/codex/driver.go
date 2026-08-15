package codex

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/agentruntime/cliadapter"
	"agent-platform/internal/agentruntime/processharness"
)

const Version = "0.147.0"

type Driver struct{}

func New(config cliadapter.Config) *cliadapter.Adapter {
	if len(config.Command) == 0 {
		config.Command = []string{"codex"}
	}
	if config.ExpectedVersion == "" {
		config.ExpectedVersion = Version
	}
	return cliadapter.New(Driver{}, config)
}

func (Driver) Name() string { return "codex" }

func (Driver) VersionArgs() []string { return []string{"--version"} }

func (Driver) ParseVersion(output string) (string, error) {
	return cliadapter.ParseVersionToken(output, "codex-cli")
}

func (Driver) Build(request agentruntime.ExecuteRequest, _ string) (cliadapter.Invocation, error) {
	args := []string{"exec"}
	if request.CheckpointRef != "" {
		args = append(args, "resume")
	}
	args = append(args,
		"--json",
		"--model", request.Model,
		"--dangerously-bypass-approvals-and-sandbox",
		"--ignore-rules",
	)
	if request.CheckpointRef != "" {
		args = append(args, request.CheckpointRef)
	}
	args = append(args, "-")
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
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
		Item     struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Text     string `json:"text"`
			Command  string `json:"command"`
			ExitCode *int   `json:"exit_code"`
			Changes  any    `json:"changes"`
		} `json:"item"`
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, fmt.Errorf("decode Codex JSONL: %w", err)
	}
	switch envelope.Type {
	case "thread.started":
		p.result.CheckpointRef = envelope.ThreadID
	case "item.started":
		if envelope.Item.Type == "command_execution" {
			return []cliadapter.ParsedEvent{{
				Kind:    agentruntime.EventCommandRequested,
				Payload: map[string]any{"item_id": envelope.Item.ID, "command": envelope.Item.Command},
			}}, nil
		}
	case "item.completed":
		switch envelope.Item.Type {
		case "command_execution":
			return []cliadapter.ParsedEvent{{
				Kind:    agentruntime.EventCommandCompleted,
				Payload: map[string]any{"item_id": envelope.Item.ID, "command": envelope.Item.Command, "exit_code": envelope.Item.ExitCode},
			}}, nil
		case "agent_message":
			p.result.FinalMessage = envelope.Item.Text
		case "file_change":
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventFileChanged, Payload: map[string]any{"changes": envelope.Item.Changes}}}, nil
		}
	case "turn.completed":
		p.result.Usage.InputTokens = envelope.Usage.InputTokens
		p.result.Usage.OutputTokens = envelope.Usage.OutputTokens
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult { return p.result }
