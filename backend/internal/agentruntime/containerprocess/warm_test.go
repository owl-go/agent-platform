package containerprocess

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestWarmManagerReusesContainerDefinitionAndExecutesBothInvocations(t *testing.T) {
	manager, err := NewWarmManager("docker", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	name, err := WarmContainerName("session:owner:session", "claude", digestImage())
	if err != nil {
		t.Fatal(err)
	}
	exists, running, fingerprint := false, false, ""
	creates := 0
	manager.docker = func(_ context.Context, arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "inspect":
			if !exists {
				return []byte("No such container"), errors.New("missing")
			}
			if strings.Contains(arguments[2], "warm-config") {
				return []byte(fingerprint + "\n"), nil
			}
			if running {
				return []byte("true\n"), nil
			}
			return []byte("false\n"), nil
		case "create":
			exists, creates = true, creates+1
			for index, argument := range arguments {
				if argument == "--label" && index+1 < len(arguments) && strings.HasPrefix(arguments[index+1], "agent-platform.warm-config=") {
					fingerprint = strings.TrimPrefix(arguments[index+1], "agent-platform.warm-config=")
				}
			}
			return []byte(name), nil
		case "start":
			running = true
			return []byte(name), nil
		case "stop":
			running = false
			return []byte(name), nil
		default:
			t.Fatalf("unexpected Docker call: %#v", arguments)
			return nil, nil
		}
	}
	var commands [][]string
	manager.runHost = func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		commands = append(commands, append([]string(nil), spec.Command...))
		return processharness.Result{}, nil
	}
	config := validConfig(nil, nil)
	config.ContainerWorkspace = "/workspace"
	config.ScratchDirectory = "/workspaces/scratch"

	for _, command := range [][]string{{"claude", "--version"}, {"claude", "execute"}} {
		lease, err := manager.Checkout(context.Background(), name)
		if err != nil {
			t.Fatal(err)
		}
		run, err := lease.Start(context.Background(), config)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := run(context.Background(), processharness.Spec{Command: command, Dir: config.WorkspaceDirectory}, discardSink{}); err != nil {
			t.Fatal(err)
		}
		if err := lease.Release(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if creates != 1 {
		t.Fatalf("container create count = %d, want one retained definition", creates)
	}
	wantPrefix := []string{"docker", "exec", "--interactive", "--workdir", "/workspace", name, "/usr/local/bin/runtime-entrypoint", "claude"}
	for _, command := range commands {
		if len(command) < len(wantPrefix) || !reflect.DeepEqual(command[:len(wantPrefix)], wantPrefix) {
			t.Fatalf("warm exec command = %#v", command)
		}
	}
}

func TestWarmManagerReapsOnlyContainersIdleForThirtyMinutes(t *testing.T) {
	manager, err := NewWarmManager("docker", 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	oldName := warmContainerPrefix + strings.Repeat("a", 32)
	freshName := warmContainerPrefix + strings.Repeat("b", 32)
	var removed []string
	manager.docker = func(_ context.Context, arguments ...string) ([]byte, error) {
		switch arguments[0] {
		case "ps":
			return []byte(oldName + "\n" + freshName + "\n"), nil
		case "inspect":
			finished := now.Add(-31 * time.Minute)
			if arguments[len(arguments)-1] == freshName {
				finished = now.Add(-29 * time.Minute)
			}
			return []byte("false|" + finished.Format(time.RFC3339Nano)), nil
		case "rm":
			removed = append(removed, arguments[len(arguments)-1])
			return nil, nil
		default:
			t.Fatalf("unexpected Docker call: %#v", arguments)
			return nil, nil
		}
	}
	count, err := manager.Reap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !reflect.DeepEqual(removed, []string{oldName}) {
		t.Fatalf("reaped count=%d containers=%v", count, removed)
	}
}
