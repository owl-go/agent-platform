package containerprocess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"agent-platform/backend/internal/agentruntime/cliadapter"
	"agent-platform/backend/internal/agentruntime/processharness"
	"agent-platform/backend/internal/sandbox"
)

const warmContainerPrefix = "agent-runtime-warm-"

type DockerCLI func(context.Context, ...string) ([]byte, error)

// WarmManager keeps an isolated container definition for a Session or Workflow
// while still stopping all Runtime processes between executions.
type WarmManager struct {
	dockerCommand string
	idleTimeout   time.Duration
	runHost       RunHost
	docker        DockerCLI
	now           func() time.Time
	mu            sync.Mutex
}

type WarmLease struct {
	manager *WarmManager
	name    string
	started bool
	closed  bool
}

func NewWarmManager(dockerCommand string, idleTimeout time.Duration) (*WarmManager, error) {
	if strings.TrimSpace(dockerCommand) == "" {
		dockerCommand = "docker"
	}
	if idleTimeout <= 0 {
		return nil, fmt.Errorf("warm Runtime container idle timeout must be positive")
	}
	manager := &WarmManager{dockerCommand: dockerCommand, idleTimeout: idleTimeout, runHost: processharness.Run, now: time.Now}
	manager.docker = func(ctx context.Context, arguments ...string) ([]byte, error) {
		return exec.CommandContext(ctx, manager.dockerCommand, arguments...).CombinedOutput()
	}
	return manager, nil
}

// WarmContainerName is stable across Worker restarts and changes whenever the
// owner scope, Runtime image, or Runtime Engine changes.
func WarmContainerName(scopeKey, runtimeCommand, image string) (string, error) {
	if strings.TrimSpace(scopeKey) == "" || strings.ContainsRune(scopeKey, '\x00') {
		return "", fmt.Errorf("warm Runtime scope key is invalid")
	}
	if strings.TrimSpace(runtimeCommand) == "" || !imageDigestPattern.MatchString(image) {
		return "", fmt.Errorf("warm Runtime identity is invalid")
	}
	digest := sha256.Sum256([]byte(scopeKey + "\x00" + runtimeCommand + "\x00" + image))
	return warmContainerPrefix + hex.EncodeToString(digest[:16]), nil
}

// Checkout stops any previously retained container before the caller replaces
// its bind-mounted per-execution files. The lease serializes setup and execution
// against the idle reaper.
func (manager *WarmManager) Checkout(ctx context.Context, name string) (*WarmLease, error) {
	if !strings.HasPrefix(name, warmContainerPrefix) || !runIDPattern.MatchString(name) {
		return nil, fmt.Errorf("warm Runtime container name is invalid")
	}
	manager.mu.Lock()
	if err := manager.stopIfRunning(ctx, name); err != nil {
		manager.mu.Unlock()
		return nil, err
	}
	return &WarmLease{manager: manager, name: name}, nil
}

func (lease *WarmLease) Start(ctx context.Context, config Config) (cliadapter.RunProcess, error) {
	if lease == nil || lease.manager == nil || lease.closed {
		return nil, fmt.Errorf("warm Runtime lease is not active")
	}
	if lease.started {
		return nil, fmt.Errorf("warm Runtime lease already started")
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	fingerprint := warmFingerprint(lease.name, config)
	exists, label, err := lease.manager.inspectFingerprint(ctx, lease.name)
	if err != nil {
		return nil, err
	}
	if exists && label != fingerprint {
		return nil, fmt.Errorf("warm Runtime container configuration drift detected")
	}
	if !exists {
		arguments := warmCreateArguments(config, lease.name, fingerprint)
		if output, err := lease.manager.docker(ctx, arguments...); err != nil {
			return nil, fmt.Errorf("create warm Runtime container: %w: %s", err, strings.TrimSpace(string(output)))
		}
	}
	// Release must attempt a stop even when Docker reports an ambiguous start
	// failure after the container process has already begun.
	lease.started = true
	if output, err := lease.manager.docker(ctx, "start", lease.name); err != nil {
		return nil, fmt.Errorf("start warm Runtime container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return warmRunProcess(lease.manager, config, lease.name), nil
}

func (lease *WarmLease) Release(ctx context.Context) error {
	if lease == nil || lease.manager == nil || lease.closed {
		return nil
	}
	lease.closed = true
	var err error
	if lease.started {
		err = lease.manager.stopIfRunning(ctx, lease.name)
	}
	lease.manager.mu.Unlock()
	return err
}

func (manager *WarmManager) Reap(ctx context.Context) (int, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	output, err := manager.docker(ctx, "ps", "--all", "--filter", "label=agent-platform.managed=true", "--filter", "label=agent-platform.warm=true", "--format", "{{.Names}}")
	if err != nil {
		return 0, fmt.Errorf("list warm Runtime containers: %w: %s", err, strings.TrimSpace(string(output)))
	}
	removed := 0
	for _, name := range strings.Fields(string(output)) {
		if !strings.HasPrefix(name, warmContainerPrefix) || !runIDPattern.MatchString(name) {
			continue
		}
		state, inspectErr := manager.docker(ctx, "inspect", "--format", "{{.State.Running}}|{{.State.FinishedAt}}", name)
		if inspectErr != nil {
			return removed, fmt.Errorf("inspect warm Runtime container %s: %w: %s", name, inspectErr, strings.TrimSpace(string(state)))
		}
		parts := strings.Split(strings.TrimSpace(string(state)), "|")
		if len(parts) != 2 || parts[0] == "true" {
			continue
		}
		finishedAt, parseErr := time.Parse(time.RFC3339Nano, parts[1])
		if parseErr != nil || manager.now().Sub(finishedAt) < manager.idleTimeout {
			continue
		}
		if removeOutput, removeErr := manager.docker(ctx, "rm", "--force", name); removeErr != nil && !isMissingContainer(removeOutput) {
			return removed, fmt.Errorf("remove idle Runtime container %s: %w: %s", name, removeErr, strings.TrimSpace(string(removeOutput)))
		}
		removed++
	}
	return removed, nil
}

func warmRunProcess(manager *WarmManager, config Config, name string) cliadapter.RunProcess {
	return func(ctx context.Context, spec processharness.Spec, sink processharness.OutputSink) (processharness.Result, error) {
		if spec.Dir == "" {
			spec.Dir = config.WorkspaceDirectory
		}
		if err := validateProcessSpec(config, spec); err != nil {
			return processharness.Result{}, err
		}
		hostSpec := spec
		hostSpec.Dir = ""
		hostSpec.Command = warmExecCommand(manager.dockerCommand, config, spec, name)
		if observer, ok := spec.Observer.(processharness.ProcessAwareObserver); ok {
			hostSpec.Observer = &containerAwareObserver{observer: observer, controller: &containerController{ctx: ctx, name: name, control: dockerControl(manager.dockerCommand)}}
		}
		return manager.runHost(ctx, hostSpec, sink)
	}
}

func warmExecCommand(dockerCommand string, config Config, spec processharness.Spec, name string) []string {
	arguments := []string{dockerCommand, "exec", "--interactive", "--workdir", config.ContainerWorkspace}
	for _, entry := range spec.Env {
		variable, _, _ := strings.Cut(entry, "=")
		arguments = append(arguments, "--env", variable)
	}
	if config.WorkflowPlan != "" && spec.Observer != nil {
		arguments = append(arguments, "--env", "AGENT_PLATFORM_WORKFLOW_B64="+config.WorkflowPlan)
	}
	arguments = append(arguments, name, "/usr/local/bin/runtime-entrypoint")
	return append(arguments, spec.Command...)
}

func warmCreateArguments(config Config, name, fingerprint string) []string {
	network := "none"
	if config.Egress == sandbox.EgressPublic {
		network = config.PublicEgressNetwork
	}
	arguments := []string{
		"create", "--name", name,
		"--runtime", config.Runtime,
		"--user", fmt.Sprintf("%d:%d", config.UID, config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", network,
		"--memory", strconv.FormatInt(config.Limits.MemoryBytes, 10),
		"--pids-limit", strconv.FormatInt(config.Limits.PIDs, 10),
		"--cpus", strconv.FormatFloat(config.Limits.CPUs, 'f', -1, 64),
		"--mount", "type=bind,src=" + config.WorkspaceDirectory + ",dst=" + config.ContainerWorkspace + ",readonly=false",
		"--mount", "type=bind,src=" + config.CredentialDirectory + ",dst=/run/agent-credentials,readonly=true",
	}
	if config.NativeStateDirectory != "" {
		arguments = append(arguments, "--mount", "type=bind,src="+config.NativeStateDirectory+",dst=/tmp/runtime-home/.codex,readonly=false")
	}
	if config.ScratchDirectory != "" {
		arguments = append(arguments, "--mount", "type=bind,src="+config.ScratchDirectory+",dst="+config.ScratchDirectory+",readonly=false")
	}
	if config.Egress == sandbox.EgressPublic {
		arguments = append(arguments, "--mount", "type=bind,src="+config.ResolverConfigFile+",dst=/etc/resolv.conf,readonly=true")
	}
	arguments = append(arguments,
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size="+strconv.FormatInt(config.Limits.TempBytes, 10),
		"--workdir", config.ContainerWorkspace,
		"--init",
		"--label", "agent-platform.managed=true",
		"--label", "agent-platform.warm=true",
		"--label", "agent-platform.warm-config="+fingerprint,
		"--entrypoint", "/bin/sh",
		config.Image,
		"-c", `trap 'exit 0' TERM INT; while :; do sleep 3600 & wait $!; done`,
	)
	return arguments
}

func warmFingerprint(name string, config Config) string {
	value := strings.Join([]string{
		name, config.Image, config.RuntimeCommand, config.Runtime,
		config.WorkspaceDirectory, config.ContainerWorkspace, config.CredentialDirectory,
		config.NativeStateDirectory, config.ScratchDirectory, config.PublicEgressNetwork,
		config.ResolverConfigFile, string(config.Egress),
		strconv.FormatFloat(config.Limits.CPUs, 'f', -1, 64), strconv.FormatInt(config.Limits.MemoryBytes, 10),
		strconv.FormatInt(config.Limits.PIDs, 10), strconv.FormatInt(config.Limits.TempBytes, 10),
		strconv.Itoa(config.UID), strconv.Itoa(config.GID),
	}, "\x00")
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func (manager *WarmManager) inspectFingerprint(ctx context.Context, name string) (bool, string, error) {
	output, err := manager.docker(ctx, "inspect", "--format", `{{index .Config.Labels "agent-platform.warm-config"}}`, name)
	if err != nil {
		if isMissingContainer(output) {
			return false, "", nil
		}
		return false, "", fmt.Errorf("inspect warm Runtime container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return true, strings.TrimSpace(string(output)), nil
}

func (manager *WarmManager) stopIfRunning(ctx context.Context, name string) error {
	output, err := manager.docker(ctx, "inspect", "--format", "{{.State.Running}}", name)
	if err != nil {
		if isMissingContainer(output) {
			return nil
		}
		return fmt.Errorf("inspect warm Runtime container state: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "true" {
		return nil
	}
	stopOutput, stopErr := manager.docker(ctx, "stop", "--time", "0", name)
	if stopErr != nil && !isMissingContainer(stopOutput) {
		return fmt.Errorf("stop warm Runtime container: %w: %s", stopErr, strings.TrimSpace(string(stopOutput)))
	}
	return nil
}

func isMissingContainer(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "no such container") || strings.Contains(message, "no such object")
}
