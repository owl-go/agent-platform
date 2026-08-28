package codex

import (
	"slices"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime"
	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestDriverBuildsNewAndResumeInvocations(t *testing.T) {
	driver := Driver{}
	request := agentruntime.ExecuteRequest{Model: "configured-model", ModelEndpoint: "https://models.example.test/openai", Instruction: "fix tests"}
	created, err := driver.Build(request, t.TempDir())
	if err != nil {
		t.Fatalf("build new invocation: %v", err)
	}
	wantPrefix := []string{
		"--strict-config",
		"-c", `model_provider="agent_workspace"`,
		"-c", `model_providers.agent_workspace.name="Agent Workspace"`,
		"-c", `model_providers.agent_workspace.base_url="https://models.example.test/openai"`,
		"-c", `model_providers.agent_workspace.env_key="OPENAI_API_KEY"`,
		"-c", `model_providers.agent_workspace.wire_api="responses"`,
	}
	wantNew := append(slices.Clone(wantPrefix), "exec", "--json", "--model", "configured-model", "--dangerously-bypass-approvals-and-sandbox", "--ignore-rules", "-")
	if !slices.Equal(created.Args, wantNew) || created.Stdin == nil {
		t.Fatalf("new invocation = %+v", created)
	}

	request.CheckpointRef = "thread-1"
	resumed, err := driver.Build(request, t.TempDir())
	if err != nil {
		t.Fatalf("build resume invocation: %v", err)
	}
	wantResume := append(slices.Clone(wantPrefix), "exec", "resume", "--json", "--model", "configured-model", "--dangerously-bypass-approvals-and-sandbox", "--ignore-rules", "thread-1", "-")
	if !slices.Equal(resumed.Args, wantResume) {
		t.Fatalf("resume args = %v, want %v", resumed.Args, wantResume)
	}
}

func TestDriverBuildsInvocationForTrustedHTTPModelEndpoint(t *testing.T) {
	invocation, err := (Driver{}).Build(agentruntime.ExecuteRequest{
		Model:         "gpt-5.6-sol",
		ModelEndpoint: "http://47.237.108.63:3000/openai",
		ModelProtocols: []string{
			"openai_responses",
		},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("build HTTP model invocation: %v", err)
	}
	if !slices.Contains(invocation.Args, `model_providers.agent_workspace.base_url="http://47.237.108.63:3000/openai"`) {
		t.Fatalf("args = %v", invocation.Args)
	}
}

func TestDriverRejectsUnsafeModelEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"https://user:password@models.example.test/openai",
		"http://user:password@models.example.test/openai",
		"https://models.example.test/openai?token=secret",
		"http://models.example.test/openai?token=secret",
		"https://models.example.test/openai#fragment",
		"ftp://models.example.test/openai",
	} {
		t.Run(strings.ReplaceAll(endpoint, "/", "_"), func(t *testing.T) {
			_, err := (Driver{}).Build(agentruntime.ExecuteRequest{ModelEndpoint: endpoint}, t.TempDir())
			if err == nil {
				t.Fatalf("Build(%q) succeeded", endpoint)
			}
		})
	}
}

func TestDriverUsesOfficialEndpointForStandaloneConformance(t *testing.T) {
	invocation, err := (Driver{}).Build(agentruntime.ExecuteRequest{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(invocation.Args, `model_providers.agent_workspace.base_url="https://api.openai.com/v1"`) {
		t.Fatalf("args = %v", invocation.Args)
	}
}

func TestParserReadsThreadItemsAndUsage(t *testing.T) {
	parser := Driver{}.NewParser(t.TempDir())
	fixtures := []string{
		`{"type":"thread.started","thread_id":"thread-1"}`,
		`{"type":"item.started","item":{"id":"item-1","type":"command_execution","command":"go test ./..."}}`,
		`{"type":"item.completed","item":{"id":"item-1","type":"command_execution","command":"go test ./...","exit_code":0}}`,
		`{"type":"item.completed","item":{"id":"item-2","type":"agent_message","text":"done"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":14,"output_tokens":8}}`,
	}
	var kinds []agentruntime.EventKind
	for _, fixture := range fixtures {
		events, err := parser.Parse(processharness.StreamStdout, []byte(fixture))
		if err != nil {
			t.Fatalf("parse fixture: %v", err)
		}
		for _, event := range events {
			kinds = append(kinds, event.Kind)
		}
	}
	wantKinds := []agentruntime.EventKind{agentruntime.EventCommandRequested, agentruntime.EventCommandCompleted, agentruntime.EventMessageDelta}
	if !slices.Equal(kinds, wantKinds) {
		t.Fatalf("event kinds = %v, want %v", kinds, wantKinds)
	}
	result := parser.Result()
	if result.FinalMessage != "done" || result.CheckpointRef != "thread-1" || result.Usage.InputTokens != 14 || result.Usage.OutputTokens != 8 {
		t.Fatalf("parsed result = %+v", result)
	}
}
