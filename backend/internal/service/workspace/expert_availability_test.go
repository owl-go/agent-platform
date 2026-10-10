package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"errors"
	"testing"
)

type expertDependencyRepository struct {
	workspaceapplication.Repository
	dependencyError error
}

func (repo *expertDependencyRepository) ExpertDependenciesAvailable(context.Context, string, domain.Expert) error {
	return repo.dependencyError
}

func TestExpertAvailabilityPropagatesSystemFailures(t *testing.T) {
	storageFailure := errors.New("dependency storage unavailable")
	for _, dependencyError := range []error{storageFailure, context.Canceled, domain.ErrInvalid, domain.ErrConflict, domain.ErrNotFound} {
		t.Run(dependencyError.Error(), func(t *testing.T) {
			app, err := workspaceapplication.New(&expertDependencyRepository{dependencyError: dependencyError})
			if err != nil {
				t.Fatal(err)
			}
			service := &Service{workspace: app, accounts: &accountapplication.Service{}}
			ctx := accountapplication.WithPrincipal(t.Context(), accountdomain.Principal{UserID: "owner"})
			statuses, err := service.expertAvailability(ctx, []domain.Expert{{ID: "expert", Guidance: "# Guide", ConnectorDependencies: []domain.ExpertConnectorDependency{{Source: "tools", Kind: "mcp", Version: "1.0.0"}}}})
			if dependencyError == storageFailure || dependencyError == context.Canceled {
				if !errors.Is(err, dependencyError) {
					t.Fatalf("system failure was replaced by availability guidance: %v", err)
				}
			} else if err != nil || statuses["expert"].Available || statuses["expert"].Reason == "" {
				t.Fatalf("expected unavailable dependency status: %v %+v", err, statuses)
			}
		})
	}
}
