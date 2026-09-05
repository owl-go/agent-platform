package workspaceworker

import (
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/platformconfig"
)

func TestNewCLIConnectorBuilderUsesAvailablePinnedRuntimes(t *testing.T) {
	config := platformconfig.Config{
		Worker: platformconfig.WorkerConfig{
			SandboxUID: 65532, SandboxGID: 65532,
			CLIBuilder: platformconfig.CLIBuilderConfig{Enabled: true, ImageDigest: "registry.example/builder@sha256:" + strings.Repeat("a", 64), EgressNetwork: "agent-npm-egress", Timeout: platformconfig.Duration(10 * time.Minute)},
			Runtimes: map[string]platformconfig.RuntimeEngineConfig{
				"codex":  {Available: true, ImageDigest: "registry.example/codex@sha256:" + strings.Repeat("b", 64), CLIVersion: "test"},
				"claude": {Available: false},
			},
		},
		Sandbox: platformconfig.SandboxConfig{Runtime: "runsc", ResolverConfig: "/etc/resolv.conf"},
	}
	builder, err := newCLIConnectorBuilder(config, memory.New())
	if err != nil {
		t.Fatal(err)
	}
	if builder == nil || len(builder.RuntimeDigests) != 1 || builder.RuntimeDigests[0] != "sha256:"+strings.Repeat("b", 64) {
		t.Fatalf("builder = %#v", builder)
	}
}

func TestNewCLIConnectorBuilderStaysDisabledByDefault(t *testing.T) {
	builder, err := newCLIConnectorBuilder(platformconfig.Config{}, memory.New())
	if err != nil || builder != nil {
		t.Fatalf("builder=%#v err=%v", builder, err)
	}
}
