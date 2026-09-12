package workspace

import (
	"context"
	"encoding/base64"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"

	"golang.org/x/crypto/bcrypt"
)

type workflowCredentialRepository struct {
	workspaceapplication.Repository
	key        string
	hash       string
	ciphertext []byte
}

func (repository *workflowCredentialRepository) SetWorkflowCredential(_ context.Context, _ string, _ string, key, hash string, ciphertext []byte) (workspacedomain.Workflow, error) {
	repository.key, repository.hash, repository.ciphertext = key, hash, append([]byte(nil), ciphertext...)
	return workspacedomain.Workflow{}, nil
}

func (repository *workflowCredentialRepository) GetWorkflowCredential(context.Context, string, string) (string, []byte, error) {
	return repository.key, append([]byte(nil), repository.ciphertext...), nil
}

func TestWorkflowCredentialCanBeViewedAfterGeneration(t *testing.T) {
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString([]byte("01234567890123456789012345678901")))
	if err != nil {
		t.Fatal(err)
	}
	repository := &workflowCredentialRepository{}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{accounts: &accountapplication.Service{}, workspace: application, box: box}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "owner-1"})

	created, err := service.GenerateWorkflowCredential(ctx, &workspacev1.GenerateWorkflowCredentialRequest{WorkflowId: "workflow-1"})
	if err != nil {
		t.Fatal(err)
	}
	if repository.key != created.ApiKey || len(repository.ciphertext) == 0 {
		t.Fatal("generated credential was not persisted with ciphertext")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repository.hash), []byte(created.ApiSecret)); err != nil {
		t.Fatalf("generated Secret hash does not verify: %v", err)
	}

	viewed, err := service.GetWorkflowCredential(ctx, &workspacev1.GetWorkflowCredentialRequest{WorkflowId: "workflow-1"})
	if err != nil {
		t.Fatal(err)
	}
	if viewed.ApiKey != created.ApiKey || viewed.ApiSecret != created.ApiSecret {
		t.Fatalf("viewed credential = %#v, created = %#v", viewed, created)
	}
}
