package conformancepreflight_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCanaryDoesNotCountAsModelCredential(t *testing.T) {
	repoRoot := repositoryRoot(t)
	temp := t.TempDir()
	credentialDirectories := make([]string, 0, 5)
	for _, runtimeName := range []string{"claude", "codex", "hermes", "openclaw", "pi"} {
		directory := filepath.Join(temp, runtimeName)
		if err := os.MkdirAll(filepath.Join(directory, "env"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(directory, "git"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "env", "CONFORMANCE_CANARY_SECRET"), []byte("canary-secret-value"), 0o600); err != nil {
			t.Fatal(err)
		}
		credentialDirectories = append(credentialDirectories, directory)
	}

	command := exec.Command("bash", filepath.Join(repoRoot, "scripts", "conformance", "production-preflight.sh"))
	command.Env = append(os.Environ(),
		"CONFORMANCE_REPOSITORY_URL=git@example.com:owner/repository.git",
		"CONFORMANCE_BASE_BRANCH=phase0-fixture",
		"CONFORMANCE_WORK_ROOT="+filepath.Join(temp, "work"),
		"CONFORMANCE_EVIDENCE_ROOT="+filepath.Join(temp, "evidence"),
		"SANDBOX_REDIRECT_TEST_URL=https://example.com/redirect",
		"SANDBOX_REBIND_TEST_URL=http://127.0.0.1.example.test/",
		"SANDBOX_CONTROL_PLANE_TEST_URL=http://192.0.2.1/",
		"CONFORMANCE_CLAUDE_IMAGE=registry.example/claude@sha256:"+strings.Repeat("a", 64),
		"CONFORMANCE_CLAUDE_MODEL=model",
		"CONFORMANCE_CLAUDE_CREDENTIAL_DIR="+credentialDirectories[0],
		"CONFORMANCE_CODEX_IMAGE=registry.example/codex@sha256:"+strings.Repeat("b", 64),
		"CONFORMANCE_CODEX_MODEL=model",
		"CONFORMANCE_CODEX_CREDENTIAL_DIR="+credentialDirectories[1],
		"CONFORMANCE_HERMES_IMAGE=registry.example/hermes@sha256:"+strings.Repeat("c", 64),
		"CONFORMANCE_HERMES_MODEL=model",
		"CONFORMANCE_HERMES_CREDENTIAL_DIR="+credentialDirectories[2],
		"CONFORMANCE_OPENCLAW_IMAGE=registry.example/openclaw@sha256:"+strings.Repeat("d", 64),
		"CONFORMANCE_OPENCLAW_MODEL=model",
		"CONFORMANCE_OPENCLAW_CREDENTIAL_DIR="+credentialDirectories[3],
		"CONFORMANCE_PI_IMAGE=registry.example/pi@sha256:"+strings.Repeat("e", 64),
		"CONFORMANCE_PI_MODEL=model",
		"CONFORMANCE_PI_CREDENTIAL_DIR="+credentialDirectories[4],
		"ALIYUN_OSS_ENDPOINT=https://oss.example.com",
		"ALIYUN_OSS_ACCESS_KEY=access",
		"ALIYUN_OSS_SECRET_KEY=secret",
		"ALIYUN_OSS_BUCKET=bucket",
		"MINIO_ENDPOINT=127.0.0.1:9000",
		"MINIO_ACCESS_KEY=access",
		"MINIO_SECRET_KEY=secret",
		"MINIO_BUCKET=bucket",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("preflight unexpectedly accepted canary-only credential directories")
	}
	for _, runtimeName := range []string{"CLAUDE", "CODEX", "HERMES", "OPENCLAW", "PI"} {
		want := "CONFORMANCE_" + runtimeName + "_CREDENTIAL_DIR/env contains no model credential besides CONFORMANCE_CANARY_SECRET"
		if !strings.Contains(string(output), want) {
			t.Fatalf("preflight output does not contain %q:\n%s", want, output)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}
