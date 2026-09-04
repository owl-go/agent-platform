package workspacefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeSessionStatePathAndRemovalStayWithinRoot(t *testing.T) {
	root := t.TempDir()
	path, err := NativeSessionStatePath(root, "owner-1", "session-1", "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".native-session-state", "owner-1", "session-1", "codex")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RemoveNativeSessionState(root, "owner-1", "session-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("session state still exists: %v", err)
	}
}

func TestNativeExpertSessionStatePathIncludesFrozenIdentity(t *testing.T) {
	root := t.TempDir()
	path, err := NativeExpertSessionStatePath(root, "owner-1", "session-1", "expert-1", 7, "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".native-session-state", "owner-1", "session-1", "expert-expert-1-v7-codex")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestNativeExpertRunConversationStatePathIncludesFrozenIdentity(t *testing.T) {
	root := t.TempDir()
	path, err := NativeExpertRunConversationStatePath(root, "owner-1", "conversation-1", "expert-1", 7, "codex")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".native-run-conversation-state", "owner-1", "conversation-1", "expert-expert-1-v7-codex")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestNativeSessionStateRejectsPathTraversal(t *testing.T) {
	for _, value := range []string{"", ".", "..", "../other", "owner/session"} {
		if _, err := NativeSessionStatePath(t.TempDir(), value, "session-1", "codex"); err == nil {
			t.Fatalf("owner %q was accepted", value)
		}
	}
}
