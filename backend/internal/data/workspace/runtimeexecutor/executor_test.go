package runtimeexecutor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/credentials"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/objectstore/memory"
	"agent-platform/backend/internal/platformconfig"
)

func TestMaterializeAttachmentsVerifiesAndProtectsCopies(t *testing.T) {
	provider := memory.New()
	content := []byte("attachment contents")
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	key := "attachments/owner-1/11111111-1111-4111-8111-111111111111"
	if _, err := provider.Put(context.Background(), key, bytes.NewReader(content), objectstore.PutOptions{Size: int64(len(content)), SHA256: sha, ContentType: "text/plain", Metadata: map[string]string{"name": "notes.txt"}}); err != nil {
		t.Fatal(err)
	}
	executor := &Executor{objects: provider, config: platformconfig.Config{Worker: platformconfig.WorkerConfig{SandboxUID: os.Getuid(), SandboxGID: os.Getgid()}}}
	root := t.TempDir()
	paths, err := executor.materializeAttachments(context.Background(), application.ExecutionJob{OwnerID: "owner-1", Attachments: []domain.Attachment{{ID: "11111111-1111-4111-8111-111111111111", Name: "notes.txt", ObjectKey: key, Size: int64(len(content)), SHA256: sha}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths = %v", paths)
	}
	got, err := os.ReadFile(paths[0])
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("copy = %q, %v", got, err)
	}
	info, err := os.Stat(paths[0])
	if err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("copy mode = %v, %v", info.Mode().Perm(), err)
	}
}

func TestBuildInstructionAppliesPresetAndCustomPersonality(t *testing.T) {
	tests := []struct {
		name        string
		personality string
		guidance    string
		want        []string
	}{
		{name: "preset", personality: "direct_efficient", guidance: "Prefer bullet points.", want: []string{"Be direct, concise", "Prefer bullet points.", "Do the work"}},
		{name: "custom", personality: "custom", guidance: "Answer like a patient tutor.", want: []string{"Answer like a patient tutor.", "Do the work"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instruction := buildInstruction(application.ExecutionJob{
				Instruction: "Do the work",
				Snapshot:    domain.ExecutionSnapshot{Personality: test.personality, PersonalityInstructions: test.guidance},
			}, nil)
			for _, value := range test.want {
				if !strings.Contains(instruction, value) {
					t.Fatalf("instruction %q does not contain %q", instruction, value)
				}
			}
		})
	}
}

func TestSanitizeNativeStateRemovesTransientConfigAndRedactsSessionFiles(t *testing.T) {
	root := t.TempDir()
	secret := "native-state-secret-canary"
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("token = \""+secret+"\""), 0o600); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(root, "sessions", "2026", "08")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(sessions, "rollout.jsonl")
	if err := os.WriteFile(rollout, []byte(`{"message":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sanitizeNativeState(root, credentials.NewRedactor([]byte(secret))); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("transient config remains: %v", err)
	}
	content, err := os.ReadFile(rollout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), secret) || !strings.Contains(string(content), "[REDACTED]") {
		t.Fatalf("session state was not redacted: %s", content)
	}
}

func TestPreparePersistentWorkspaceTreeMakesMergedFilesReadableByThePlatformUser(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "output")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "result.txt")
	if err := os.WriteFile(file, []byte("done"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := preparePersistentWorkspaceTree(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{directory: 0o750, file: 0o600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Fatalf("%s mode = %o, want %o", path, got, want)
		}
	}
}

func TestPrepareRuntimeScratchCreatesAnInitiallyMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slot", "scratch")
	if err := prepareRuntimeScratch(path, os.Getuid(), os.Getgid()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o750 {
		t.Fatalf("Runtime scratch directory = mode %o, directory=%t", info.Mode().Perm(), info.IsDir())
	}
}

func TestWarmSlotIsStablePerResourceAndRuntime(t *testing.T) {
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: "/workspaces"},
		Worker:    platformconfig.WorkerConfig{CredentialTempRoot: "/credentials"},
	}}
	runtime := platformconfig.RuntimeEngineConfig{ImageDigest: "registry.example/runtime@sha256:" + strings.Repeat("a", 64)}
	first, slot, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	repeat, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobSession, OwnerID: "owner-1", SessionID: "session-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	workflow, _, err := executor.warmSlot(application.ExecutionJob{
		Kind: application.JobWorkflow, OwnerID: "owner-1", WorkflowID: "workflow-1",
		Snapshot: domain.ExecutionSnapshot{RuntimeEngine: domain.RuntimeCodex},
	}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	if first != repeat || first == workflow {
		t.Fatalf("warm container names: first=%q repeat=%q workflow=%q", first, repeat, workflow)
	}
	if !strings.Contains(slot.workspace, strings.TrimPrefix(first, "agent-runtime-warm-")) || !strings.HasPrefix(slot.credentials, "/credentials/") {
		t.Fatalf("warm slot paths = %+v", slot)
	}
}
