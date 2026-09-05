package cliconnector

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDockerBuildEnvironmentAppliesIsolatedPolicy(t *testing.T) {
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
		return os.WriteFile(host+"/bins.json", []byte(`{"tool":"bin/tool.js"}`), 0600)
	}
	config := DockerBuildConfig{DockerCommand: "docker", Runtime: "runsc", ImageDigest: "registry.example/cli-builder@sha256:" + strings.Repeat("a", 64), EgressNetwork: "npm-egress", ResolverConfig: "/etc/resolv.conf", UID: os.Getuid(), GID: os.Getgid(), Timeout: time.Minute}
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
	for _, required := range []string{"--runtime", "runsc", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--network", "npm-egress"} {
		if !slices.Contains(arguments, required) {
			t.Fatalf("missing %q in %v", required, arguments)
		}
	}
	if slices.Contains(arguments, "/var/run/docker.sock") {
		t.Fatalf("Docker socket mounted: %v", arguments)
	}
}

func TestDockerBuildEnvironmentRequiresPinnedImageAndRunsc(t *testing.T) {
	_, err := NewDockerBuildEnvironment(DockerBuildConfig{DockerCommand: "docker", Runtime: "runc", ImageDigest: "node:latest"}, func(context.Context, string, ...string) error { return nil })
	if err == nil {
		t.Fatal("expected unsafe configuration to fail")
	}
}
