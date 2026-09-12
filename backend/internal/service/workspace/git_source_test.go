package workspace

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
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
	workflow  domain.Workflow
	gitSecret []byte
}

func (repo *gitSourceRepository) GetWorkflow(context.Context, string, string, bool) (domain.Workflow, error) {
	return repo.workflow, nil
}

func (repo *gitSourceRepository) GetWorkflowGitSecret(context.Context, string, string) ([]byte, error) {
	return append([]byte(nil), repo.gitSecret...), nil
}

func (repo *gitSourceRepository) SetWorkflowGitSource(_ context.Context, _ string, _ string, source domain.GitSource, _ []byte) (domain.Workflow, error) {
	repo.workflow.GitSource = &source
	return repo.workflow, nil
}

func TestConfigureGitSourceReusesSavedSSHCredential(t *testing.T) {
	key := base64.RawStdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	box, err := secretcrypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte(`{"password":"","ssh_private_key":"saved-private-key"}`), "workflow-git:owner")
	if err != nil {
		t.Fatal(err)
	}
	repository := &gitSourceRepository{workflow: domain.Workflow{ID: "workflow", WorkspacePath: "workflows/owner/workflow", GitSource: &domain.GitSource{Authentication: "ssh", CredentialConfigured: true}}, gitSecret: ciphertext}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	var cloned workspacefs.GitCloneOptions
	service := &Service{accounts: &accountapplication.Service{}, workspace: app, box: box, cloneGitSource: func(_ context.Context, _ string, options workspacefs.GitCloneOptions) error {
		cloned = options
		cloned.PrivateKey = append([]byte(nil), options.PrivateKey...)
		return nil
	}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "owner"})
	_, err = service.ConfigureWorkflowGitSource(ctx, &workspacev1.ConfigureWorkflowGitSourceRequest{WorkflowId: "workflow", Url: "git@github.com:owl-go/agent-platform.git", Branch: "main", Authentication: "ssh"})
	if err != nil {
		t.Fatal(err)
	}
	if string(cloned.PrivateKey) != "saved-private-key" {
		t.Fatalf("private key = %q, want saved credential", cloned.PrivateKey)
	}
}

func TestLoadWorkflowGitCredentialsReusesEncryptedSecret(t *testing.T) {
	key := base64.RawStdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	box, err := secretcrypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := box.Encrypt([]byte(`{"password":"git-password","ssh_private_key":"private-key"}`), "workflow-git:owner")
	if err != nil {
		t.Fatal(err)
	}
	repository := &gitSourceRepository{gitSecret: ciphertext}
	app, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, box: box}

	password, privateKey, err := service.loadWorkflowGitCredentials(context.Background(), "owner", "workflow")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(password)
	defer clear(privateKey)
	if string(password) != "git-password" || string(privateKey) != "private-key" {
		t.Fatalf("credentials = %q/%q", password, privateKey)
	}
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
