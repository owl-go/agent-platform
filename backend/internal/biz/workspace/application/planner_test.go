package application

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestPlanExecutionBuildsAnonymousExpertAndTeamPlans(t *testing.T) {
	stage := func(position int, expertID string) domain.ExecutionStageSnapshot {
		value := domain.ExecutionStageSnapshot{Position: position, RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ID: "model-1"}}
		if expertID != "" {
			value.Expert = &domain.ExpertSnapshot{ID: expertID, Version: 1}
		}
		return value
	}
	tests := []struct {
		name      string
		selection ExecutionSelection
		want      int
	}{
		{name: "anonymous", selection: ExecutionSelection{Anonymous: pointerTo(stage(1, ""))}, want: 1},
		{name: "Expert", selection: ExecutionSelection{Expert: pointerTo(stage(1, "expert-1"))}, want: 1},
		{name: "Expert Team", selection: ExecutionSelection{Team: []domain.ExecutionStageSnapshot{stage(1, "expert-1"), stage(2, "expert-2")}}, want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanExecution(domain.ExecutionSnapshot{WorkflowName: "work"}, test.selection)
			if err != nil {
				t.Fatal(err)
			}
			if plan.SchemaVersion != 2 || len(plan.Stages) != test.want {
				t.Fatalf("plan = %#v", plan)
			}
		})
	}
}

func TestPlanExecutionRequiresOneSharedTeamExecutionConfiguration(t *testing.T) {
	selection := ExecutionSelection{Team: []domain.ExecutionStageSnapshot{
		{Position: 1, Expert: &domain.ExpertSnapshot{ID: "expert-a"}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ID: "model-a"}},
		{Position: 2, Expert: &domain.ExpertSnapshot{ID: "expert-b"}, RuntimeEngine: domain.RuntimeClaude, ProviderModel: domain.ProviderModelSnapshot{ID: "model-b"}},
	}}
	if _, err := PlanExecution(domain.ExecutionSnapshot{}, selection); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("mixed team execution configuration error = %v, want ErrInvalid", err)
	}
}

func TestPlanExecutionAllowsSameExpertForDistinctStableMembers(t *testing.T) {
	model := domain.ProviderModelSnapshot{ID: "model-1"}
	stages := []domain.ExecutionStageSnapshot{
		{Position: 1, TeamMemberID: "builder", Expert: &domain.ExpertSnapshot{ID: "expert-1"}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: model},
		{Position: 2, TeamMemberID: "reviewer", Expert: &domain.ExpertSnapshot{ID: "expert-1"}, RuntimeEngine: domain.RuntimeCodex, ProviderModel: model},
	}
	if _, err := PlanExecution(domain.ExecutionSnapshot{}, ExecutionSelection{Team: stages}); err != nil {
		t.Fatalf("plan distinct roles with same Expert: %v", err)
	}
}

func TestPlanExecutionRejectsAmbiguousOrInvalidSelections(t *testing.T) {
	stage := domain.ExecutionStageSnapshot{Position: 1, RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ID: "model-1"}}
	for _, selection := range []ExecutionSelection{
		{},
		{Anonymous: &stage, Expert: &stage},
		{Team: []domain.ExecutionStageSnapshot{stage}},
	} {
		if _, err := PlanExecution(domain.ExecutionSnapshot{}, selection); err == nil {
			t.Fatalf("selection %#v was accepted", selection)
		}
	}
}

func pointerTo[T any](value T) *T { return &value }
