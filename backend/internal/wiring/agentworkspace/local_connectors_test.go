package agentworkspace

import (
	"strings"
	"testing"

	workspacerepo "agent-platform/backend/internal/data/workspace/gormrepo"
	"agent-platform/backend/internal/platformconfig"
)

func TestAPIConnectorEvidenceOnlyOptsInToExplicitLocalProfile(t *testing.T) {
	for _, profile := range []string{"", "production"} {
		r := workspacerepo.New(nil, nil)
		// API-only production YAML has no Worker runtime and must remain valid.
		if err := configureLocalAPIConnectors(r, platformconfig.Config{Sandbox: platformconfig.SandboxConfig{Profile: profile}}); err != nil || r.LocalConnectorVerification() {
			t.Fatalf("production API acquired Worker dependency: %v", err)
		}
	}
	r := workspacerepo.New(nil, nil)
	image := "sha256:" + strings.Repeat("a", 64)
	cfg := platformconfig.Config{Sandbox: platformconfig.SandboxConfig{Profile: "local", Runtime: "runc"}, Worker: platformconfig.WorkerConfig{Runtimes: map[string]platformconfig.RuntimeEngineConfig{"codex": {Available: true, ImageDigest: image}}}}
	if err := configureLocalAPIConnectors(r, cfg); err != nil || !r.LocalConnectorVerification() {
		t.Fatalf("explicit local profile not configured: %v", err)
	}
	if got := r.ConnectorVerificationDigests("production-digest"); len(got) != 1 || got[0] != image {
		t.Fatalf("wrong local verification target: %v", got)
	}
}
