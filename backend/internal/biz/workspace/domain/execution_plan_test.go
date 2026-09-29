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
	if plan == nil || len(plan.Resources) != 1 || len(plan.SideEffects) != 1 || plan.AllowsDirectAnswer() || plan.Generator != "platform_rules" || plan.GenerationCreditHundredths != 0 {
		t.Fatalf("unexpected side-effect Plan: %#v", plan)
	}
}

func TestCompleteModelGenerationPreservesExecutionStepKinds(t *testing.T) {
	plan, err := BuildExecutionPlan(ExecutionPlanContext{
		Objective: "分析项目代码", Preference: PlanPreferenceAlways,
		Stages: []ExecutionStageSnapshot{{Position: 1}},
	}, time.Now())
	if err != nil || plan == nil {
		t.Fatalf("build Plan: %#v, %v", plan, err)
	}
	kinds := []string{plan.Steps[0].Kind, plan.Steps[1].Kind, plan.Steps[2].Kind}
	if err := plan.CompleteModelGeneration([]string{
		"确认代码分析范围和入口", "梳理目录并追踪核心调用链", "汇总鉴权风险与改进建议",
	}, 37, false); err != nil {
		t.Fatal(err)
	}
	if plan.Generator != "model" || plan.GenerationCreditHundredths != 37 || plan.Version != 2 {
		t.Fatalf("unexpected model Plan metadata: %#v", plan)
	}
	for index, step := range plan.Steps {
		if step.Kind != kinds[index] || step.Label == "" || step.State != "pending" {
			t.Fatalf("model altered execution step %d: %#v", index, step)
		}
	}
}

func TestCompleteModelGenerationRejectsIncompleteLabelsAndRecordsFailure(t *testing.T) {
	plan, err := BuildExecutionPlan(ExecutionPlanContext{
		Objective: "分析项目代码", Preference: PlanPreferenceAlways,
		Stages: []ExecutionStageSnapshot{{Position: 1}},
	}, time.Now())
	if err != nil || plan == nil {
		t.Fatalf("build Plan: %#v, %v", plan, err)
	}
	if err := plan.CompleteModelGeneration([]string{"only one"}, 0, false); err == nil {
		t.Fatal("incomplete model Plan should be rejected")
	}
	if err := plan.CompleteModelGeneration([]string{"确认代码范围", "梳理核心调用", "梳理核心调用"}, 0, false); err == nil {
		t.Fatal("duplicate model Plan labels should be rejected")
	}
	for _, step := range plan.Steps {
		if step.Label != "" {
			t.Fatalf("rejected model Plan changed a step: %#v", step)
		}
	}
	if err := plan.CompleteModelGeneration(nil, 0, true); err != nil {
		t.Fatal(err)
	}
	if plan.Generator != "model_failed" || plan.GenerationCreditHundredths != 0 {
		t.Fatalf("failed generation was not visible: %#v", plan)
	}
}
