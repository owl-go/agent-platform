package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanWorkspaceRejectsCredentialOutsideGitMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := []byte("known-secret")
	if err := os.WriteFile(filepath.Join(root, ".git", "ignored"), secret, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanWorkspace(root, [][]byte{secret}); err != nil {
		t.Fatalf("Git metadata should be skipped: %v", err)
	}
	path := filepath.Join(root, "leak.txt")
	if err := os.WriteFile(path, append([]byte("prefix-"), secret...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := scanWorkspace(root, [][]byte{secret}); err == nil || !strings.Contains(err.Error(), "leak.txt") {
		t.Fatalf("expected credential leak, got %v", err)
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
