package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildExecutionPlanConditionalRules(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	stage := ExecutionStageSnapshot{Position: 1, CreditRate: &CreditRateSnapshot{FallbackHundredths: 125}}

	plain, err := BuildExecutionPlan(ExecutionPlanContext{Objective: "Answer a question", Stages: []ExecutionStageSnapshot{stage}}, now)
	if err != nil || plain != nil {
		t.Fatalf("plain question plan = %#v, err = %v", plain, err)
	}

	explicit, err := BuildExecutionPlan(ExecutionPlanContext{Objective: "Investigate", Preference: PlanPreferenceAlways, Stages: []ExecutionStageSnapshot{stage}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if explicit == nil || explicit.State != "pending" || explicit.EstimatedModelCalls != 1 || explicit.EstimatedCreditHundredths != 125 || explicit.GenerationCreditHundredths != 0 {
		t.Fatalf("unexpected explicit Plan: %#v", explicit)
	}
}

func TestBuildExecutionPlanDeduplicatesResourcesAndMarksSideEffects(t *testing.T) {
	connector := CLIConnectorSnapshot{ID: "crm", Name: "CRM", Capabilities: json.RawMessage(`[{"risk":"high"}]`)}
	stages := []ExecutionStageSnapshot{
		{Position: 1, CLIConnectors: []CLIConnectorSnapshot{connector}},
		{Position: 2, CLIConnectors: []CLIConnectorSnapshot{connector}},
	}
	plan, err := BuildExecutionPlan(ExecutionPlanContext{Objective: "Update CRM", Stages: stages}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || len(plan.Resources) != 1 || len(plan.SideEffects) != 1 || plan.AllowsDirectAnswer() {
		t.Fatalf("unexpected side-effect Plan: %#v", plan)
	}
}
