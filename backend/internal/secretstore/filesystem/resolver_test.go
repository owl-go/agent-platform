package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-platform/backend/internal/credentials"
)

func TestResolverLoadsIsolatedSecretBundles(t *testing.T) {
	root := protectedDirectory(t, t.TempDir(), "secrets")
	model := protectedDirectory(t, root, "model")
	env := protectedDirectory(t, model, "env")
	writeSecret(t, env, "MODEL_API_KEY", "model-secret")
	git := protectedDirectory(t, root, "git")
	files := protectedDirectory(t, git, "files")
	gitFiles := protectedDirectory(t, files, "git")
	writeSecret(t, gitFiles, "id_ed25519", "private-key")
	writeSecret(t, gitFiles, "known_hosts", "github.com ssh-ed25519 key")

	resolver, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	request, err := resolver.Resolve(context.Background(), "run-1", []credentials.Binding{
		{Ref: "secret://model", Purpose: "model"}, {Ref: "secret://git", Purpose: "git_ssh"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Ref != "run-1" || request.Variables["MODEL_API_KEY"] != "model-secret" || string(request.Files["git/id_ed25519"]) != "private-key" {
		t.Fatalf("resolved Request = %+v", request)
	}
}

func TestResolverRejectsTraversalSymlinksAndUnsafeModes(t *testing.T) {
	root := protectedDirectory(t, t.TempDir(), "secrets")
	outside := protectedDirectory(t, t.TempDir(), "outside")
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	resolver, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range []string{"secret://../outside", "secret://linked"} {
		if _, err := resolver.Resolve(context.Background(), "run", []credentials.Binding{{Ref: reference, Purpose: "model"}}); err == nil {
			t.Fatalf("Resolve accepted %q", reference)
		}
	}
	unsafeRoot := filepath.Join(t.TempDir(), "unsafe")
	if err := os.Mkdir(unsafeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := New(unsafeRoot); err == nil {
		t.Fatal("New accepted group/world-readable Secret Store root")
	}
}

func protectedDirectory(t *testing.T, parent, name string) string {
	t.Helper()
	path := filepath.Join(parent, name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeSecret(t *testing.T, parent, name, value string) {
	t.Helper()
	path := filepath.Join(parent, name)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
}
