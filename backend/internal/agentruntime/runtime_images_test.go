package agentruntime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedRuntimeDockerfilePinsFiveCLIsAndNonRootUser(t *testing.T) {
	tests := map[string]struct {
		version string
		install string
	}{
		"claude":   {version: "2.1.233", install: "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}"},
		"codex":    {version: "0.147.0", install: "@openai/codex@${CODEX_VERSION}"},
		"hermes":   {version: "0.19.0", install: "hermes-agent[mcp,anthropic]==${HERMES_VERSION}"},
		"openclaw": {version: "2026.7.1-2", install: "openclaw@${OPENCLAW_VERSION}"},
		"pi":       {version: "0.84.4", install: "@earendil-works/pi-coding-agent@${PI_VERSION}"},
	}
	path := filepath.Join("..", "..", "..", "deploy", "runtimes", "unified", "Dockerfile")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read unified Dockerfile: %v", err)
	}
	text := string(contents)
	for _, required := range []string{`ENTRYPOINT ["/usr/local/bin/runtime-entrypoint"]`, "USER 65532:65532", "WORKDIR /workspace", "FROM public.ecr.aws/docker/library/node:24", "FROM public.ecr.aws/docker/library/python:3.13", "@sha256:"} {
		if !strings.Contains(text, required) {
			t.Fatalf("unified Dockerfile does not contain %q", required)
		}
	}
	for runtimeName, test := range tests {
		t.Run(runtimeName, func(t *testing.T) {
			for _, required := range []string{test.version, test.install} {
				if !strings.Contains(text, required) {
					t.Fatalf("unified Dockerfile does not contain %q", required)
				}
			}
		})
	}
}
