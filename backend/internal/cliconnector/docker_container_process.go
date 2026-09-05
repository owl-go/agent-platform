package cliconnector

import (
	"bytes"
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

	"agent-platform/backend/internal/sandbox"
)

const connectorContainerRoot = "/opt/agent-platform/connector"

var (
	containerName        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	environmentName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	containerImageDigest = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
)

type EgressGate interface {
	Execute(context.Context, string, []string, func(context.Context) (Result, error)) (Result, error)
}

type DockerCommandRunner func(context.Context, map[string]string, string, ...string) ([]byte, error)
type DockerStartRunner func(context.Context, []string) (Result, error)
type ContainerNameFactory func() (string, error)

type DockerContainerProcessConfig struct {
	DockerCommand      string
	Image              string
	Runtime            string
	RunID              string
	BundleDirectory    string
	WorkspaceDirectory string
	ContainerWorkspace string
	ResolverConfigFile string
	EgressNetwork      string
	Limits             sandbox.Limits
	UID                int
	GID                int
	Egress             EgressGate
	Run                DockerCommandRunner
	Start              DockerStartRunner
	Name               ContainerNameFactory
}

// DockerContainerProcess runs one reviewed command in a dedicated container.
// The container remains stopped until its capability-specific Egress policy is installed.
type DockerContainerProcess struct {
	config DockerContainerProcessConfig
}

func NewDockerContainerProcess(config DockerContainerProcessConfig) (*DockerContainerProcess, error) {
	if config.DockerCommand == "" {
		config.DockerCommand = "docker"
	}
	if config.ContainerWorkspace == "" {
		config.ContainerWorkspace = "/workspace"
	}
	if config.Run == nil {
		config.Run = runDockerCommand
	}
	if config.Start == nil {
		config.Start = runDockerStart
	}
	if config.Name == nil {
		config.Name = randomConnectorContainerName
	}
	if err := validateDockerContainerConfig(config); err != nil {
		return nil, err
	}
	return &DockerContainerProcess{config: config}, nil
}

func (process *DockerContainerProcess) Run(ctx context.Context, request ProcessRequest) (result Result, returnErr error) {
	if err := validateProcessRequest(request); err != nil {
		return Result{}, err
	}
	name, err := process.config.Name()
	if err != nil || !containerName.MatchString(name) {
		return Result{}, errors.New("create CLI Connector container identity")
	}

	create := process.createCommand(name, request)
	if output, err := process.config.Run(ctx, request.Environment, create[0], create[1:]...); err != nil {
		return Result{}, fmt.Errorf("create CLI Connector container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	removed := false
	defer func() {
		if removed {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := process.remove(cleanupCtx, name); err != nil {
			returnErr = errors.Join(returnErr, err)
		}
	}()

	if output, err := process.config.Run(ctx, nil, process.config.DockerCommand, "network", "connect", process.config.EgressNetwork, name); err != nil {
		return Result{}, fmt.Errorf("connect CLI Connector Egress network: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return process.config.Egress.Execute(ctx, name, append([]string(nil), request.EgressHosts...), func(executionCtx context.Context) (Result, error) {
		result, startErr := process.config.Start(executionCtx, []string{process.config.DockerCommand, "start", "--attach", "--interactive", name})
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(executionCtx), 15*time.Second)
		defer cancel()
		removeErr := process.remove(cleanupCtx, name)
		if removeErr != nil {
			return result, errors.Join(startErr, fmt.Errorf("%w: %v", ErrEgressSubjectActive, removeErr))
		}
		removed = true
		return result, startErr
	})
}

func (process *DockerContainerProcess) remove(ctx context.Context, name string) error {
	output, err := process.config.Run(ctx, nil, process.config.DockerCommand, "rm", "--force", name)
	if err != nil {
		return fmt.Errorf("remove CLI Connector container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func validateDockerContainerConfig(config DockerContainerProcessConfig) error {
	paths := []string{config.BundleDirectory, config.WorkspaceDirectory, config.ContainerWorkspace, config.ResolverConfigFile}
	for _, value := range paths {
		if !filepath.IsAbs(value) || value == "/" || strings.Contains(value, ",") {
			return errors.New("CLI Connector container paths must be absolute non-root paths without commas")
		}
	}
	if strings.ContainsAny(config.DockerCommand, "\x00\r\n") || !containerImageDigest.MatchString(config.Image) || config.Runtime != "runsc" || !containerName.MatchString(config.RunID) || !dockerNetworkName.MatchString(config.EgressNetwork) || config.UID <= 0 || config.GID <= 0 || config.Egress == nil {
		return errors.New("invalid Docker CLI Connector container configuration")
	}
	if config.Limits.CPUs <= 0 || config.Limits.MemoryBytes <= 0 || config.Limits.PIDs <= 0 || config.Limits.TempBytes <= 0 {
		return errors.New("positive CLI Connector container limits are required")
	}
	return nil
}

func validateProcessRequest(request ProcessRequest) error {
	if !definitionID.MatchString(request.ConnectorID) || request.Executable == "" || filepath.Base(request.Executable) != request.Executable || strings.ContainsAny(request.Executable, "\\/\x00\r\n") || len(request.Arguments) == 0 || len(request.EgressHosts) == 0 {
		return errors.New("invalid CLI Connector process request")
	}
	for _, argument := range request.Arguments {
		if strings.ContainsRune(argument, '\x00') {
			return errors.New("invalid CLI Connector argument")
		}
	}
	for name := range request.Environment {
		if !environmentName.MatchString(name) || name == "AGENT_PLATFORM_CLI_SOCKET" {
			return errors.New("invalid CLI Connector environment")
		}
		if strings.ContainsRune(request.Environment[name], '\x00') {
			return errors.New("invalid CLI Connector environment value")
		}
	}
	for _, host := range request.EgressHosts {
		if !egressHost.MatchString(host) || strings.Contains(host, "..") {
			return errors.New("invalid CLI Connector Egress host")
		}
	}
	return nil
}

func (process *DockerContainerProcess) createCommand(name string, request ProcessRequest) []string {
	bundle := filepath.Join(process.config.BundleDirectory, request.ConnectorID)
	executable := filepath.Join(connectorContainerRoot, "node_modules", ".bin", request.Executable)
	arguments := []string{
		process.config.DockerCommand, "create", "--name", name,
		"--runtime", process.config.Runtime,
		"--user", fmt.Sprintf("%d:%d", process.config.UID, process.config.GID),
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--network", "none",
		"--memory", strconv.FormatInt(process.config.Limits.MemoryBytes, 10),
		"--pids-limit", strconv.FormatInt(process.config.Limits.PIDs, 10),
		"--cpus", strconv.FormatFloat(process.config.Limits.CPUs, 'f', -1, 64),
		"--mount", "type=bind,src=" + process.config.WorkspaceDirectory + ",dst=" + process.config.ContainerWorkspace + ",readonly=false",
		"--mount", "type=bind,src=" + bundle + ",dst=" + connectorContainerRoot + ",readonly=true",
		"--mount", "type=bind,src=" + process.config.ResolverConfigFile + ",dst=/etc/resolv.conf,readonly=true",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=" + strconv.FormatInt(process.config.Limits.TempBytes, 10),
		"--workdir", process.config.ContainerWorkspace,
		"--entrypoint", "/usr/local/bin/runtime-entrypoint",
		"--label", "agent-platform.managed=true",
		"--label", "agent-platform.run-id=" + process.config.RunID,
		"--label", "agent-platform.role=cli-connector",
		"--init",
	}
	names := make([]string, 0, len(request.Environment))
	for name := range request.Environment {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		arguments = append(arguments, "--env", name)
	}
	arguments = append(arguments, process.config.Image, executable)
	return append(arguments, request.Arguments...)
}

func runDockerCommand(ctx context.Context, environment map[string]string, command string, arguments ...string) ([]byte, error) {
	process := exec.CommandContext(ctx, command, arguments...)
	process.Env = processEnvironment(os.Environ(), environment)
	return process.CombinedOutput()
}

func runDockerStart(ctx context.Context, arguments []string) (Result, error) {
	if len(arguments) == 0 {
		return Result{}, errors.New("Docker command is required")
	}
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		result.ExitCode = exitError.ExitCode()
		return result, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("start CLI Connector container: %w", err)
	}
	return result, nil
}

func randomConnectorContainerName() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "agent-cli-" + hex.EncodeToString(value[:]), nil
}

func processEnvironment(base []string, overrides map[string]string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[name]; !replaced {
			result = append(result, entry)
		}
	}
	for name, value := range overrides {
		result = append(result, name+"="+value)
	}
	return result
}
