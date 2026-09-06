package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/workspacefs"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestGitCloneErrorsExposeOnlySafeActionableCategories(t *testing.T) {
	for _, test := range []struct{ message, code string }{
		{"Git clone requires an empty Workspace", "git_workspace_not_empty"},
		{"configured known_hosts is unavailable", "git_server_unavailable"},
		{"Host key verification failed.", "git_ssh_host_untrusted"},
		{"Load key: error in libcrypto", "git_ssh_key_invalid"},
		{"Permission denied (publickey).", "git_authentication_failed"},
		{"Authentication failed", "git_authentication_failed"},
		{"Remote branch missing not found in upstream origin", "git_branch_not_found"},
		{"Repository not found", "git_repository_unavailable"},
		{"Could not resolve hostname", "git_connection_failed"},
		{"cloned repository exceeds 1 GiB", "git_repository_too_large"},
		{"unexpected failure", "git_clone_failed"},
	} {
		t.Run(test.code, func(t *testing.T) {
			err := gitCloneError(errors.New(test.message + " credential-canary /private/path"))
			public := kratoserrors.FromError(err)
			if public.Reason != test.code || strings.Contains(err.Error(), "credential-canary") || strings.Contains(err.Error(), "/private/path") {
				t.Fatalf("unexpected public error: %v", err)
			}
		})
	}
}

type gitSourceRepository struct {
	application.Repository
	workflow domain.Workflow
}

func (repo *gitSourceRepository) GetWorkflow(context.Context, string, string, bool) (domain.Workflow, error) {
	return repo.workflow, nil
}

func TestConfigureGitSourceReportsNonemptyWorkspaceWithoutDeletingFiles(t *testing.T) {
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "owner"})
	files, err := workspacefs.New(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	const workspacePath = "workflows/owner/workflow"
	if _, err := files.Upload(ctx, workspacePath, "notes.txt", []byte("keep")); err != nil {
		t.Fatal(err)
	}
	app, err := application.New(&gitSourceRepository{workflow: domain.Workflow{ID: "workflow", WorkspacePath: workspacePath}})
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: app, files: files}
	_, err = service.ConfigureWorkflowGitSource(ctx, &workspacev1.ConfigureWorkflowGitSourceRequest{WorkflowId: "workflow", Url: "https://git.example.com/team/project.git", Branch: "main", Authentication: "none"})
	if public := kratoserrors.FromError(err); public == nil || public.Reason != "git_workspace_not_empty" {
		t.Fatalf("expected actionable empty Workspace error, got %v", err)
	}
	content, _, err := files.Read(ctx, workspacePath, "notes.txt")
	if err != nil || string(content) != "keep" {
		t.Fatalf("existing Workspace was changed: %v", err)
	}
}
