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

	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/sandbox"
)

var (
	imageDigestPattern  = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
	identifierPattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	runIDPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	workflowPlanPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type RunHost func(context.Context, processharness.Spec, processharness.OutputSink) (processharness.Result, error)
type Cleanup func(context.Context, string) error
type NameFactory func() (string, error)
type ScratchPreparer func(string, int, int) error
type ControlContainer func(context.Context, string, bool) error

type Config struct {
	DockerCommand        string
	Image                string
	RuntimeCommand       string
	RunID                string
	Runtime              string
	WorkspaceDirectory   string
	WorkspaceVolume      string
	ContainerWorkspace   string
	CredentialDirectory  string
	NativeStateDirectory string
	ScratchDirectory     string
	PublicEgressNetwork  string
	ResolverConfigFile   string
	Egress               sandbox.EgressMode
	Limits               sandbox.Limits
	UID                  int
	GID                  int
	RunHost              RunHost
	Cleanup              Cleanup
	Name                 NameFactory
	PrepareScratch       ScratchPreparer
	ControlContainer     ControlContainer
	WorkflowPlan         string
	DirectCommand        bool
}

func New(config Config) (cliadapter.RunProcess, error) {
	if config.ContainerWorkspace == "" {
		config.ContainerWorkspace = "/workspace"
	}
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
	if config.ControlContainer == nil {
		config.ControlContainer = dockerControl(config.DockerCommand)
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
		if observer, ok := spec.Observer.(processharness.ProcessAwareObserver); ok {
			hostSpec.Observer = &containerAwareObserver{
				observer:   observer,
				controller: &containerController{ctx: ctx, name: name, control: config.ControlContainer},
			}
		}
		if config.WorkspaceVolume != "" {
			hostSpec.Dir = ""
		}
		result, returnErr = config.RunHost(ctx, hostSpec, sink)

		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := config.Cleanup(cleanupCtx, name); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("remove sandbox container: %w", err))
		}
		return result, returnErr
	}, nil
}

type containerAwareObserver struct {
	observer   processharness.ProcessAwareObserver
	controller processharness.ProcessController
}

func (observer *containerAwareObserver) Observe(ctx context.Context, stream processharness.Stream, data []byte) error {
	return observer.observer.Observe(ctx, stream, data)
}

func (observer *containerAwareObserver) BindProcess(processharness.ProcessController) {
	observer.observer.BindProcess(observer.controller)
}

type containerController struct {
	ctx     context.Context
	name    string
	control ControlContainer
}

func (controller *containerController) Pause() error  { return controller.execute(true) }
func (controller *containerController) Resume() error { return controller.execute(false) }

func (controller *containerController) execute(paused bool) error {
	controlCtx, cancel := context.WithTimeout(context.WithoutCancel(controller.ctx), 10*time.Second)
	defer cancel()
	return controller.control(controlCtx, controller.name, paused)
}

func dockerControl(command string) ControlContainer {
	return func(ctx context.Context, name string, paused bool) error {
		if paused {
			return runDockerControl(ctx, command, "pause", name)
		}
		if err := runDockerControl(ctx, command, "unpause", name); err != nil {
			return err
		}
		return runDockerControl(ctx, command, "kill", "--signal", "CONT", name)
	}
}

func runDockerControl(ctx context.Context, command string, arguments ...string) error {
	output, err := exec.CommandContext(ctx, command, arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker container control %v: %w: %s", arguments, err, strings.TrimSpace(string(output)))
	}
	return nil
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
	if (config.WorkspaceDirectory == "") == (config.WorkspaceVolume == "") {
		return fmt.Errorf("exactly one workspace directory or volume is required")
	}
	if config.WorkspaceDirectory != "" && !filepath.IsAbs(config.WorkspaceDirectory) {
		return fmt.Errorf("workspace directory must be absolute")
	}
	if config.WorkspaceVolume != "" && !runIDPattern.MatchString(config.WorkspaceVolume) {
		return fmt.Errorf("workspace volume is invalid")
	}
	if !filepath.IsAbs(config.ContainerWorkspace) || config.ContainerWorkspace == "/" || strings.Contains(config.ContainerWorkspace, ",") {
		return fmt.Errorf("container workspace must be an absolute non-root path without commas")
	}
	if !filepath.IsAbs(config.CredentialDirectory) {
		return fmt.Errorf("credential directory must be absolute")
	}
	if config.NativeStateDirectory != "" && !filepath.IsAbs(config.NativeStateDirectory) {
		return fmt.Errorf("native Runtime state directory must be absolute")
	}
	if config.ScratchDirectory != "" && !filepath.IsAbs(config.ScratchDirectory) {
		return fmt.Errorf("Runtime scratch directory must be absolute")
	}
	if config.NativeStateDirectory != "" && config.RuntimeCommand != "codex" {
		return fmt.Errorf("native Runtime state directory is only supported for Codex")
	}
	if config.Egress != sandbox.EgressNone && config.Egress != sandbox.EgressPublic {
		return fmt.Errorf("invalid egress mode")
	}
	if config.Egress == sandbox.EgressPublic && config.PublicEgressNetwork == "" {
		return fmt.Errorf("public egress network is required")
	}
	if config.Egress == sandbox.EgressPublic && (!filepath.IsAbs(config.ResolverConfigFile) || strings.Contains(config.ResolverConfigFile, ",")) {
		return fmt.Errorf("public egress resolver config must be an absolute path without commas")
	}
	if config.Limits.CPUs <= 0 || config.Limits.MemoryBytes <= 0 || config.Limits.PIDs <= 0 || config.Limits.TempBytes <= 0 {
		return fmt.Errorf("positive CPU, memory, PID, and temporary storage limits are required")
	}
	if config.WorkflowPlan != "" && (len(config.WorkflowPlan) > 256*1024 || !workflowPlanPattern.MatchString(config.WorkflowPlan)) {
		return fmt.Errorf("encoded Git workflow plan is invalid")
	}
	return nil
}

func validateProcessSpec(config Config, spec processharness.Spec) error {
	if len(spec.Command) == 0 || spec.Command[0] != config.RuntimeCommand {
		return fmt.Errorf("runtime command does not match configured image")
	}
	expectedWorkspace := config.WorkspaceDirectory
	if config.WorkspaceVolume != "" {
		expectedWorkspace = config.ContainerWorkspace
	}
	if !filepath.IsAbs(spec.Dir) || filepath.Clean(spec.Dir) != filepath.Clean(expectedWorkspace) {
		return fmt.Errorf("workspace directory does not match configured run workspace")
	}
	seenEnvironment := make(map[string]struct{}, len(spec.Env))
	for _, entry := range spec.Env {
		name, _, _ := strings.Cut(entry, "=")
		if !identifierPattern.MatchString(name) {
			return fmt.Errorf("invalid non-secret environment variable")
		}
		if _, duplicate := seenEnvironment[name]; duplicate {
			return fmt.Errorf("duplicate non-secret environment variable %s", name)
		}
		seenEnvironment[name] = struct{}{}
		credentialPath := filepath.Join(config.CredentialDirectory, "env", name)
		if _, err := os.Lstat(credentialPath); err == nil {
			return fmt.Errorf("credential environment cannot override platform variable %s", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect credential environment variable %s: %w", name, err)
		}
	}
	return nil
}

func dockerCommand(config Config, spec processharness.Spec, name string, scratchDirectories []string) []string {
	network := "none"
	if config.Egress == sandbox.EgressPublic {
		network = config.PublicEgressNetwork
	}
	workspaceMount := "type=bind,src=" + spec.Dir + ",dst=" + config.ContainerWorkspace + ",readonly=false"
	if config.WorkspaceVolume != "" {
		workspaceMount = "type=volume,src=" + config.WorkspaceVolume + ",dst=" + config.ContainerWorkspace + ",readonly=false"
	}
	args := []string{
		config.DockerCommand, "run", "--name", name, "--rm", "--interactive",
		"--runtime", config.Runtime,
		"--user", fmt.Sprintf("%d:%d", config.UID, config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", network,
		"--memory", strconv.FormatInt(config.Limits.MemoryBytes, 10),
		"--pids-limit", strconv.FormatInt(config.Limits.PIDs, 10),
		"--cpus", strconv.FormatFloat(config.Limits.CPUs, 'f', -1, 64),
		"--mount", workspaceMount,
		"--mount", "type=bind,src=" + config.CredentialDirectory + ",dst=/run/agent-credentials,readonly=true",
	}
	if config.NativeStateDirectory != "" {
		args = append(args, "--mount", "type=bind,src="+config.NativeStateDirectory+",dst=/tmp/runtime-home/.codex,readonly=false")
	}
	if config.Egress == sandbox.EgressPublic {
		args = append(args, "--mount", "type=bind,src="+config.ResolverConfigFile+",dst=/etc/resolv.conf,readonly=true")
	}
	for _, directory := range scratchDirectories {
		args = append(args, "--mount", "type=bind,src="+directory+",dst="+directory+",readonly=false")
	}
	if config.DirectCommand {
		args = append(args, "--entrypoint", "/usr/local/bin/runtime-entrypoint")
	}
	args = append(args,
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size="+strconv.FormatInt(config.Limits.TempBytes, 10),
		"--workdir", config.ContainerWorkspace,
	)
	for _, entry := range spec.Env {
		name, _, _ := strings.Cut(entry, "=")
		args = append(args, "--env", name)
	}
	// Describe has no observer and must probe the pinned CLI directly. Only an
	// Execute invocation can consume and resume the approval protocol.
	if config.WorkflowPlan != "" && spec.Observer != nil {
		args = append(args, "--env", "AGENT_PLATFORM_WORKFLOW_B64="+config.WorkflowPlan)
	}
	args = append(args,
		"--init",
		"--label", "agent-platform.managed=true",
		"--label", "agent-platform.run-id="+config.RunID,
		config.Image,
	)
	if config.DirectCommand {
		return append(args, spec.Command...)
	}
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
