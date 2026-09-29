package gormrepo

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func testExecutionPlan(sideEffects ...string) domain.ExecutionPlan {
	return domain.ExecutionPlan{
		ID: "plan-1", State: "pending", Objective: "Do the work", SideEffects: sideEffects, Reasons: []string{"user_requested"},
		Steps:               []domain.ExecutionPlanStep{{ID: "review", Kind: "review_input", Position: 1, State: "pending"}, {ID: "run", Kind: "execute_stage", Position: 2, State: "pending"}, {ID: "result", Kind: "deliver_result", Position: 3, State: "pending"}},
		EstimatedModelCalls: 1, Generator: "platform_rules", CreatedAt: time.Now(), Version: 1,
	}
}

func TestDecideExecutionPlanRejectsDirectSideEffectBypass(t *testing.T) {
	plan := testExecutionPlan("external_connector_operation")
	err := decideExecutionPlan(&plan, "direct", 1, time.Now())
	if !errors.Is(err, domain.ErrInvalid) || plan.State != "pending" {
		t.Fatalf("decision error = %v, state = %q", err, plan.State)
	}
}

func TestSessionMessagePairOnlyWaitsForRequestedPlan(t *testing.T) {
	for _, test := range []struct {
		name      string
		planState string
		wantState string
	}{
		{name: "automatic safety Plan", planState: "approved", wantState: "queued"},
		{name: "explicit Plan", planState: "pending", wantState: "waiting_for_user"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := testExecutionPlan("external_connector_operation")
			plan.State = test.planState
			encoded, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			_, assistant := sessionMessagePairRecords("session", "request", []byte("[]"), []byte("{}"), encoded, plan.State == "pending")
			if assistant.State != test.wantState {
				t.Fatalf("assistant state = %q, want %q", assistant.State, test.wantState)
			}
		})
	}
}

func TestExecutionPlanLifecycleProjection(t *testing.T) {
	plan := testExecutionPlan()
	if err := decideExecutionPlan(&plan, "start", 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(plan)
	started, err := executionPlanStarted(encoded)
	if err != nil {
		t.Fatal(err)
	}
	stage, err := executionPlanStageUpdated(started, 1, "succeeded")
	if err != nil {
		t.Fatal(err)
	}
	finished, err := executionPlanFinished(stage, "completed")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(finished, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.State != "completed" {
		t.Fatalf("state = %q", plan.State)
	}
	for _, step := range plan.Steps {
		if step.State != "completed" {
			t.Fatalf("step %q state = %q", step.Kind, step.State)
		}
	}
}

func TestCompletePlanGenerationPreservesFrozenRulesAndVersion(t *testing.T) {
	plan := testExecutionPlan("external_connector_operation")
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := completePlanGeneration(encoded, []string{"确认 CRM 更新范围", "逐条核对并更新 CRM", "汇总更新结果与异常"}, 25, false)
	if err != nil {
		t.Fatal(err)
	}
	var updated domain.ExecutionPlan
	if err := json.Unmarshal(generated, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Generator != "model" || updated.Version != 2 || updated.GenerationCreditHundredths != 25 || updated.AllowsDirectAnswer() || updated.Steps[1].Kind != "execute_stage" {
		t.Fatalf("model changed frozen Plan rules: %#v", updated)
	}
	if _, err := completePlanGeneration(generated, []string{"重复生成"}, 0, false); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("repeat generation should conflict: %v", err)
	}
	fallback, err := completePlanGeneration(encoded, nil, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	updated = domain.ExecutionPlan{}
	if err := json.Unmarshal(fallback, &updated); err != nil || updated.Generator != "model_failed" || updated.Steps[1].Label != "" {
		t.Fatalf("fallback Plan = %#v, %v", updated, err)
	}
}
