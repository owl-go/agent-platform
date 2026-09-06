package runtimeexecutor

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/platformconfig"
)

func TestStageWorkspaceBelongsToRuntimeUser(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "workspace-permissions-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	// The child must traverse the test parent, just as a bind mount exposes its root.
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	uid, gid := os.Getuid(), os.Getgid()
	if os.Geteuid() == 0 {
		uid, gid = 65532, 65532
	}
	persistent := filepath.Join(root, "persistent", "workflow")
	if err := os.MkdirAll(filepath.Join(persistent, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"README.md": "project", "src/main.txt": "source", "check.sh": "#!/bin/sh\nexit 0\n"} {
		mode := os.FileMode(0o600)
		if name == "check.sh" {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(persistent, name), []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	executor := &Executor{config: platformconfig.Config{
		Workspace: platformconfig.WorkspaceConfig{Root: filepath.Join(root, "persistent")},
		Worker:    platformconfig.WorkerConfig{SandboxUID: uid, SandboxGID: gid},
	}}
	job := application.ExecutionJob{Kind: application.JobWorkflow, Snapshot: domain.ExecutionSnapshot{WorkspacePath: "workflow"}}
	staged, _, _, err := executor.stageWorkspaceAt(job, filepath.Join(root, "staged"))
	if err != nil {
		t.Fatal(err)
	}
	// Use a separate unprivileged process when run by the Linux Worker identity.
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "runtimeexecutor.test")
	input, err := os.Open(testBinary)
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.OpenFile(binary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		_ = input.Close()
		t.Fatal(err)
	}
	_, copyErr := io.Copy(output, input)
	if err := errors.Join(copyErr, input.Close(), output.Close(), os.Chmod(binary, 0o755)); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-test.run=^TestRuntimeWorkspaceAccessHelper$")
	command.Env = append(os.Environ(), "TEST_RUNTIME_WORKSPACE="+staged)
	if os.Geteuid() == 0 {
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}}
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Runtime UID %d cannot use staged Workspace: %v\n%s", uid, err, output)
	}
	for name, mode := range map[string]os.FileMode{".": 0o700, "src": 0o700, "README.md": 0o600, "check.sh": 0o700} {
		info, err := os.Stat(filepath.Join(staged, name))
		if err != nil {
			t.Fatal(err)
		}
		stat := info.Sys().(*syscall.Stat_t)
		if int(stat.Uid) != uid || int(stat.Gid) != gid || info.Mode().Perm() != mode {
			t.Errorf("%s owner/mode = %d:%d %o, want %d:%d %o", name, stat.Uid, stat.Gid, info.Mode().Perm(), uid, gid, mode)
		}
	}
	if content, err := os.ReadFile(filepath.Join(persistent, "README.md")); err != nil || string(content) != "project" {
		t.Fatalf("staging changed persistent Workspace: %q, %v", content, err)
	}
}

func TestRuntimeWorkspaceAccessHelper(t *testing.T) {
	root := os.Getenv("TEST_RUNTIME_WORKSPACE")
	if root == "" {
		return
	}
	for _, name := range []string{"README.md", "src/main.txt"} {
		path := filepath.Join(root, name)
		if _, err := os.ReadFile(path); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte("updated"), 0o600); err != nil {
			t.Fatalf("edit %s: %v", name, err)
		}
	}
	directory := filepath.Join(root, "src", "created")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(directory, "new.txt")
	if err := os.WriteFile(file, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file, file+".renamed"); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(filepath.Join(root, "check.sh")).CombinedOutput(); err != nil {
		t.Fatalf("execute Workspace script: %v\n%s", err, output)
	}
}
