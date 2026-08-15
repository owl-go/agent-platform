package agentruntime_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/agentruntime/claude"
	"agent-platform/internal/agentruntime/cliadapter"
	"agent-platform/internal/agentruntime/codex"
	"agent-platform/internal/agentruntime/hermes"
	"agent-platform/internal/agentruntime/openclaw"
	"agent-platform/internal/agentruntime/processharness"
	"agent-platform/internal/runworker"
)

func TestRuntimeAdaptersShareContract(t *testing.T) {
	tests := []struct {
		name    string
		version string
		new     func(cliadapter.Config) agentruntime.Adapter
	}{
		{name: "claude", version: claude.Version, new: func(config cliadapter.Config) agentruntime.Adapter { return claude.New(config) }},
		{name: "codex", version: codex.Version, new: func(config cliadapter.Config) agentruntime.Adapter { return codex.New(config) }},
		{name: "hermes", version: hermes.Version, new: func(config cliadapter.Config) agentruntime.Adapter { return hermes.New(config) }},
		{name: "openclaw", version: openclaw.Version, new: func(config cliadapter.Config) agentruntime.Adapter { return openclaw.New(config) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := test.new(cliadapter.Config{
				Command:    []string{test.name},
				OutputSink: adapterDiscardSink{},
				RunProcess: fakeRuntimeProcess,
			})
			descriptor, err := adapter.Describe(context.Background())
			if err != nil {
				t.Fatalf("describe adapter: %v", err)
			}
			if descriptor.Name != test.name || descriptor.Version != test.version {
				t.Fatalf("descriptor = %+v", descriptor)
			}
			for _, capability := range []agentruntime.Capability{
				agentruntime.CapabilityStreaming,
				agentruntime.CapabilityNativeResume,
				agentruntime.CapabilityStructuredFinal,
				agentruntime.CapabilitySubagents,
				agentruntime.CapabilityUsage,
			} {
				if descriptor.Supports(capability) {
					t.Fatalf("unverified capability %q was enabled", capability)
				}
			}

			events := &adapterEventSink{}
			result, err := runworker.New(adapter).Execute(context.Background(), agentruntime.ExecuteRequest{
				RunID:          "run-1",
				WorkspacePath:  t.TempDir(),
				Instruction:    "fix tests",
				Model:          "configured-model",
				EnvironmentRef: "environment-1",
			}, events)
			if err != nil {
				t.Fatalf("execute adapter: %v", err)
			}
			if result.FinalMessage != "done" || result.ExitCode != 0 {
				t.Fatalf("result = %+v", result)
			}
			if len(events.events) < 3 || events.events[0].Kind != agentruntime.EventRuntimeStarted || events.events[len(events.events)-1].Kind != agentruntime.EventRuntimeCompleted {
				t.Fatalf("event contract = %+v", events.events)
			}
		})
	}
}

func fakeRuntimeProcess(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (processharness.Result, error) {
	runtimeName := spec.Command[0]
	if containsArgument(spec.Command, "--version") {
		versions := map[string]string{
			"claude":   "2.1.233 (Claude Code)\n",
			"codex":    "codex-cli 0.147.0\n",
			"hermes":   "Hermes Agent v0.19.0 (2026-07-20)\n",
			"openclaw": "OpenClaw 2026.7.1-2\n",
		}
		output := versions[runtimeName]
		if err := sink.Store(ctx, processharness.Output{Stream: processharness.StreamStdout, Reader: strings.NewReader(output), Size: int64(len(output)), UTF8: true, Inline: true}); err != nil {
			return processharness.Result{}, err
		}
		return processharness.Result{}, nil
	}

	var output string
	switch runtimeName {
	case "claude":
		output = "{\"type\":\"result\",\"subtype\":\"success\",\"result\":\"done\",\"session_id\":\"session-1\",\"usage\":{}}\n"
	case "codex":
		output = "{\"type\":\"thread.started\",\"thread_id\":\"thread-1\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n"
	case "hermes":
		usagePath := argumentValue(spec.Command, "--usage-file")
		if err := os.WriteFile(usagePath, []byte(`{"session_id":"session-1"}`), 0o600); err != nil {
			return processharness.Result{}, err
		}
		output = "done\n"
	case "openclaw":
		output = "{\"payloads\":[{\"text\":\"done\"}],\"meta\":{\"agentMeta\":{\"sessionId\":\"session-1\"}}}\n"
	default:
		return processharness.Result{}, fmt.Errorf("unknown fake runtime %q", runtimeName)
	}
	if err := spec.Observer.Observe(ctx, processharness.StreamStdout, []byte(output)); err != nil {
		return processharness.Result{}, err
	}
	return processharness.Result{ExitCode: 0}, nil
}

type adapterDiscardSink struct{}

func (adapterDiscardSink) Store(_ context.Context, output processharness.Output) error {
	_, err := io.Copy(io.Discard, output.Reader)
	return err
}

type adapterEventSink struct {
	events []agentruntime.Event
}

func (s *adapterEventSink) Publish(_ context.Context, event agentruntime.Event) error {
	s.events = append(s.events, event)
	return nil
}

func containsArgument(arguments []string, expected string) bool {
	for _, argument := range arguments {
		if argument == expected {
			return true
		}
	}
	return false
}

func argumentValue(arguments []string, name string) string {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name {
			return arguments[index+1]
		}
	}
	return ""
}
