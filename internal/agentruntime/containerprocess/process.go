package containerprocess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"agent-platform/internal/agentruntime/cliadapter"
	"agent-platform/internal/agentruntime/processharness"
	"agent-platform/internal/sandbox"
)

var (
	imageDigestPattern = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
	identifierPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	runIDPattern       = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

type RunHost func(context.Context, processharness.Spec, processharness.OutputSink) (processharness.Result, error)
type Cleanup func(context.Context, string) error
type NameFactory func() (string, error)
type ScratchPreparer func(string, int, int) error

type Config struct {
	DockerCommand       string
	Image               string
	RuntimeCommand      string
	RunID               string
	Runtime             string
	WorkspaceDirectory  string
	CredentialDirectory string
	PublicEgressNetwork string
	Egress              sandbox.EgressMode
	Limits              sandbox.Limits
	UID                 int
	GID                 int
	RunHost             RunHost
	Cleanup             Cleanup
	Name                NameFactory
	PrepareScratch      ScratchPreparer
}

func New(config Config) (cliadapter.RunProcess, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	if config.DockerCommand == "" {
		config.DockerCommand = "docker"
	}
	if config.RunHost == nil {
		config.RunHost = processharness.Run
	}
	if config.Cleanup == nil {
		config.Cleanup = dockerCleanup(config.DockerCommand)
	}
	if config.Name == nil {
		config.Name = randomName
	}
	if config.PrepareScratch == nil {
		config.PrepareScratch = prepareScratch
	}

	return func(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (result processharness.Result, returnErr error) {
		if spec.Dir == "" {
			spec.Dir = config.WorkspaceDirectory
		}
		if err := validateProcessSpec(config, spec); err != nil {
			return processharness.Result{}, err
		}
		name, err := config.Name()
		if err != nil {
			return processharness.Result{}, fmt.Errorf("create sandbox container name: %w", err)
		}
		if !runIDPattern.MatchString(name) {
			return processharness.Result{}, fmt.Errorf("invalid sandbox container name")
		}
		scratchDirectories, err := findScratchDirectories(spec.Command)
		if err != nil {
			return processharness.Result{}, err
		}
		for _, directory := range scratchDirectories {
			if err := config.PrepareScratch(directory, config.UID, config.GID); err != nil {
				return processharness.Result{}, fmt.Errorf("prepare adapter scratch directory: %w", err)
			}
		}

		hostSpec := spec
		hostSpec.Command = dockerCommand(config, spec, name, scratchDirectories)
		result, returnErr = config.RunHost(ctx, hostSpec, sink)

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := config.Cleanup(cleanupCtx, name); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove sandbox container: %w", err))
		}
		return result, returnErr
	}, nil
}

func validateConfig(config Config) error {
	if !imageDigestPattern.MatchString(config.Image) {
		return fmt.Errorf("runtime image must use an immutable sha256 digest")
	}
	if config.RuntimeCommand == "" || strings.ContainsAny(config.RuntimeCommand, "/\\\x00") {
		return fmt.Errorf("runtime command must be a bare executable name")
	}
	if !runIDPattern.MatchString(config.RunID) {
		return fmt.Errorf("run ID is invalid")
	}
	if config.Runtime != "runsc" {
		return fmt.Errorf("sandbox runtime must be runsc")
	}
	if config.UID <= 0 || config.GID <= 0 {
		return fmt.Errorf("sandbox UID and GID must be non-root")
	}
	if !filepath.IsAbs(config.WorkspaceDirectory) {
		return fmt.Errorf("workspace directory must be absolute")
	}
	if !filepath.IsAbs(config.CredentialDirectory) {
		return fmt.Errorf("credential directory must be absolute")
	}
	if config.Egress != sandbox.EgressNone && config.Egress != sandbox.EgressPublic {
		return fmt.Errorf("invalid egress mode")
	}
	if config.Egress == sandbox.EgressPublic && config.PublicEgressNetwork == "" {
		return fmt.Errorf("public egress network is required")
	}
	if config.Limits.CPUs <= 0 || config.Limits.MemoryBytes <= 0 || config.Limits.PIDs <= 0 || config.Limits.TempBytes <= 0 {
		return fmt.Errorf("positive CPU, memory, PID, and temporary storage limits are required")
	}
	return nil
}

func validateProcessSpec(config Config, spec processharness.Spec) error {
	if len(spec.Command) == 0 || spec.Command[0] != config.RuntimeCommand {
		return fmt.Errorf("runtime command does not match configured image")
	}
	if !filepath.IsAbs(spec.Dir) || filepath.Clean(spec.Dir) != filepath.Clean(config.WorkspaceDirectory) {
		return fmt.Errorf("workspace directory does not match configured run workspace")
	}
	for _, entry := range spec.Env {
		name, _, _ := strings.Cut(entry, "=")
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("invalid non-secret environment variable")
		}
	}
	return nil
}

func dockerCommand(config Config, spec processharness.Spec, name string, scratchDirectories []string) []string {
	network := "none"
	if config.Egress == sandbox.EgressPublic {
		network = config.PublicEgressNetwork
	}
	args := []string{
		config.DockerCommand, "run", "--name", name, "--rm",
		"--runtime", config.Runtime,
		"--user", fmt.Sprintf("%d:%d", config.UID, config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", network,
		"--memory", strconv.FormatInt(config.Limits.MemoryBytes, 10),
		"--pids-limit", strconv.FormatInt(config.Limits.PIDs, 10),
		"--cpus", strconv.FormatFloat(config.Limits.CPUs, 'f', -1, 64),
		"--mount", "type=bind,src=" + spec.Dir + ",dst=/workspace,rw",
		"--mount", "type=bind,src=" + config.CredentialDirectory + ",dst=/run/agent-credentials,ro",
	}
	for _, directory := range scratchDirectories {
		args = append(args, "--mount", "type=bind,src="+directory+",dst="+directory+",rw")
	}
	args = append(args,
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size="+strconv.FormatInt(config.Limits.TempBytes, 10),
		"--workdir", "/workspace",
	)
	for _, entry := range spec.Env {
		name, _, _ := strings.Cut(entry, "=")
		args = append(args, "--env", name)
	}
	args = append(args,
		"--init",
		"--label", "agent-platform.managed=true",
		"--label", "agent-platform.run-id="+config.RunID,
		config.Image,
	)
	return append(args, spec.Command[1:]...)
}

func findScratchDirectories(command []string) ([]string, error) {
	temporaryRoot := filepath.Clean(os.TempDir())
	directories := make(map[string]struct{})
	for _, argument := range command[1:] {
		if !filepath.IsAbs(argument) {
			continue
		}
		cleaned := filepath.Clean(argument)
		relative, err := filepath.Rel(temporaryRoot, cleaned)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			continue
		}
		first := strings.Split(relative, string(os.PathSeparator))[0]
		if !strings.HasPrefix(first, "agent-runtime-adapter-") {
			continue
		}
		directory := filepath.Join(temporaryRoot, first)
		info, err := os.Stat(directory)
		if err != nil {
			return nil, fmt.Errorf("inspect adapter scratch directory: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("adapter scratch path is not a directory")
		}
		directories[directory] = struct{}{}
	}
	result := make([]string, 0, len(directories))
	for directory := range directories {
		result = append(result, directory)
	}
	sort.Strings(result)
	return result, nil
}

func prepareScratch(root string, uid, gid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("adapter scratch directory contains symlink %q", path)
		}
		return os.Chown(path, uid, gid)
	})
}

func randomName() (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return "agent-runtime-" + hex.EncodeToString(suffix[:]), nil
}

func dockerCleanup(dockerCommand string) Cleanup {
	return func(ctx context.Context, name string) error {
		output, err := exec.CommandContext(ctx, dockerCommand, "rm", "--force", name).CombinedOutput()
		if err != nil && !strings.Contains(strings.ToLower(string(output)), "no such container") {
			return fmt.Errorf("docker rm: %s: %w", strings.TrimSpace(string(output)), err)
		}
		return nil
	}
}
