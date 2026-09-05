package workspace

import (
	"context"
	"net/http"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestAdministratorCannotDecideCommandApproval(t *testing.T) {
	service := &Service{accounts: &accountapplication.Service{}}
	ctx := accountapplication.WithPrincipal(context.Background(), accountdomain.Principal{UserID: "administrator-1", Administrator: true})
	_, err := service.DecideCommandApproval(ctx, &workspacev1.DecideCommandApprovalRequest{ApprovalId: "approval-1", Decision: "approved", ExpectedVersion: 1})
	if code := kratoserrors.Code(err); code != http.StatusForbidden {
		t.Fatalf("Administrator approval code = %d, want %d", code, http.StatusForbidden)
	}
}
