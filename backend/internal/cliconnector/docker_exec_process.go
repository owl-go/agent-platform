package cliconnector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const runtimeConnectorRoot = "/opt/agent-platform/connectors"

var (
	containerName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type EgressGate interface {
	Execute(context.Context, string, []string, func(context.Context) (Result, error)) (Result, error)
}

type DockerExecRunner func(context.Context, []string, map[string]string) (Result, error)

type DockerExecProcessConfig struct {
	DockerCommand      string
	ContainerName      string
	ContainerWorkspace string
	Egress             EgressGate
	Run                DockerExecRunner
}

// DockerExecProcess starts a verified bundle only inside its owning Runtime
// container. EgressGate must establish the capability-specific network policy
// before the Docker command can run.
type DockerExecProcess struct {
	dockerCommand      string
	containerName      string
	containerWorkspace string
	egress             EgressGate
	run                DockerExecRunner
}

func NewDockerExecProcess(config DockerExecProcessConfig) (*DockerExecProcess, error) {
	if config.DockerCommand == "" {
		config.DockerCommand = "docker"
	}
	if strings.ContainsAny(config.DockerCommand, "\x00\r\n") || !containerName.MatchString(config.ContainerName) || !filepath.IsAbs(config.ContainerWorkspace) || config.ContainerWorkspace == "/" || config.Egress == nil {
		return nil, errors.New("invalid Docker CLI Connector process configuration")
	}
	if config.Run == nil {
		config.Run = runDockerExec
	}
	return &DockerExecProcess{dockerCommand: config.DockerCommand, containerName: config.ContainerName, containerWorkspace: config.ContainerWorkspace, egress: config.Egress, run: config.Run}, nil
}

func (process *DockerExecProcess) Run(ctx context.Context, request ProcessRequest) (Result, error) {
	if !definitionID.MatchString(request.ConnectorID) || request.Executable == "" || filepath.Base(request.Executable) != request.Executable || strings.ContainsAny(request.Executable, "\\/\x00\r\n") || len(request.Arguments) == 0 || len(request.EgressHosts) == 0 {
		return Result{}, errors.New("invalid CLI Connector process request")
	}
	for _, argument := range request.Arguments {
		if strings.ContainsRune(argument, '\x00') {
			return Result{}, errors.New("invalid CLI Connector argument")
		}
	}
	for name := range request.Environment {
		if !environmentName.MatchString(name) || name == "AGENT_PLATFORM_CLI_SOCKET" {
			return Result{}, errors.New("invalid CLI Connector environment")
		}
	}
	for _, host := range request.EgressHosts {
		if !egressHost.MatchString(host) || strings.Contains(host, "..") {
			return Result{}, errors.New("invalid CLI Connector Egress host")
		}
	}

	executable := filepath.Join(runtimeConnectorRoot, request.ConnectorID, "node_modules", ".bin", request.Executable)
	arguments := []string{process.dockerCommand, "exec", "--interactive", "--workdir", process.containerWorkspace}
	names := make([]string, 0, len(request.Environment))
	for name := range request.Environment {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		arguments = append(arguments, "--env", name)
	}
	arguments = append(arguments, process.containerName, executable)
	arguments = append(arguments, request.Arguments...)

	return process.egress.Execute(ctx, process.containerName, append([]string(nil), request.EgressHosts...), func(executionCtx context.Context) (Result, error) {
		return process.run(executionCtx, arguments, cloneEnvironment(request.Environment))
	})
}

func runDockerExec(ctx context.Context, arguments []string, environment map[string]string) (Result, error) {
	if len(arguments) == 0 {
		return Result{}, errors.New("Docker command is required")
	}
	command := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
	command.Env = mergeProcessEnvironment(os.Environ(), environment)
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
		return Result{}, fmt.Errorf("execute sandboxed CLI Connector: %w", err)
	}
	return result, nil
}

func mergeProcessEnvironment(base []string, overrides map[string]string) []string {
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
