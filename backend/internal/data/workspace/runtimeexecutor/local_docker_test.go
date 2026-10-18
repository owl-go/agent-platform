package runtimeexecutor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the Linux test binary inside the local Worker, whose workspace path is
// shared with the Docker host. No provider credentials or model calls are used.
func TestLocalDockerReadOnlyMountCleanup(t *testing.T) {
	root := os.Getenv("WORKSPACE_TEST_LOCAL_DOCKER_ROOT")
	if root == "" {
		t.Skip("requires the explicit local Docker Worker and a shared workspace root")
	}
	if os.Getuid() != 0 || !filepath.IsAbs(root) || filepath.Clean(root) == "/" {
		t.Fatal("local Docker test requires the control Worker and an absolute workspace root")
	}
	directory, err := os.MkdirTemp(root, "local-mount-check-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	docker := func(args ...string) string {
		t.Helper()
		output, err := exec.CommandContext(t.Context(), "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	image := docker("image", "inspect", "agent-workspace-runtime:local", "--format", "{{.Id}}")
	name := "agent-workspace-" + filepath.Base(directory)
	workspace, attachments := filepath.Join(directory, "workspace"), filepath.Join(directory, "attachments")
	if err := os.Mkdir(attachments, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(attachments, 65532, 65532); err != nil {
		t.Fatal(err)
	}
	for iteration := 0; iteration < 12; iteration++ {
		if err := os.Mkdir(workspace, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Chown(workspace, 65532, 65532); err != nil {
			t.Fatal(err)
		}
		if err := prepareRuntimeAttachmentMountpoint(workspace, 65532, 65532); err != nil {
			t.Fatal(err)
		}
		if iteration == 0 {
			docker("create", "--name", name, "--runtime", "runc", "--user", "65532:65532",
				"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--network", "none",
				"--pids-limit", "512", "--memory", "256m", "--entrypoint", "/bin/sleep",
				"--mount", "type=bind,src="+workspace+",dst=/workspace",
				"--mount", "type=bind,src="+attachments+",dst=/workspace/.agent-platform-attachments,readonly=true", image, "infinity")
			t.Cleanup(func() {
				if output, err := exec.CommandContext(context.Background(), "docker", "rm", "--force", name).CombinedOutput(); err != nil {
					t.Errorf("remove test container: %v: %s", err, output)
				}
			})
		}
		docker("start", name)
		if uid := docker("exec", name, "id", "-u"); uid != "65532" {
			t.Fatalf("Runtime UID=%q", uid)
		}
		docker("stop", "--time", "0", name)
		if err := removeReleasedMountpoint(t.Context(), "local", func() error {
			return os.Remove(filepath.Join(workspace, ".agent-platform-attachments"))
		}); err != nil {
			t.Fatalf("iteration %d: %v", iteration+1, err)
		}
		if err := os.Remove(workspace); err != nil {
			t.Fatal(err)
		}
	}
}
