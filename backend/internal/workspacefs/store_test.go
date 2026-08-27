package workspacefs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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
