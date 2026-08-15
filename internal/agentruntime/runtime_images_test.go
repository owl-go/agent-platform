package agentruntime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeDockerfilesPinOneCLIAndNonRootUser(t *testing.T) {
	tests := map[string]struct {
		version string
		install string
		entry   string
	}{
		"claude":   {version: "2.1.233", install: "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}", entry: `ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "claude"]`},
		"codex":    {version: "0.147.0", install: "@openai/codex@${CODEX_VERSION}", entry: `ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "codex"]`},
		"hermes":   {version: "0.19.0", install: "hermes-agent==${HERMES_VERSION}", entry: `ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "hermes"]`},
		"openclaw": {version: "2026.7.1-2", install: "openclaw@${OPENCLAW_VERSION}", entry: `ENTRYPOINT ["/usr/local/bin/runtime-entrypoint", "openclaw"]`},
	}
	for runtimeName, test := range tests {
		t.Run(runtimeName, func(t *testing.T) {
			path := filepath.Join("..", "..", "deploy", "runtimes", runtimeName, "Dockerfile")
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read Dockerfile: %v", err)
			}
			text := string(contents)
			for _, required := range []string{test.version, test.install, test.entry, "USER 65532:65532", "WORKDIR /workspace"} {
				if !strings.Contains(text, required) {
					t.Fatalf("Dockerfile does not contain %q", required)
				}
			}
		})
	}
}
