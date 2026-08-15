package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agent-platform/internal/agentruntime"
	"agent-platform/internal/credentials"
)

func TestScanWorkspaceRejectsCredentialInGitMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := []byte("known-secret")
	if err := os.WriteFile(filepath.Join(root, ".git", "ignored"), secret, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanWorkspace(root, [][]byte{secret}); err == nil || !strings.Contains(err.Error(), "ignored") {
		t.Fatalf("expected Git metadata leak, got %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".git", "ignored")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "leak.txt")
	if err := os.WriteFile(path, append([]byte("prefix-"), secret...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanWorkspace(root, [][]byte{secret}); err == nil || !strings.Contains(err.Error(), "leak.txt") {
		t.Fatalf("expected credential leak, got %v", err)
	}
}

func TestScanWorkspaceRejectsCredentialInCompressedGitObject(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "user.email", "test@example.invalid")
	secret := []byte("credential-only-in-old-object")
	path := filepath.Join(root, "temporary-secret.txt")
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "add temporary file")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "remove temporary file")
	if err := scanWorkspace(root, [][]byte{secret}); err == nil || !strings.Contains(err.Error(), "Git object database") {
		t.Fatalf("expected historical Git object leak, got %v", err)
	}
}

func TestRedactResultProtectsFinalMessageAndArtifactReference(t *testing.T) {
	redactor := credentials.NewRedactor([]byte("known-secret"))
	result := redactResult(redactor, agentruntime.Result{
		FinalMessage: "model repeated known-secret",
		DiffArtifact: "artifact/known-secret/diff",
	})
	if strings.Contains(result.FinalMessage, "known-secret") || strings.Contains(result.DiffArtifact, "known-secret") {
		t.Fatalf("result was not redacted: %+v", result)
	}
}

func TestValidateOptionsKeepsEvidenceOutsideWorkspace(t *testing.T) {
	root := t.TempDir()
	opts := options{
		runtime: "claude", image: "image", model: "model", workspace: filepath.Join(root, "workspace"),
		credentialDir: filepath.Join(root, "credentials"), outputDir: filepath.Join(root, "workspace", "evidence"),
		runID: "run-1", network: "network", instruction: "task", timeout: 1,
	}
	if err := validateOptions(opts); err == nil {
		t.Fatal("expected evidence path rejection")
	}
}

func TestLoadCredentialPatternsAlsoProtectsTrimmedFileValue(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "key"), []byte("secret-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	patterns, err := loadCredentialPatterns(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 || string(patterns[0]) != "secret-value\n" || string(patterns[1]) != "secret-value" {
		t.Fatalf("patterns = %q", patterns)
	}
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
}
