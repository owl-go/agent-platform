package gitworkflow

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowClonesValidatesCommitsAndPushesReviewBranch(t *testing.T) {
	remote := seedRemote(t)
	workspace, credentials := emptyDirectory(t), emptyDirectory(t)
	plan := Plan{
		RunID: "run-1", RepositoryURL: remote, TargetBranch: "main", ReviewBranch: "agent-platform/backend/task-1",
		GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test",
		QualityCommands: []QualityCommand{{Name: "verify result", Kind: "test", Executable: "sh", Arguments: []string{"-c", "test \"$(cat result.txt)\" = changed"}, TimeoutSeconds: 10}},
	}
	workflow := Workflow{Plan: plan, Workspace: workspace, CredentialRoot: credentials, Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}
	if err := workflow.Execute(context.Background(), []string{"sh", "-c", "printf changed > result.txt"}); err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(t.TempDir(), "checkout")
	runGit(t, "", "clone", "--branch", plan.ReviewBranch, remote, checkout)
	contents, err := os.ReadFile(filepath.Join(checkout, "result.txt"))
	if err != nil || string(contents) != "changed" {
		t.Fatalf("pushed result = %q, %v", contents, err)
	}
	message := runGit(t, checkout, "log", "-1", "--pretty=%B")
	if !strings.Contains(message, "Agent-Platform-Run: run-1") {
		t.Fatalf("commit message = %q", message)
	}
}

func TestWorkflowRejectsCredentialInStagedDiff(t *testing.T) {
	remote := seedRemote(t)
	workspace, credentials := emptyDirectory(t), emptyDirectory(t)
	secret := "do-not-commit-this-secret"
	if err := os.WriteFile(filepath.Join(credentials, "token"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	plan := Plan{RunID: "run-2", RepositoryURL: remote, TargetBranch: "main", ReviewBranch: "agent-platform/backend/task-2", GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test"}
	err := (Workflow{Plan: plan, Workspace: workspace, CredentialRoot: credentials, Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}).
		Execute(context.Background(), []string{"sh", "-c", "printf '" + secret + "' > leaked.txt"})
	if err == nil || !strings.Contains(err.Error(), "Secret scan rejected") {
		t.Fatalf("error = %v", err)
	}
	if output, err := exec.Command("git", "--git-dir", remote, "show-ref", "--verify", "refs/heads/"+plan.ReviewBranch).CombinedOutput(); err == nil {
		t.Fatalf("rejected branch was pushed: %s", output)
	}
}

func TestWorkflowRequiresApprovalBeforeStartingRuntime(t *testing.T) {
	remote := seedRemote(t)
	workspace, credentials := emptyDirectory(t), emptyDirectory(t)
	approved := false
	plan := Plan{
		RunID: "run-approval", RepositoryURL: remote, TargetBranch: "main", ReviewBranch: "agent-platform/approval",
		GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test", RequireApproval: true,
	}
	workflow := Workflow{
		Plan: plan, Workspace: workspace, CredentialRoot: credentials, Stdout: &strings.Builder{}, Stderr: &strings.Builder{},
		ApprovalGate: func(_ context.Context, _ io.Writer, received Plan) error {
			if received.ReviewBranch != plan.ReviewBranch {
				t.Fatalf("Approval plan = %+v", received)
			}
			approved = true
			return nil
		},
	}
	if err := workflow.Execute(context.Background(), []string{"sh", "-c", "test \"$APPROVED\" = yes && printf changed > result.txt"}); err == nil {
		t.Fatal("runtime unexpectedly inherited test-only approval state")
	}
	if !approved {
		t.Fatal("Runtime started without invoking Approval Gate")
	}

	approved = false
	workflow.ApprovalGate = func(context.Context, io.Writer, Plan) error { approved = true; return nil }
	if err := os.Setenv("APPROVED", "yes"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("APPROVED") })
	workspace = emptyDirectory(t)
	workflow.Workspace = workspace
	if err := workflow.Execute(context.Background(), []string{"sh", "-c", "test \"$APPROVED\" = yes && printf changed > result.txt"}); err != nil {
		t.Fatal(err)
	}
	if !approved {
		t.Fatal("Approval Gate was not invoked")
	}
}

func TestWorkflowFailsClosedWithoutRequiredApprovalGate(t *testing.T) {
	remote := seedRemote(t)
	plan := Plan{
		RunID: "run-approval", RepositoryURL: remote, TargetBranch: "main", ReviewBranch: "agent-platform/approval-missing",
		GitAuthorName: "Agent Platform", GitAuthorEmail: "agent@example.test", RequireApproval: true,
	}
	err := (Workflow{Plan: plan, Workspace: emptyDirectory(t), CredentialRoot: emptyDirectory(t), Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}).
		Execute(context.Background(), []string{"sh", "-c", "printf unsafe > result.txt"})
	if err == nil || !strings.Contains(err.Error(), "Approval Gate is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodePlanIsStrict(t *testing.T) {
	encoded, err := json.Marshal(Plan{RunID: "run", RepositoryURL: "git@example.test:org/repo.git", TargetBranch: "main", ReviewBranch: "review", GitAuthorName: "Agent", GitAuthorEmail: "agent@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePlan(base64.RawURLEncoding.EncodeToString(encoded)); err != nil {
		t.Fatal(err)
	}
	withUnknown := base64.RawURLEncoding.EncodeToString([]byte(`{"run_id":"run","unknown":true}`))
	if _, err := DecodePlan(withUnknown); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

func seedRemote(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	source, remote := filepath.Join(root, "source"), filepath.Join(root, "remote.git")
	if err := os.Mkdir(source, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, source, "init", "--initial-branch", "main")
	runGit(t, source, "config", "user.name", "Fixture")
	runGit(t, source, "config", "user.email", "fixture@example.test")
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, source, "add", "README.md")
	runGit(t, source, "commit", "-m", "initial")
	runGit(t, "", "init", "--bare", remote)
	runGit(t, source, "remote", "add", "origin", remote)
	runGit(t, source, "push", "origin", "main")
	return remote
}

func emptyDirectory(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func runGit(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", arguments, output, err)
	}
	return string(output)
}
