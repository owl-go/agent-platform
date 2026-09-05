package cliconnector

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingEgressGate struct {
	container string
	hosts     []string
	runs      int
	err       error
}

func (gate *recordingEgressGate) Execute(ctx context.Context, container string, hosts []string, run func(context.Context) (Result, error)) (Result, error) {
	gate.container = container
	gate.hosts = append([]string(nil), hosts...)
	if gate.err != nil {
		return Result{}, gate.err
	}
	gate.runs++
	return run(ctx)
}

func TestDockerExecProcessRunsVerifiedBundleBehindEgressGate(t *testing.T) {
	gate := &recordingEgressGate{}
	var command []string
	var environment map[string]string
	process, err := NewDockerExecProcess(DockerExecProcessConfig{
		ContainerName: "agent-runtime-warm-test", ContainerWorkspace: "/workspace", Egress: gate,
		Run: func(_ context.Context, arguments []string, variables map[string]string) (Result, error) {
			command = append([]string(nil), arguments...)
			environment = cloneEnvironment(variables)
			return Result{Stdout: []byte("ok")}, nil
		},
	})
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
	wantCommand := []string{"docker", "exec", "--interactive", "--workdir", "/workspace", "--env", "TOKEN", "agent-runtime-warm-test", "/opt/agent-platform/connectors/connector-1/node_modules/.bin/tool", "identity", "show"}
	if !reflect.DeepEqual(command, wantCommand) {
		t.Fatalf("command = %#v, want %#v", command, wantCommand)
	}
	if environment["TOKEN"] != "secret" || gate.container != "agent-runtime-warm-test" || !reflect.DeepEqual(gate.hosts, []string{"open.feishu.cn"}) {
		t.Fatalf("environment=%#v container=%q hosts=%#v", environment, gate.container, gate.hosts)
	}
}

func TestDockerExecProcessDoesNotStartWhenEgressGateRejects(t *testing.T) {
	gate := &recordingEgressGate{err: errors.New("egress unavailable")}
	started := false
	process, err := NewDockerExecProcess(DockerExecProcessConfig{
		ContainerName: "runtime-1", ContainerWorkspace: "/workspace", Egress: gate,
		Run: func(context.Context, []string, map[string]string) (Result, error) {
			started = true
			return Result{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = process.Run(context.Background(), ProcessRequest{ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"read"}, EgressHosts: []string{"example.com"}})
	if err == nil || started || gate.runs != 0 {
		t.Fatalf("err=%v started=%v gate runs=%d", err, started, gate.runs)
	}
}

func TestDockerExecProcessRejectsRuntimeSocketEnvironmentOverride(t *testing.T) {
	gate := &recordingEgressGate{}
	process, err := NewDockerExecProcess(DockerExecProcessConfig{ContainerName: "runtime-1", ContainerWorkspace: "/workspace", Egress: gate, Run: func(context.Context, []string, map[string]string) (Result, error) {
		return Result{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = process.Run(context.Background(), ProcessRequest{ConnectorID: "connector-1", Executable: "tool", Arguments: []string{"read"}, Environment: map[string]string{"AGENT_PLATFORM_CLI_SOCKET": "/forged"}, EgressHosts: []string{"example.com"}})
	if err == nil || gate.runs != 0 {
		t.Fatalf("err=%v gate runs=%d", err, gate.runs)
	}
}
