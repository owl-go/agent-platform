//go:build linux

package cliconnector

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/agentruntime/containerprocess"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/sandbox"
)

func TestUnixBrokerRuntimeClientLongPath(t *testing.T) {
	image := os.Getenv("CLI_BROKER_TEST_RUNTIME_IMAGE")
	if image == "" {
		t.Skip("CLI_BROKER_TEST_RUNTIME_IMAGE requires a local pinned Runtime image, Docker and runsc")
	}
	for _, warm := range []bool{false, true} {
		name := "cold"
		if warm {
			name = "warm"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			root := t.TempDir()
			scratch := filepath.Join(root, strings.Repeat("workspace-", 12), ".runtime-containers", strings.Repeat("a", 32), "scratch")
			workspace, credentials := filepath.Join(root, "workspace"), filepath.Join(root, "credentials")
			for _, directory := range []string{scratch, workspace, credentials} {
				if err := os.MkdirAll(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(directory, 65532, 65532); err != nil {
					t.Fatal(err)
				}
			}
			definition := brokerDefinition(RiskLow)
			broker, err := NewBroker(BrokerConfig{Definitions: []Definition{definition}, RuntimeDigest: definition.RuntimeDigests[0], Wrapper: Wrapper{Process: &recordingProcess{}}})
			if err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(scratch, "broker", "cli-broker.sock")
			server, err := StartUnixBroker(ctx, broker, socket, 65532, 65532)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			config := containerprocess.Config{
				DockerCommand: "docker", Image: image, RuntimeCommand: "node", RunID: "broker-long-path-test",
				Runtime: "runsc", UID: 65532, GID: 65532, Egress: sandbox.EgressNone,
				WorkspaceDirectory: workspace, ContainerWorkspace: "/workspace", CredentialDirectory: credentials,
				ScratchDirectory: scratch, CLIBrokerSocket: socket, DirectCommand: true,
				Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 512 << 20, PIDs: 128, TempBytes: 64 << 20},
			}
			spec := processharness.Spec{Command: []string{"node", "/usr/local/bin/agent-cli", "--connector", "connector-1", "--capability", "identity", "--identity", "user", "--", "auth", "status"}, Dir: workspace}
			if !warm {
				run, err := containerprocess.New(config)
				if err != nil {
					t.Fatal(err)
				}
				sink := &brokerClientOutput{}
				result, err := run(ctx, spec, sink)
				if err != nil || result.ExitCode != 0 || sink.stdout != "ok" {
					t.Fatalf("client result=%#v err=%v stdout=%q stderr=%q", result, err, sink.stdout, sink.stderr)
				}
				return
			}
			manager, err := containerprocess.NewWarmManager("docker", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			containerName, err := containerprocess.WarmContainerName(root, "node", image)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cleanupCancel()
				_ = exec.CommandContext(cleanupCtx, "docker", "rm", "--force", containerName).Run()
			})
			for range 2 {
				lease, err := manager.Checkout(ctx, containerName)
				if err != nil {
					t.Fatal(err)
				}
				run, startErr := lease.Start(ctx, config)
				if startErr != nil {
					_ = lease.Release(ctx)
					t.Fatal(startErr)
				}
				sink := &brokerClientOutput{}
				result, runErr := run(ctx, spec, sink)
				releaseErr := lease.Release(ctx)
				if runErr != nil || releaseErr != nil || result.ExitCode != 0 || sink.stdout != "ok" {
					t.Fatalf("client result=%#v run=%v release=%v stdout=%q stderr=%q", result, runErr, releaseErr, sink.stdout, sink.stderr)
				}
			}
		})
	}
}

type brokerClientOutput struct{ stdout, stderr string }

func (output *brokerClientOutput) Store(_ context.Context, item processharness.Output) error {
	data, err := io.ReadAll(item.Reader)
	if item.Stream == processharness.StreamStdout {
		output.stdout += string(data)
	} else {
		output.stderr += string(data)
	}
	return err
}
