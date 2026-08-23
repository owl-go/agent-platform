package runtimeprocessor

import (
	"testing"

	"agent-platform/backend/internal/biz/execution/domain"
	"agent-platform/backend/internal/gitworkflow"
)

func TestProductionWorkflowPlanRequiresApprovalBeforeRuntime(t *testing.T) {
	encoded, err := encodeWorkflowPlan(domain.Lease{
		RunID: "run-1", RepositorySSHURL: "git@example.test:org/repo.git", TargetBranch: "main",
		ReviewBranch: "agent-platform/task", GitAuthorName: "Agent", GitAuthorEmail: "agent@example.test",
	}, []gitworkflow.QualityCommand{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := gitworkflow.DecodePlan(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequireApproval {
		t.Fatal("Production Runtime workflow can start without Run Approval")
	}
}
