package pi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
)

const Version = "0.84.4"

const providerName = "agent-workspace"

type Driver struct{}

func New(config cliadapter.Config) *cliadapter.Adapter {
	if len(config.Command) == 0 {
		config.Command = []string{"pi"}
	}
	if config.ExpectedVersion == "" {
		config.ExpectedVersion = Version
	}
	return cliadapter.New(Driver{}, config)
}

func (Driver) Name() string { return "pi" }

func (Driver) VersionArgs() []string { return []string{"--version"} }

func (Driver) ParseVersion(output string) (string, error) {
	for _, field := range strings.Fields(output) {
		candidate := strings.TrimPrefix(strings.TrimSpace(field), "v")
		if candidate == Version {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unexpected PI Agent version output %q", output)
}

func (Driver) Build(request agentruntime.ExecuteRequest, scratchDirectory string) (cliadapter.Invocation, error) {
	if request.CheckpointRef != "" {
		return cliadapter.Invocation{}, fmt.Errorf("PI Agent native resume is not verified")
	}
	if request.MCPConfigPath != "" {
		return cliadapter.Invocation{}, fmt.Errorf("PI Agent MCP support is not verified")
	}
	endpoint, err := modelEndpoint(request.ModelEndpoint)
	if err != nil {
		return cliadapter.Invocation{}, err
	}
	protocol, err := modelProtocol(request.ModelProtocols)
	if err != nil {
		return cliadapter.Invocation{}, err
	}

	config := struct {
		Providers map[string]providerConfiguration `json:"providers"`
	}{Providers: map[string]providerConfiguration{
		providerName: {
			BaseURL: endpoint,
			API:     protocol,
			APIKey:  "$OPENAI_API_KEY",
			Models: []modelConfiguration{{
				ID: request.Model, Name: request.Model, Input: []string{"text", "image"},
			}},
		},
	}}
	encoded, err := json.Marshal(config)
	if err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("encode PI Agent model configuration: %w", err)
	}
	configPath := filepath.Join(scratchDirectory, "models.json")
	if err := os.WriteFile(configPath, append(encoded, '\n'), 0o600); err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("write PI Agent model configuration: %w", err)
	}
	sessionDirectory := filepath.Join(scratchDirectory, "sessions")
	if err := os.MkdirAll(sessionDirectory, 0o700); err != nil {
		return cliadapter.Invocation{}, fmt.Errorf("create PI Agent session directory: %w", err)
	}

	return cliadapter.Invocation{
		Env: []string{"PI_CODING_AGENT_DIR=" + scratchDirectory},
		Args: []string{
			"--mode", "json",
			"--print",
			"--provider", providerName,
			"--model", request.Model,
			"--session-dir", sessionDirectory,
			"--no-extensions",
			"--no-skills",
			"--no-prompt-templates",
			"--no-context-files",
			"--",
			request.Instruction,
		},
	}, nil
}

type providerConfiguration struct {
	BaseURL string               `json:"baseUrl"`
	API     string               `json:"api"`
	APIKey  string               `json:"apiKey"`
	Models  []modelConfiguration `json:"models"`
}

type modelConfiguration struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Input []string `json:"input"`
}

func modelEndpoint(value string) (string, error) {
	if value == "" {
		value = "https://api.openai.com/v1"
	}
	endpoint, err := url.Parse(value)
	scheme := ""
	if endpoint != nil {
		scheme = strings.ToLower(endpoint.Scheme)
	}
	if err != nil || endpoint == nil || (scheme != "http" && scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", fmt.Errorf("PI Agent model endpoint must be an HTTP or HTTPS URL without credentials, query, or fragment")
	}
	return endpoint.String(), nil
}

func modelProtocol(protocols []string) (string, error) {
	if len(protocols) == 0 {
		return "openai-responses", nil
	}
	for _, candidate := range []struct {
		platform string
		pi       string
	}{
		{platform: "openai_responses", pi: "openai-responses"},
		{platform: "openai_chat", pi: "openai-completions"},
		{platform: "anthropic_messages", pi: "anthropic-messages"},
		{platform: "gemini", pi: "google-generative-ai"},
	} {
		for _, available := range protocols {
			if available == candidate.platform {
				return candidate.pi, nil
			}
		}
	}
	return "", fmt.Errorf("PI Agent requires OpenAI Responses, OpenAI Chat, Anthropic Messages, or Gemini protocol")
}

func (Driver) NewParser(string) cliadapter.Parser { return &parser{} }

type parser struct {
	streamedText strings.Builder
	stderr       []string
	result       cliadapter.ParsedResult
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStderr {
		if value := strings.TrimSpace(string(line)); value != "" {
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
	if len(strings.TrimSpace(string(line))) == 0 {
		return nil, nil
	}
	var envelope struct {
		Type                  string `json:"type"`
		ToolCallID            string `json:"toolCallId"`
		ToolName              string `json:"toolName"`
		Args                  any    `json:"args"`
		Result                any    `json:"result"`
		IsError               bool   `json:"isError"`
		AssistantMessageEvent struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			ID       string `json:"id"`
			ToolName string `json:"toolName"`
		} `json:"assistantMessageEvent"`
		Message struct {
			Role         string `json:"role"`
			StopReason   string `json:"stopReason"`
			ErrorMessage string `json:"errorMessage"`
			Content      []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Usage struct {
				Input  int64 `json:"input"`
				Output int64 `json:"output"`
				Cost   struct {
					Total float64 `json:"total"`
				} `json:"cost"`
			} `json:"usage"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, fmt.Errorf("decode PI Agent JSONL: %w", err)
	}
	switch envelope.Type {
	case "message_update":
		if envelope.AssistantMessageEvent.Type == "text_delta" && envelope.AssistantMessageEvent.Delta != "" {
			p.streamedText.WriteString(envelope.AssistantMessageEvent.Delta)
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": envelope.AssistantMessageEvent.Delta}}}, nil
		}
	case "tool_execution_start":
		return []cliadapter.ParsedEvent{{Kind: agentruntime.EventCommandRequested, Payload: map[string]any{"item_id": envelope.ToolCallID, "tool": envelope.ToolName, "input": envelope.Args}}}, nil
	case "tool_execution_end":
		return []cliadapter.ParsedEvent{{Kind: agentruntime.EventCommandCompleted, Payload: map[string]any{"item_id": envelope.ToolCallID, "tool": envelope.ToolName, "result": envelope.Result, "is_error": envelope.IsError}}}, nil
	case "message_end":
		if envelope.Message.Role != "assistant" {
			return nil, nil
		}
		var textParts []string
		for _, content := range envelope.Message.Content {
			if content.Type == "text" && content.Text != "" {
				textParts = append(textParts, content.Text)
			}
		}
		message := strings.Join(textParts, "")
		if message != "" {
			p.result.FinalMessage = message
		}
		p.result.Usage = agentruntime.Usage{
			InputTokens: envelope.Message.Usage.Input, OutputTokens: envelope.Message.Usage.Output,
			CostMicros: int64(math.Round(envelope.Message.Usage.Cost.Total * 1_000_000)),
		}
		if envelope.Message.StopReason == "error" || envelope.Message.StopReason == "aborted" {
			cause := fmt.Errorf("PI Agent stopped with %s: %s", envelope.Message.StopReason, envelope.Message.ErrorMessage)
			if authenticationFailure(envelope.Message.ErrorMessage) {
				p.result.Error = &agentruntime.Error{Code: agentruntime.ErrorAuthenticationFailed, Message: "PI Agent model authentication failed", Cause: cause}
			} else {
				p.result.Error = cause
			}
		}
		if message != "" && p.streamedText.Len() == 0 {
			return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": message}}}, nil
		}
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult {
	if p.result.Error == nil {
		diagnostic := strings.Join(p.stderr, "\n")
		if authenticationFailure(diagnostic) {
			p.result.Error = &agentruntime.Error{
				Code: agentruntime.ErrorAuthenticationFailed, Message: "PI Agent model authentication failed",
				Cause: fmt.Errorf("PI Agent authentication diagnostic: %s", diagnostic),
			}
		}
	}
	return p.result
}

func authenticationFailure(message string) bool {
	value := strings.ToLower(message)
	return strings.Contains(value, "401") || strings.Contains(value, "unauthorized") || strings.Contains(value, "authentication") || strings.Contains(value, "api key")
}
