package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-platform/backend/internal/objectstore/memory"
)

func TestUploadAndRestoreRoundTrip(t *testing.T) {
	provider := memory.New()
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "workspace.txt"), []byte("snapshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploaded, err := upload(context.Background(), provider, source, "phase-0/workspace.tar")
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.Action != "uploaded" || uploaded.SHA256 == "" || uploaded.Size == 0 {
		t.Fatalf("upload report = %+v", uploaded)
	}
	target := filepath.Join(t.TempDir(), "restored")
	restored, err := restore(context.Background(), provider, target, "phase-0/workspace.tar")
	if err != nil {
		t.Fatal(err)
	}
	if restored.SHA256 != uploaded.SHA256 || restored.Size != uploaded.Size {
		t.Fatalf("restore report = %+v, upload = %+v", restored, uploaded)
	}
	contents, err := os.ReadFile(filepath.Join(target, "workspace.txt"))
	if err != nil || string(contents) != "snapshot" {
		t.Fatalf("restored contents = %q, err = %v", contents, err)
	}
}
