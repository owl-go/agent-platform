package codex

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
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
	if len(request.ModelProtocols) > 0 && !request.SupportsModelProtocol("openai_responses") {
		return cliadapter.Invocation{}, fmt.Errorf("Codex requires the OpenAI Responses protocol")
	}
	endpointValue := request.ModelEndpoint
	if endpointValue == "" {
		endpointValue = "https://api.openai.com/v1"
	}
	endpoint, err := url.Parse(endpointValue)
	scheme := ""
	if endpoint != nil {
		scheme = strings.ToLower(endpoint.Scheme)
	}
	if err != nil || endpoint == nil || (scheme != "http" && scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return cliadapter.Invocation{}, fmt.Errorf("Codex model endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
	}
	args := []string{
		"--strict-config",
		"-c", `model_provider="agent_workspace"`,
		"-c", `model_providers.agent_workspace.name="Agent Workspace"`,
		"-c", "model_providers.agent_workspace.base_url=" + strconv.Quote(endpoint.String()),
		"-c", `model_providers.agent_workspace.env_key="OPENAI_API_KEY"`,
		"-c", `model_providers.agent_workspace.wire_api="responses"`,
		"exec",
	}
	if request.CheckpointRef != "" {
		args = append(args, "resume")
	}
	for _, attachment := range request.Attachments {
		if strings.HasPrefix(strings.ToLower(attachment.ContentType), "image/") {
			args = append(args, "--image", attachment.Path)
		}
	}
	// A following option terminates Codex's variadic --image values so stdin's
	// prompt marker is not misparsed as another image path on a new Session.
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
			InputTokens  *int64 `json:"input_tokens"`
			OutputTokens *int64 `json:"output_tokens"`
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
		case "reasoning":
			if envelope.Item.Text != "" {
				return []cliadapter.ParsedEvent{{Kind: agentruntime.EventReasoningSummary, Payload: map[string]string{"summary": envelope.Item.Text}}}, nil
			}
		case "command_execution":
			return []cliadapter.ParsedEvent{{
				Kind:    agentruntime.EventCommandCompleted,
				Payload: map[string]any{"item_id": envelope.Item.ID, "command": envelope.Item.Command, "exit_code": envelope.Item.ExitCode},
			}}, nil
		case "agent_message":
			p.result.FinalMessage = envelope.Item.Text
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": envelope.Item.Text}}}, nil
		case "file_change":
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventFileChanged, Payload: map[string]any{"changes": envelope.Item.Changes}}}, nil
		}
	case "turn.completed":
		if envelope.Usage.InputTokens != nil && envelope.Usage.OutputTokens != nil {
			p.result.Usage.InputTokens = *envelope.Usage.InputTokens
			p.result.Usage.OutputTokens = *envelope.Usage.OutputTokens
			p.result.Usage.Reported = true
		}
		if p.result.FinalMessage != "" {
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageCompleted, Payload: map[string]string{"message": p.result.FinalMessage}}}, nil
		}
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult { return p.result }
