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
