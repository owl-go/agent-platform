package cliconnector

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"agent-platform/backend/internal/sandbox"
)

type recordingEgressGate struct {
	container string
	hosts     []string
	runs      int
	err       error
	events    *[]string
}

func (gate *recordingEgressGate) Execute(ctx context.Context, container string, hosts []string, run func(context.Context) (Result, error)) (Result, error) {
	gate.container = container
	gate.hosts = append([]string(nil), hosts...)
	if gate.err != nil {
		return Result{}, gate.err
	}
	gate.runs++
	if gate.events != nil {
		*gate.events = append(*gate.events, "policy-installed")
	}
	result, err := run(ctx)
	if gate.events != nil {
		*gate.events = append(*gate.events, "policy-removed")
	}
	return result, err
}

func TestDockerContainerProcessStartsOnlyAfterNetworkPolicyAndCleansUp(t *testing.T) {
	var events []string
	gate := &recordingEgressGate{events: &events}
	var commands [][]string
	var start []string
	var createEnvironment map[string]string
	process, err := NewDockerContainerProcess(testDockerContainerConfig(gate, func(_ context.Context, environment map[string]string, command string, arguments ...string) ([]byte, error) {
		commands = append(commands, append([]string{command}, arguments...))
		events = append(events, arguments[0])
		if len(commands) == 1 {
			createEnvironment = cloneEnvironment(environment)
		}
		return nil, nil
	}, func(_ context.Context, arguments []string) (Result, error) {
		start = append([]string(nil), arguments...)
		events = append(events, "start")
		return Result{Stdout: []byte("ok")}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	result, err := process.Run(context.Background(), ProcessRequest{
		ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"identity", "show"},
		Environment: map[string]string{"TOKEN": "secret"}, EgressHosts: []string{"open.feishu.cn"},
	})
	if err != nil || string(result.Stdout) != "ok" || gate.runs != 1 {
		t.Fatalf("result=%q gate runs=%d err=%v", result.Stdout, gate.runs, err)
	}
	if len(commands) != 3 || commands[0][1] != "create" || !reflect.DeepEqual(commands[1], []string{"docker", "network", "connect", "agent-public-egress", "agent-cli-test"}) || !reflect.DeepEqual(commands[2], []string{"docker", "rm", "--force", "agent-cli-test"}) {
		t.Fatalf("commands = %#v", commands)
	}
	create := commands[0]
	joined := strings.Join(create, " ")
	for _, required := range []string{"--network none", "--runtime runsc", "--read-only", "--cap-drop ALL", "no-new-privileges", "src=/runtime/connectors/connector-1,dst=/opt/agent-platform/connector,readonly=true", "--entrypoint /usr/local/bin/runtime-entrypoint", "--env TOKEN", "/opt/agent-platform/connector/node_modules/.bin/tool identity show"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("create command missing %q: %#v", required, create)
		}
	}
	if strings.Contains(joined, "secret") {
		t.Fatal("secret value was exposed in Docker arguments")
	}
	if createEnvironment["TOKEN"] != "secret" {
		t.Fatalf("create environment = %#v", createEnvironment)
	}
	if !reflect.DeepEqual(start, []string{"docker", "start", "--attach", "--interactive", "agent-cli-test"}) || gate.container != "agent-cli-test" || !reflect.DeepEqual(gate.hosts, []string{"open.feishu.cn"}) {
		t.Fatalf("start=%#v container=%q hosts=%#v", start, gate.container, gate.hosts)
	}
	wantEvents := []string{"create", "network", "policy-installed", "start", "rm", "policy-removed"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}

func TestDockerContainerProcessCleansUpWhenEgressGateRejects(t *testing.T) {
	gate := &recordingEgressGate{err: errors.New("egress unavailable")}
	var commands [][]string
	started := false
	process, err := NewDockerContainerProcess(testDockerContainerConfig(gate, func(_ context.Context, _ map[string]string, command string, arguments ...string) ([]byte, error) {
		commands = append(commands, append([]string{command}, arguments...))
		return nil, nil
	}, func(context.Context, []string) (Result, error) {
		started = true
		return Result{}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = process.Run(context.Background(), ProcessRequest{ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"read"}, EgressHosts: []string{"example.com"}})
	if err == nil || started || gate.runs != 0 || len(commands) != 3 || commands[2][1] != "rm" {
		t.Fatalf("err=%v started=%v gate runs=%d commands=%#v", err, started, gate.runs, commands)
	}
}

func TestDockerContainerProcessDoesNotCleanupContainerCreateItDidNotOwn(t *testing.T) {
	gate := &recordingEgressGate{}
	var commands [][]string
	process, err := NewDockerContainerProcess(testDockerContainerConfig(gate, func(_ context.Context, _ map[string]string, command string, arguments ...string) ([]byte, error) {
		commands = append(commands, append([]string{command}, arguments...))
		return []byte("create failed"), errors.New("exit 1")
	}, func(context.Context, []string) (Result, error) { return Result{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	_, err = process.Run(context.Background(), ProcessRequest{ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"read"}, EgressHosts: []string{"example.com"}})
	if err == nil || len(commands) != 1 || commands[0][1] != "create" {
		t.Fatalf("err=%v commands=%#v", err, commands)
	}
}

func TestDockerContainerProcessRejectsRuntimeSocketEnvironmentOverride(t *testing.T) {
	gate := &recordingEgressGate{}
	process, err := NewDockerContainerProcess(testDockerContainerConfig(gate, func(context.Context, map[string]string, string, ...string) ([]byte, error) {
		t.Fatal("Docker must not run")
		return nil, nil
	}, func(context.Context, []string) (Result, error) { return Result{}, nil }))
	if err != nil {
		t.Fatal(err)
	}
	_, err = process.Run(context.Background(), ProcessRequest{ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"read"}, Environment: map[string]string{"AGENT_PLATFORM_CLI_SOCKET": "/forged"}, EgressHosts: []string{"example.com"}})
	if err == nil || gate.runs != 0 {
		t.Fatalf("err=%v gate runs=%d", err, gate.runs)
	}
}

func testDockerContainerConfig(gate EgressGate, run DockerCommandRunner, start DockerStartRunner) DockerContainerProcessConfig {
	return DockerContainerProcessConfig{
		Image: "registry.example/runtime@sha256:" + strings.Repeat("a", 64), Runtime: "runsc", RunID: "run-1",
		BundleDirectory: "/runtime/connectors", WorkspaceDirectory: "/runtime/workspace", ContainerWorkspace: "/workspace",
		ResolverConfigFile: "/etc/agent/resolv.conf", EgressNetwork: "agent-public-egress",
		Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 1 << 30, PIDs: 128, TempBytes: 256 << 20}, UID: 65532, GID: 65532,
		Egress: gate, Run: run, Start: start, Name: func() (string, error) { return "agent-cli-test", nil },
	}
}
