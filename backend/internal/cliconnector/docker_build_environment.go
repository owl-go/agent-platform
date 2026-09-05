package cliconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DockerBuildConfig struct {
	DockerCommand, Runtime, ImageDigest, EgressNetwork, ResolverConfig string
	UID, GID                                                           int
	Timeout                                                            time.Duration
}

type DockerCommand func(context.Context, string, ...string) error

type DockerBuildEnvironment struct {
	config DockerBuildConfig
	run    DockerCommand
}

var imageRepoDigest = regexp.MustCompile(`^[^\s@]+@sha256:[0-9a-f]{64}$`)

func NewDockerBuildEnvironment(config DockerBuildConfig, run DockerCommand) (*DockerBuildEnvironment, error) {
	if config.DockerCommand == "" || config.Runtime != "runsc" || !imageRepoDigest.MatchString(config.ImageDigest) || config.EgressNetwork == "" || !filepath.IsAbs(config.ResolverConfig) || config.UID <= 0 || config.GID <= 0 || config.Timeout <= 0 || config.Timeout > 30*time.Minute {
		return nil, errors.New("invalid isolated CLI Builder configuration")
	}
	if run == nil {
		run = func(ctx context.Context, command string, arguments ...string) error {
			output, err := exec.CommandContext(ctx, command, arguments...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}
	}
	return &DockerBuildEnvironment{config: config, run: run}, nil
}

func (environment *DockerBuildEnvironment) Build(ctx context.Context, request PackageBuildRequest) (PackageArtifact, error) {
	output, err := os.MkdirTemp("", "agent-cli-build-*")
	if err != nil {
		return PackageArtifact{}, err
	}
	defer os.RemoveAll(output)
	if err := os.Chown(output, environment.config.UID, environment.config.GID); err != nil {
		return PackageArtifact{}, fmt.Errorf("set CLI build output ownership: %w", err)
	}
	execution, cancel := context.WithTimeout(ctx, environment.config.Timeout)
	defer cancel()
	arguments := []string{
		"run", "--rm", "--runtime", environment.config.Runtime,
		"--user", fmt.Sprintf("%d:%d", environment.config.UID, environment.config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", environment.config.EgressNetwork,
		"--memory", strconv.FormatInt(1<<30, 10), "--cpus", "2", "--pids-limit", "256",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=268435456",
		"--tmpfs", "/work:rw,nosuid,nodev,size=536870912",
		"--mount", "type=bind,src=" + output + ",dst=/output,readonly=false",
		"--mount", "type=bind,src=" + environment.config.ResolverConfig + ",dst=/etc/resolv.conf,readonly=true",
		"--label", "agent-platform.managed=true", "--label", "agent-platform.workload=cli-builder",
		environment.config.ImageDigest, request.Package, request.ExactVersion, strings.Join(request.Architectures, ","),
	}
	if err := environment.run(execution, environment.config.DockerCommand, arguments...); err != nil {
		return PackageArtifact{}, fmt.Errorf("run isolated CLI Builder: %w", err)
	}
	packageBytes, err := os.ReadFile(filepath.Join(output, "package.tgz"))
	if err != nil {
		return PackageArtifact{}, fmt.Errorf("read exact npm package: %w", err)
	}
	bundleBytes, err := os.ReadFile(filepath.Join(output, "bundle.tgz"))
	if err != nil {
		return PackageArtifact{}, fmt.Errorf("read CLI bundle: %w", err)
	}
	integrityBytes, err := os.ReadFile(filepath.Join(output, "integrity.txt"))
	if err != nil {
		return PackageArtifact{}, fmt.Errorf("read npm integrity: %w", err)
	}
	var bins map[string]string
	binBytes, err := os.ReadFile(filepath.Join(output, "bins.json"))
	if err != nil || json.Unmarshal(binBytes, &bins) != nil {
		return PackageArtifact{}, errors.New("read CLI package bin metadata")
	}
	return PackageArtifact{PackageBytes: packageBytes, BundleBytes: bundleBytes, Integrity: strings.TrimSpace(string(integrityBytes)), Bins: bins}, nil
}

var _ PackageBuildEnvironment = (*DockerBuildEnvironment)(nil)
