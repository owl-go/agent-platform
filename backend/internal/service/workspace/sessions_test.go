package workspace

import (
	"context"
	"errors"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
)

type deleteSessionRepository struct {
	workspaceapplication.Repository
	owner     string
	sessionID string
}

func (repository *deleteSessionRepository) DeleteSession(_ context.Context, owner, sessionID string) error {
	repository.owner = owner
	repository.sessionID = sessionID
	return nil
}

func TestDeleteSessionSucceedsWhenNativeStateCleanupFails(t *testing.T) {
	repository := &deleteSessionRepository{}
	workspace, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{
		accounts:  &accountapplication.Service{},
		workspace: workspace,
		removeNativeSessionState: func(root, owner, sessionID string) error {
			if root != "" || owner != "owner-1" || sessionID != "session-1" {
				t.Fatalf("cleanup scope = %q/%q/%q", root, owner, sessionID)
			}
			return errors.New("native state is temporarily unavailable")
		},
	}
	request := &workspacev1.DeleteSessionRequest{SessionId: "session-1"}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "owner-1"})

	response, err := service.DeleteSession(ctx, request)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}
	if !response.Deleted {
		t.Fatal("DeleteSession() returned Deleted=false")
	}
	if repository.owner != "owner-1" || repository.sessionID != "session-1" {
		t.Fatalf("DeleteSession() scope = %q/%q", repository.owner, repository.sessionID)
	}
}

var _ workspaceapplication.Repository = (*deleteSessionRepository)(nil)
