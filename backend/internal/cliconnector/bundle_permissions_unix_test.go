//go:build unix

package cliconnector

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// Isolate the process-wide umask from other tests. The all-in-one supervisor
// deliberately uses 0077 to protect credentials, including in its Worker.
func TestExtractBundleReadableBySandboxUnderPrivateUmask(t *testing.T) {
	if os.Getenv("AGENT_TEST_PRIVATE_BUNDLE_UMASK") != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestExtractBundleReadableBySandboxUnderPrivateUmask$")
		command.Env = append(os.Environ(), "AGENT_TEST_PRIVATE_BUNDLE_UMASK=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("private-umask extraction: %v\n%s", err, output)
		}
		return
	}
	previous := syscall.Umask(0o077)
	defer syscall.Umask(previous)
	root := t.TempDir()
	if err := extractBundle(testBundle(t, map[string]string{"node_modules/package/bin/tool": "#!/bin/sh\n"}), root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"node_modules", "node_modules/package", "node_modules/package/bin", "node_modules/package/bin/tool"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o555 != 0o555 {
			t.Errorf("sandbox cannot read/execute %s: %o", name, info.Mode().Perm())
		}
		if !info.IsDir() && info.Mode().Perm()&0o222 != 0 {
			t.Errorf("executable is writable: %o", info.Mode().Perm())
		}
	}
}
