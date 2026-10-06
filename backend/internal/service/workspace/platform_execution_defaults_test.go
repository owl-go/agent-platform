package workspace

import (
	"context"
	"net/http"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/platformconfig"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

type executionDefaultRepository struct {
	workspaceapplication.Repository
	calls int
}

func (r *executionDefaultRepository) GetPlatformExecutionDefault(context.Context) (domain.PlatformExecutionDefault, error) {
	return domain.PlatformExecutionDefault{}, domain.ErrNotFound
}
func (r *executionDefaultRepository) SetPlatformExecutionDefault(_ context.Context, admin string, runtime domain.RuntimeEngine, model string, version int64) (domain.PlatformExecutionDefault, error) {
	r.calls++
	return domain.PlatformExecutionDefault{RuntimeEngine: runtime, ProviderModelID: model, UpdatedBy: admin, Version: version + 1}, nil
}
func TestPlatformExecutionDefaultServiceDoesNotRequireRunAndKeepsAccessChecks(t *testing.T) {
	repo := &executionDefaultRepository{}
	app, err := workspaceapplication.New(repo)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: app, accounts: &accountapplication.Service{}, config: platformconfig.Config{Worker: platformconfig.WorkerConfig{Runtimes: map[string]platformconfig.RuntimeEngineConfig{"codex": {Available: true}}}}}
	admin := accountapplication.WithPrincipal(t.Context(), accountdomain.Principal{UserID: "admin", Administrator: true})
	for _, legacyProof := range []string{"", "ignored-legacy-run"} {
		response, err := service.SetPlatformExecutionDefault(admin, &workspacev1.SetPlatformExecutionDefaultRequest{RuntimeEngine: "codex", ProviderModelId: "model", ValidationRunId: legacyProof, ExpectedVersion: 2})
		if err != nil || response.ValidationRunId != "" || response.Version != 3 || response.ProviderModelId != "model" {
			t.Fatal("configuration save required a validation Run", err)
		}
	}
	user := accountapplication.WithPrincipal(t.Context(), accountdomain.Principal{UserID: "user"})
	if _, err := service.SetPlatformExecutionDefault(user, &workspacev1.SetPlatformExecutionDefaultRequest{RuntimeEngine: "codex", ProviderModelId: "model"}); kratoserrors.Code(err) != http.StatusForbidden {
		t.Fatal("ordinary User changed default", err)
	}
	if _, err := service.SetPlatformExecutionDefault(admin, &workspacev1.SetPlatformExecutionDefaultRequest{RuntimeEngine: "claude", ProviderModelId: "model"}); kratoserrors.Code(err) != http.StatusUnprocessableEntity {
		t.Fatal("unavailable Runtime accepted", err)
	}
	if repo.calls != 2 {
		t.Fatal("rejected request reached persistence")
	}
}
