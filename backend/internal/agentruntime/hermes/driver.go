package hermes

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
)

const Version = "0.19.0"

type Driver struct{}

func New(config cliadapter.Config) *cliadapter.Adapter {
	if len(config.Command) == 0 {
		config.Command = []string{"hermes"}
	}
	if config.ExpectedVersion == "" {
		config.ExpectedVersion = Version
	}
	return cliadapter.New(Driver{}, config)
}

func (Driver) Name() string { return "hermes" }

func (Driver) VersionArgs() []string { return []string{"--version"} }

func (Driver) ParseVersion(output string) (string, error) {
	for _, field := range strings.Fields(output) {
		if strings.HasPrefix(field, "v") && len(field) > 1 {
			return strings.TrimPrefix(field, "v"), nil
		}
	}
	return "", fmt.Errorf("unexpected Hermes version output %q", output)
}

func (Driver) Build(request agentruntime.ExecuteRequest, scratchDirectory string) (cliadapter.Invocation, error) {
	if request.CheckpointRef != "" {
		return cliadapter.Invocation{}, fmt.Errorf("Hermes native resume is not verified")
	}
	return cliadapter.Invocation{Args: []string{
		"--oneshot", request.Instruction,
		"--model", request.Model,
		"--toolsets", "all",
		"--ignore-rules",
		"--usage-file", filepath.Join(scratchDirectory, "usage.json"),
	}}, nil
}

func (Driver) NewParser(scratchDirectory string) cliadapter.Parser {
	return &parser{usagePath: filepath.Join(scratchDirectory, "usage.json")}
}

type parser struct {
	lines     []string
	usagePath string
	loaded    bool
	result    cliadapter.ParsedResult
}

func (p *parser) Parse(stream processharness.Stream, line []byte) ([]cliadapter.ParsedEvent, error) {
	if stream == processharness.StreamStdout {
		value := string(line)
		p.lines = append(p.lines, value)
		return []cliadapter.ParsedEvent{{Kind: agentruntime.EventMessageDelta, Payload: map[string]string{"delta": value + "\n"}}}, nil
	}
	return nil, nil
}

func (p *parser) Result() cliadapter.ParsedResult {
	if p.loaded {
		return p.result
	}
	p.loaded = true
	p.result.FinalMessage = strings.TrimSpace(strings.Join(p.lines, "\n"))
	contents, err := os.ReadFile(p.usagePath)
	if err != nil {
		p.result.Error = fmt.Errorf("read Hermes usage report: %w", err)
		return p.result
	}
	var usage struct {
		InputTokens      int64   `json:"input_tokens"`
		OutputTokens     int64   `json:"output_tokens"`
		EstimatedCostUSD float64 `json:"estimated_cost_usd"`
		SessionID        string  `json:"session_id"`
		Completed        bool    `json:"completed"`
		Failed           bool    `json:"failed"`
	}
	if err := json.Unmarshal(contents, &usage); err != nil {
		p.result.Error = fmt.Errorf("decode Hermes usage report: %w", err)
		return p.result
	}
	p.result.CheckpointRef = usage.SessionID
	p.result.Usage = agentruntime.Usage{
		InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens,
		CostMicros: int64(math.Round(usage.EstimatedCostUSD * 1_000_000)),
	}
	if usage.Failed || !usage.Completed {
		cause := fmt.Errorf("Hermes usage report marked execution as failed or incomplete")
		diagnostic := strings.ToLower(p.result.FinalMessage)
		if strings.Contains(diagnostic, "401") || strings.Contains(diagnostic, "authentication") {
			p.result.Error = &agentruntime.Error{
				Code: agentruntime.ErrorAuthenticationFailed, Message: "Hermes model authentication failed", Cause: cause,
			}
		} else {
			p.result.Error = cause
		}
	}
	return p.result
}
