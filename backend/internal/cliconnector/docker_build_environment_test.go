package cliconnector

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDockerBuildEnvironmentAppliesIsolatedPolicy(t *testing.T) {
	tempRoot := t.TempDir()
	var arguments []string
	run := func(_ context.Context, _ string, args ...string) error {
		arguments = slices.Clone(args)
		mount := args[slices.Index(args, "--mount")+1]
		host := strings.TrimSuffix(strings.TrimPrefix(mount, "type=bind,src="), ",dst=/output,readonly=false")
		if err := os.WriteFile(host+"/package.tgz", []byte("package"), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(host+"/bundle.tgz", []byte("bundle"), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(host+"/integrity.txt", []byte("sha512-value\n"), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(host+"/bins.json", []byte(`{"tool":"bin/tool.js"}`), 0600); err != nil {
			return err
		}
		return os.WriteFile(host+"/manifest.json", []byte(`{"name":"tool","version":"1.2.3","bin":{"tool":"bin/tool.js"}}`), 0600)
	}
	config := DockerBuildConfig{DockerCommand: "docker", Runtime: "runsc", ImageDigest: "registry.example/cli-builder@sha256:" + strings.Repeat("a", 64), EgressNetwork: "npm-egress", ResolverConfig: "/etc/resolv.conf", TempRoot: tempRoot, UID: os.Getuid(), GID: os.Getgid(), Timeout: time.Minute}
	environment, err := NewDockerBuildEnvironment(config, run)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := environment.Build(context.Background(), PackageBuildRequest{Package: "tool", ExactVersion: "1.2.3", Architectures: []string{"linux-amd64"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(artifact.BundleBytes) != "bundle" || artifact.Bins["tool"] != "bin/tool.js" {
		t.Fatalf("artifact = %#v", artifact)
	}
	for _, required := range []string{"--runtime", "runsc", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--network", "npm-egress", "/work:rw,nosuid,nodev,uid=" + strconv.Itoa(os.Getuid()) + ",gid=" + strconv.Itoa(os.Getgid()) + ",mode=0700,size=536870912"} {
		if !slices.Contains(arguments, required) {
			t.Fatalf("missing %q in %v", required, arguments)
		}
	}
	if slices.Contains(arguments, "/var/run/docker.sock") {
		t.Fatalf("Docker socket mounted: %v", arguments)
	}
	mount := arguments[slices.Index(arguments, "--mount")+1]
	if !strings.Contains(mount, "src="+tempRoot+string(os.PathSeparator)) {
		t.Fatalf("build output is not under the host-visible temp root: %s", mount)
	}
}

func TestDockerBuildEnvironmentRequiresPinnedImageAndRunsc(t *testing.T) {
	_, err := NewDockerBuildEnvironment(DockerBuildConfig{DockerCommand: "docker", Runtime: "runc", ImageDigest: "node:latest"}, func(context.Context, string, ...string) error { return nil })
	if err == nil {
		t.Fatal("expected unsafe configuration to fail")
	}
}
