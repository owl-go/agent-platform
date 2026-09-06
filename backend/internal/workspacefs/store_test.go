package workspacefs

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestWriteSSHFilesProducesReadableKeyWithoutFinalNewline(t *testing.T) {
	keygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("OpenSSH ssh-keygen is required to verify private-key file compatibility")
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "synthetic-regression")
	if err != nil {
		t.Fatal(err)
	}
	encoded := bytes.TrimSuffix(pem.EncodeToMemory(block), []byte("\n"))
	original := bytes.Clone(encoded)
	root := t.TempDir()
	knownHosts := filepath.Join(root, "known_hosts")
	if err := os.WriteFile(knownHosts, []byte("git.example.test ssh-ed25519 AAAA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(filepath.Join(root, "workspaces"), knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	home, _, keyPath, cleanup, err := store.writeSSHFiles(encoded, "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	output, err := exec.Command(keygen, "-y", "-P", "", "-f", keyPath).Output()
	if err != nil {
		t.Fatalf("OpenSSH cannot read the materialized private key: %v", err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(output)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(publicKey.Marshal(), expected.Marshal()) {
		t.Fatal("materialization changed the key identity")
	}
	if !bytes.Equal(encoded, original) {
		t.Fatal("materialization changed the caller's private key bytes")
	}
	if info, err := os.Stat(keyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("private key permissions must remain 0600")
	}
	cleanup()
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("temporary SSH material was not removed")
	}
}

func TestStoreRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	store, err := New(root, "")
	if err != nil {
		t.Fatal(err)
	}
	workspace := "users/u/workflows/w"
	if _, err := store.CreateDirectory(context.Background(), workspace, "safe"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, filepath.FromSlash(workspace), "safe", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Read(context.Background(), workspace, "safe/escape"); err == nil {
		t.Fatal("expected symlink read to be rejected")
	}
	if _, _, err := store.List(context.Background(), workspace, "safe"); err == nil {
		t.Fatal("expected symlink listing to be rejected")
	}
}

func TestStoreUploadAndOpen(t *testing.T) {
	store, err := New(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	const workspace = "users/u/workflows/w"
	if _, err := store.Upload(context.Background(), workspace, "notes/result.txt", []byte("done")); err != nil {
		t.Fatal(err)
	}
	file, info, err := store.Open(context.Background(), workspace, "notes/result.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if info.Size() != 4 {
		t.Fatalf("size = %d", info.Size())
	}
}

func TestWriteSSHFilesMaterializesWorkflowConfigAndConfiguredIdentity(t *testing.T) {
	root := t.TempDir()
	knownHosts := filepath.Join(root, "known_hosts")
	if err := os.WriteFile(knownHosts, []byte("git.example.test ssh-ed25519 AAAA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(filepath.Join(root, "workspaces"), knownHosts)
	if err != nil {
		t.Fatal(err)
	}
	sshConfig := "Host agent-platform\n  HostName 47.237.108.63\n  User root\n  IdentityFile ~/.ssh/xinjiapo.pem\n  IdentitiesOnly yes\n"
	home, configPath, keyPath, cleanup, err := store.writeSSHFiles([]byte("private-key"), sshConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if configPath != filepath.Join(home, ".ssh", "config") || keyPath != filepath.Join(home, ".ssh", "xinjiapo.pem") {
		t.Fatalf("SSH paths = %q, %q under %q", configPath, keyPath, home)
	}
	config, err := os.ReadFile(configPath)
	if err != nil || string(config) != sshConfig {
		t.Fatalf("SSH config = %q, %v", config, err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil || string(key) != "private-key\n" {
		t.Fatalf("SSH key = %q, %v", key, err)
	}
	if info, err := os.Stat(configPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("SSH config mode = %v, %v", info, err)
	}
	if !strings.HasSuffix(keyPath, filepath.Join(".ssh", "xinjiapo.pem")) {
		t.Fatalf("identity path = %q", keyPath)
	}
}
