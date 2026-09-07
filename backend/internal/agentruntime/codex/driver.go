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

const (
	maxDiagnosticLines     = 16
	maxDiagnosticLineBytes = 4 * 1024
)

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
	result    cliadapter.ParsedResult
	stderr    []string
	lastError string
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStderr {
		if diagnostic := boundedDiagnostic(string(line)); diagnostic != "" {
			p.stderr = append(p.stderr, diagnostic)
			if len(p.stderr) > maxDiagnosticLines {
				p.stderr = p.stderr[len(p.stderr)-maxDiagnosticLines:]
			}
		}
		return nil, nil
	}
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil, nil
	}
	var envelope struct {
		Type     string `json:"type"`
		ThreadID string `json:"thread_id"`
		Message  string `json:"message"`
		Error    struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
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
	case "error":
		if diagnostic := boundedDiagnostic(envelope.Message); diagnostic != "" {
			p.lastError = diagnostic
		}
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
	case "turn.failed":
		diagnostic := boundedDiagnostic(envelope.Error.Message)
		if diagnostic == "" {
			diagnostic = p.lastError
		}
		if diagnostic == "" {
			diagnostic = "Codex turn failed without a diagnostic"
		}
		p.result.Error = &agentruntime.Error{
			Code: agentruntime.ErrorModelFailed, Message: "Codex execution failed", Cause: fmt.Errorf("%s", diagnostic),
		}
	}
	return nil, nil
}

func boundedDiagnostic(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxDiagnosticLineBytes {
		value = "..." + value[len(value)-(maxDiagnosticLineBytes-3):]
	}
	return strings.ToValidUTF8(value, "?")
}

func (p *parser) Result() cliadapter.ParsedResult {
	if p.result.Error == nil && p.result.FinalMessage == "" {
		switch {
		case p.lastError != "":
			p.result.Error = &agentruntime.Error{
				Code: agentruntime.ErrorModelFailed, Message: "Codex execution failed", Cause: fmt.Errorf("%s", p.lastError),
			}
		case len(p.stderr) > 0:
			p.result.Error = fmt.Errorf("Codex diagnostic: %s", strings.Join(p.stderr, "\n"))
		}
	}
	return p.result
}
