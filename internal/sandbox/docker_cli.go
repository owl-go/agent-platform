package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

type CLIExecutor struct {
	Binary string
}

func (e CLIExecutor) Run(ctx context.Context, args ...string) (string, error) {
	binary := e.Binary
	if binary == "" {
		binary = "docker"
	}
	command := exec.CommandContext(ctx, binary, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		exitCode := -1
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		}
		return stdout.String(), &CommandError{ExitCode: exitCode, Stderr: stderr.String(), Cause: err}
	}
	return stdout.String(), nil
}
