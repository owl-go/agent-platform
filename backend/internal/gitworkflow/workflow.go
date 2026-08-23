package gitworkflow

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"agent-platform/backend/internal/agentruntime/platformprotocol"
)

type QualityCommand struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Executable     string   `json:"executable"`
	Arguments      []string `json:"arguments"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type Plan struct {
	RunID           string           `json:"run_id"`
	RepositoryURL   string           `json:"repository_url"`
	TargetBranch    string           `json:"target_branch"`
	ReviewBranch    string           `json:"review_branch"`
	GitAuthorName   string           `json:"git_author_name"`
	GitAuthorEmail  string           `json:"git_author_email"`
	QualityCommands []QualityCommand `json:"quality_commands"`
	RequireApproval bool             `json:"require_approval"`
}

type ApprovalGate func(context.Context, io.Writer, Plan) error

type Workflow struct {
	Plan           Plan
	Workspace      string
	CredentialRoot string
	Stdout         io.Writer
	Stderr         io.Writer
	ApprovalGate   ApprovalGate
}

func DecodePlan(value string) (Plan, error) {
	encoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Plan{}, fmt.Errorf("decode workflow plan: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var plan Plan
	if err := decoder.Decode(&plan); err != nil {
		return Plan{}, fmt.Errorf("decode workflow plan: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Plan{}, fmt.Errorf("decode workflow plan: multiple JSON values")
	}
	return plan, validatePlan(plan)
}

func (workflow Workflow) Execute(ctx context.Context, runtimeCommand []string) error {
	if err := validatePlan(workflow.Plan); err != nil {
		return err
	}
	if len(runtimeCommand) == 0 || strings.ContainsAny(runtimeCommand[0], "/\\\x00") {
		return fmt.Errorf("runtime command must be a bare executable")
	}
	if !filepath.IsAbs(workflow.Workspace) || workflow.Workspace == "/" || !filepath.IsAbs(workflow.CredentialRoot) {
		return fmt.Errorf("workflow paths must be absolute and workspace cannot be root")
	}
	if workflow.Stdout == nil {
		workflow.Stdout = os.Stdout
	}
	if workflow.Stderr == nil {
		workflow.Stderr = os.Stderr
	}
	if err := workflow.prepare(ctx); err != nil {
		return err
	}
	if workflow.Plan.RequireApproval {
		if workflow.ApprovalGate == nil {
			return fmt.Errorf("Run Approval Gate is required")
		}
		if err := workflow.ApprovalGate(ctx, workflow.Stdout, workflow.Plan); err != nil {
			return fmt.Errorf("wait for Run Approval: %w", err)
		}
	}
	if err := workflow.run(ctx, runtimeCommand[0], runtimeCommand[1:], 0); err != nil {
		return fmt.Errorf("Agent Runtime failed: %w", err)
	}
	for _, command := range workflow.Plan.QualityCommands {
		if _, err := fmt.Fprintf(workflow.Stdout, "quality gate: %s\n", command.Name); err != nil {
			return err
		}
		if err := workflow.run(ctx, command.Executable, command.Arguments, time.Duration(command.TimeoutSeconds)*time.Second); err != nil {
			return fmt.Errorf("quality gate %q failed: %w", command.Name, err)
		}
	}
	return workflow.deliver(ctx)
}

func PauseForApproval(ctx context.Context, output io.Writer, plan Plan) error {
	line, err := platformprotocol.EncodeApprovalRequest("Coding Agent execution and Review Branch delivery require approval", plan.ReviewBranch)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
		return err
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGSTOP); err != nil {
		return fmt.Errorf("pause Runtime Workflow for Approval: %w", err)
	}
	return ctx.Err()
}

func (workflow Workflow) prepare(ctx context.Context) error {
	gitDirectory := filepath.Join(workflow.Workspace, ".git")
	if _, err := os.Stat(gitDirectory); errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(workflow.Workspace)
		if readErr != nil {
			return fmt.Errorf("inspect Code Workspace: %w", readErr)
		}
		if len(entries) != 0 {
			return fmt.Errorf("uninitialized Code Workspace is not empty")
		}
		if err := workflow.git(ctx, "clone", "--no-tags", "--single-branch", "--branch", workflow.Plan.TargetBranch, "--", workflow.Plan.RepositoryURL, "."); err != nil {
			return fmt.Errorf("clone Repository Binding: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("inspect Code Workspace: %w", err)
	}
	remote, err := workflow.gitOutput(ctx, "remote", "get-url", "origin")
	if err != nil || strings.TrimSpace(remote) != workflow.Plan.RepositoryURL {
		return fmt.Errorf("Code Workspace origin does not match Repository Binding")
	}
	if err := workflow.git(ctx, "check-ref-format", "refs/heads/"+workflow.Plan.TargetBranch); err != nil {
		return fmt.Errorf("invalid target branch")
	}
	if err := workflow.git(ctx, "check-ref-format", "refs/heads/"+workflow.Plan.ReviewBranch); err != nil {
		return fmt.Errorf("invalid Review Branch")
	}
	if err := workflow.git(ctx, "fetch", "--no-tags", "origin", workflow.Plan.TargetBranch); err != nil {
		return fmt.Errorf("fetch target branch: %w", err)
	}
	if workflow.git(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+workflow.Plan.ReviewBranch) == nil {
		if err := workflow.git(ctx, "checkout", workflow.Plan.ReviewBranch); err != nil {
			return err
		}
	} else if workflow.git(ctx, "fetch", "--no-tags", "origin", workflow.Plan.ReviewBranch) == nil {
		if err := workflow.git(ctx, "checkout", "-B", workflow.Plan.ReviewBranch, "origin/"+workflow.Plan.ReviewBranch); err != nil {
			return err
		}
	} else if err := workflow.git(ctx, "checkout", "-B", workflow.Plan.ReviewBranch, "origin/"+workflow.Plan.TargetBranch); err != nil {
		return fmt.Errorf("create Review Branch: %w", err)
	}
	if err := workflow.git(ctx, "config", "user.name", workflow.Plan.GitAuthorName); err != nil {
		return err
	}
	if err := workflow.git(ctx, "config", "user.email", workflow.Plan.GitAuthorEmail); err != nil {
		return err
	}
	return nil
}

func (workflow Workflow) deliver(ctx context.Context) error {
	if err := workflow.git(ctx, "add", "--all"); err != nil {
		return fmt.Errorf("stage Agent changes: %w", err)
	}
	diff, err := workflow.gitOutput(ctx, "diff", "--cached", "--binary")
	if err != nil {
		return fmt.Errorf("inspect staged Agent changes: %w", err)
	}
	if err := scanSecrets([]byte(diff), workflow.CredentialRoot); err != nil {
		return err
	}
	if strings.TrimSpace(diff) != "" {
		message := "agent-platform: complete Run " + workflow.Plan.RunID
		if err := workflow.git(ctx, "commit", "--message", message, "--message", "Agent-Platform-Run: "+workflow.Plan.RunID); err != nil {
			return fmt.Errorf("commit Agent changes: %w", err)
		}
	}
	refspec := "HEAD:refs/heads/" + workflow.Plan.ReviewBranch
	if err := workflow.git(ctx, "push", "origin", refspec); err != nil {
		return fmt.Errorf("push Review Branch: %w", err)
	}
	commit, err := workflow.gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("read delivered commit: %w", err)
	}
	changed, err := workflow.gitOutput(ctx, "diff", "--name-only", "-z", "origin/"+workflow.Plan.TargetBranch+"...HEAD")
	if err != nil {
		return fmt.Errorf("list delivered files: %w", err)
	}
	files := make([]string, 0)
	for _, name := range strings.Split(changed, "\x00") {
		if name != "" {
			files = append(files, name)
		}
	}
	report, err := json.Marshal(map[string]any{
		"event": "workflow.delivered", "review_branch": workflow.Plan.ReviewBranch,
		"commit": strings.TrimSpace(commit), "changed_files": files,
	})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(workflow.Stdout, "agent-platform-workflow: %s\n", report); err != nil {
		return err
	}
	return nil
}

func (workflow Workflow) git(ctx context.Context, arguments ...string) error {
	_, err := workflow.command(ctx, 0, false, "git", arguments...)
	return err
}

func (workflow Workflow) gitOutput(ctx context.Context, arguments ...string) (string, error) {
	return workflow.command(ctx, 0, false, "git", arguments...)
}

func (workflow Workflow) run(ctx context.Context, executable string, arguments []string, timeout time.Duration) error {
	_, err := workflow.command(ctx, timeout, true, executable, arguments...)
	return err
}

func (workflow Workflow) command(ctx context.Context, timeout time.Duration, stream bool, executable string, arguments ...string) (string, error) {
	commandCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	command := exec.CommandContext(commandCtx, executable, arguments...)
	command.Dir = workflow.Workspace
	command.Env = os.Environ()
	var captured bytes.Buffer
	command.Stdout = &captured
	if stream {
		command.Stdout = io.MultiWriter(workflow.Stdout, &captured)
	}
	command.Stderr = workflow.Stderr
	if err := command.Run(); err != nil {
		if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
			return captured.String(), fmt.Errorf("command timed out")
		}
		return captured.String(), fmt.Errorf("command exited unsuccessfully")
	}
	return captured.String(), nil
}

func scanSecrets(diff []byte, credentialRoot string) error {
	if bytes.Contains(diff, []byte("-----BEGIN ")) && bytes.Contains(diff, []byte("PRIVATE KEY-----")) {
		return fmt.Errorf("Secret scan rejected staged changes")
	}
	err := filepath.WalkDir(credentialRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("credential root contains a special file")
		}
		secret, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		secret = bytes.TrimSpace(secret)
		if len(secret) >= 4 && bytes.Contains(diff, secret) {
			return fmt.Errorf("Secret scan rejected staged changes")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan staged changes: %w", err)
	}
	return nil
}

func validatePlan(plan Plan) error {
	if plan.RunID == "" || plan.RepositoryURL == "" || plan.TargetBranch == "" || plan.ReviewBranch == "" || plan.TargetBranch == plan.ReviewBranch || plan.GitAuthorName == "" || plan.GitAuthorEmail == "" {
		return fmt.Errorf("Git workflow plan is incomplete")
	}
	for _, command := range plan.QualityCommands {
		if command.Name == "" || !validQualityKind(command.Kind) || command.Executable == "" || strings.ContainsAny(command.Executable, "\x00") || command.TimeoutSeconds <= 0 || command.TimeoutSeconds > 3600 {
			return fmt.Errorf("Git workflow quality command is invalid")
		}
	}
	return nil
}

func validQualityKind(value string) bool {
	switch value {
	case "build", "format", "lint", "test":
		return true
	default:
		return false
	}
}
