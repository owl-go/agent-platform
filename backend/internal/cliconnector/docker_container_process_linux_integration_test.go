//go:build linux

package cliconnector

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/sandbox"
)

func TestDockerConnectorWaitsForPolicyBeforeCommand(t *testing.T) {
	image, network := os.Getenv("CLI_BROKER_TEST_RUNTIME_IMAGE"), os.Getenv("CLI_CONNECTOR_TEST_NETWORK")
	if image == "" || network == "" {
		t.Skip("requires pinned Runtime image, dedicated Docker network and runsc")
	}
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "rejected"}[reject], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			root := t.TempDir()
			workspace, bundle := filepath.Join(root, "workspace"), filepath.Join(root, "bundles", "connector-1", "node_modules", ".bin")
			for _, dir := range []string{workspace, bundle} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chown(dir, 65532, 65532); err != nil {
					t.Fatal(err)
				}
			}
			fixture := "#!/bin/sh\n[ \"$TOKEN\" = diagnostic-canary ] || exit 9\nprintf ran > /workspace/command-ran\nprintf ok\n"
			if err := os.WriteFile(filepath.Join(bundle, "fixture"), []byte(fixture), 0o755); err != nil {
				t.Fatal(err)
			}
			gate := integrationPolicyGate{t: t, network: network, marker: filepath.Join(workspace, "command-ran"), reject: reject}
			process, err := NewDockerContainerProcess(DockerContainerProcessConfig{
				Image: image, Runtime: "runsc", RunID: "cli-process-integration", BundleDirectory: filepath.Join(root, "bundles"), WorkspaceDirectory: workspace,
				ResolverConfigFile: "/etc/resolv.conf", EgressNetwork: network, UID: 65532, GID: 65532,
				Limits: sandbox.Limits{CPUs: 1, MemoryBytes: 512 << 20, PIDs: 128, TempBytes: 64 << 20}, Egress: &gate,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := process.Run(ctx, ProcessRequest{ConnectorID: "connector-1", Executable: "fixture", Arguments: []string{"status"}, Environment: map[string]string{"TOKEN": "diagnostic-canary"}, EgressHosts: []string{"example.com"}})
			if !gate.called {
				t.Fatalf("policy gate was not reached: %v", err)
			}
			if reject {
				if err == nil {
					t.Fatal("expected policy rejection")
				}
				if _, err := os.Stat(gate.marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("rejected command ran")
				}
			} else if err != nil || result.ExitCode != 0 || string(result.Stdout) != "ok" {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			if exec.CommandContext(ctx, "docker", "inspect", gate.container).Run() == nil {
				t.Fatal("Connector container was not removed")
			}
		})
	}
}

type integrationPolicyGate struct {
	t                          *testing.T
	network, marker, container string
	reject, called             bool
}

func (gate *integrationPolicyGate) Execute(ctx context.Context, container string, _ []string, run func(context.Context) (Result, error)) (Result, error) {
	gate.called, gate.container = true, container
	output, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{(index .NetworkSettings.Networks \""+gate.network+"\").IPAddress}}", container).Output()
	if _, parseErr := netip.ParseAddr(strings.TrimSpace(string(output))); err != nil || parseErr != nil {
		gate.t.Fatalf("network address unavailable: %q %v", output, err)
	}
	output, err = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .Config.Env}}", container).Output()
	if err != nil || strings.Contains(string(output), "diagnostic-canary") || strings.Contains(string(output), "TOKEN=") {
		gate.t.Fatal("credentials reached bootstrap container")
	}
	if _, err := os.Stat(gate.marker); !errors.Is(err, os.ErrNotExist) {
		gate.t.Fatal("command ran before policy installation")
	}
	if gate.reject {
		return Result{}, errors.New("test policy rejection")
	}
	return run(ctx)
}
