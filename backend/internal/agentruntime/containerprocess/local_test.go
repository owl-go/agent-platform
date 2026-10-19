package containerprocess

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"agent-platform/backend/internal/agentruntime/processharness"
)

func TestLocalRuncPreservesIsolationAndProductionRejectsIt(t *testing.T) {
	var command []string
	config := validConfig(func(_ context.Context, spec processharness.Spec, _ processharness.OutputSink) (processharness.Result, error) {
		command = spec.Command
		return processharness.Result{}, nil
	}, func(context.Context, string) error { return nil })
	config.Runtime = "runc"
	config.Image = "sha256:" + strings.Repeat("a", 64)
	if _, err := New(config); err == nil {
		t.Fatal("implicit runc downgrade accepted")
	}
	config.Profile = "local"
	run, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run(t.Context(), processharness.Spec{Command: []string{"claude", "--version"}, Dir: config.WorkspaceDirectory}, discardSink{}); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"--runtime runc", "--user 65532:65532", "--read-only", "--cap-drop ALL", "--security-opt no-new-privileges", "--pids-limit", "--memory", "readonly=true"} {
		if !strings.Contains(strings.Join(command, " "), required) {
			t.Fatalf("missing protection %q", required)
		}
	}
	if warmFingerprint("same", config) == warmFingerprint("same", func() Config { c := config; c.Profile = ""; return c }()) {
		t.Fatal("profile missing from warm fingerprint")
	}
	warm := strings.Join(warmCreateArguments(config, "local-test", "fingerprint"), " ")
	if !strings.Contains(warm, "--runtime runc") || strings.Contains(warm, "dst=/etc/resolv.conf") {
		t.Fatal("local warm container lost runc or Docker DNS")
	}
	brokerConfig := config
	brokerConfig.ScratchDirectory = "/runtime/scratch"
	brokerConfig.CLIBrokerSocket = filepath.Join(brokerConfig.ScratchDirectory, "broker", "cli-broker.sock")
	if _, err := New(brokerConfig); err != nil {
		t.Fatal(err)
	}
	warmBroker := strings.Join(warmCreateArguments(brokerConfig, "local-test", "fingerprint"), " ")
	if !strings.Contains(warmBroker, "dst=/run/agent-cli,readonly=true") {
		t.Fatal("local broker socket must be read-only")
	}
	production := validConfig(nil, nil)
	explicitProduction := production
	explicitProduction.Profile = "production"
	if warmFingerprint("same", production) != warmFingerprint("same", explicitProduction) {
		t.Fatal("explicit production profile invalidated legacy production fingerprint")
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.PublicEgressNetwork = "host" },
		func(c *Config) { c.CLIBrokerSocket = "/tmp/broker/cli-broker.sock" },
		func(c *Config) { c.ConnectorDirectory = "/tmp/connectors" },
	} {
		unsafe := config
		mutate(&unsafe)
		if _, err := New(unsafe); err == nil {
			t.Fatal("unsafe local override accepted")
		}
	}
	config.UID = 0
	if _, err := New(config); err == nil {
		t.Fatal("root local Runtime accepted")
	}
}
