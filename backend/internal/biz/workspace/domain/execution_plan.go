package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	PlanPreferenceAuto   = "auto"
	PlanPreferenceAlways = "always"
)

type ExecutionPlan struct {
	ID                         string                  `json:"id"`
	State                      string                  `json:"state"`
	Objective                  string                  `json:"objective"`
	Steps                      []ExecutionPlanStep     `json:"steps"`
	Resources                  []ExecutionPlanResource `json:"resources"`
	SideEffects                []string                `json:"side_effects"`
	Reasons                    []string                `json:"reasons"`
	EstimatedModelCalls        int                     `json:"estimated_model_calls"`
	EstimatedCreditHundredths  int64                   `json:"estimated_credit_hundredths"`
	GenerationCreditHundredths int64                   `json:"generation_credit_hundredths"`
	Generator                  string                  `json:"generator"`
	CreatedAt                  time.Time               `json:"created_at"`
	DecidedAt                  *time.Time              `json:"decided_at,omitempty"`
	Version                    int64                   `json:"version"`
}

type ExecutionPlanStep struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Label    string `json:"label,omitempty"`
	Position int    `json:"position"`
	State    string `json:"state"`
}

type ExecutionPlanResource struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ExecutionPlanContext struct {
	Objective        string
	Preference       string
	Stages           []ExecutionStageSnapshot
	KnowledgeBaseIDs []string
	GitSource        *GitSource
	Workflow         bool
}

func BuildExecutionPlan(input ExecutionPlanContext, now time.Time) (*ExecutionPlan, error) {
	preference := strings.TrimSpace(input.Preference)
	if preference == "" {
		preference = PlanPreferenceAuto
	}
	if preference != PlanPreferenceAuto && preference != PlanPreferenceAlways {
		return nil, fmt.Errorf("%w: invalid Plan preference", ErrInvalid)
	}
	if len(input.Stages) == 0 {
		return nil, fmt.Errorf("%w: Plan requires at least one execution stage", ErrInvalid)
	}
	resources, externalCount, hasExternalSideEffect := planResources(input.Stages, input.KnowledgeBaseIDs)
	reasons := make([]string, 0, 5)
	sideEffects := make([]string, 0, 2)
	if preference == PlanPreferenceAlways {
		reasons = append(reasons, "user_requested")
	}
	if len(input.Stages) > 1 {
		reasons = append(reasons, "multiple_stages")
	}
	if externalCount > 1 {
		reasons = append(reasons, "multiple_external_sources")
	}
	if hasExternalSideEffect {
		reasons = append(reasons, "external_side_effect")
	}
	if input.Workflow {
		reasons = append(reasons, "workflow_execution")
		sideEffects = append(sideEffects, "workspace_files_may_change")
	}
	if hasExternalSideEffect {
		sideEffects = append(sideEffects, "external_connector_operation")
	}
	if input.GitSource != nil {
		if !containsPlanReason(reasons, "workflow_execution") {
			reasons = append(reasons, "workspace_change")
		}
	}
	if len(reasons) == 0 {
		return nil, nil
	}
	steps := []ExecutionPlanStep{{ID: uuid.NewString(), Kind: "review_input", Position: 1, State: "pending"}}
	if len(input.KnowledgeBaseIDs) > 0 {
		steps = append(steps, ExecutionPlanStep{ID: uuid.NewString(), Kind: "retrieve_knowledge", Position: len(steps) + 1, State: "pending"})
	}
	for _, stage := range input.Stages {
		label := ""
		if stage.Expert != nil {
			label = stage.Expert.Name
		} else if stage.TeamMemberName != "" {
			label = stage.TeamMemberName
		}
		steps = append(steps, ExecutionPlanStep{ID: uuid.NewString(), Kind: "execute_stage", Label: label, Position: len(steps) + 1, State: "pending"})
	}
	steps = append(steps, ExecutionPlanStep{ID: uuid.NewString(), Kind: "deliver_result", Position: len(steps) + 1, State: "pending"})
	estimatedCredits := int64(0)
	for _, stage := range input.Stages {
		if stage.CreditRate != nil && stage.CreditRate.FallbackHundredths > 0 {
			estimatedCredits += stage.CreditRate.FallbackHundredths
		}
	}
	return &ExecutionPlan{
		ID: uuid.NewString(), State: "pending", Objective: boundedPlanObjective(input.Objective), Steps: steps, Resources: resources,
		SideEffects: sideEffects, Reasons: reasons, EstimatedModelCalls: len(input.Stages), EstimatedCreditHundredths: estimatedCredits,
		GenerationCreditHundredths: 0, Generator: "platform_rules", CreatedAt: now.UTC(), Version: 1,
	}, nil
}

func (plan ExecutionPlan) AllowsDirectAnswer() bool { return len(plan.SideEffects) == 0 }

// CompleteModelGeneration changes only the user-visible labels. The ordered
// step kinds and their Worker-owned state transitions remain frozen.
func (plan *ExecutionPlan) CompleteModelGeneration(labels []string, creditHundredths int64, generationFailed bool) error {
	if plan == nil || plan.State != "pending" || creditHundredths < 0 {
		return fmt.Errorf("%w: invalid model Plan generation state", ErrInvalid)
	}
	if generationFailed {
		plan.Generator = "model_failed"
	} else {
		if len(labels) != len(plan.Steps) {
			return fmt.Errorf("%w: model Plan label count does not match steps", ErrInvalid)
		}
		seen := make(map[string]bool, len(labels))
		normalized := make([]string, len(labels))
		for index, label := range labels {
			label = strings.TrimSpace(label)
			key := strings.ToLower(label)
			if len([]rune(label)) < 4 || len([]rune(label)) > 80 || strings.ContainsAny(label, "\r\n\x00") || seen[key] {
				return fmt.Errorf("%w: invalid model Plan step label", ErrInvalid)
			}
			seen[key] = true
			normalized[index] = label
		}
		for index, label := range normalized {
			plan.Steps[index].Label = label
		}
		plan.Generator = "model"
	}
	plan.GenerationCreditHundredths = creditHundredths
	plan.Version++
	return plan.Validate()
}

func (plan ExecutionPlan) Validate() error {
	if plan.ID == "" || plan.Version < 1 || plan.Objective == "" || (plan.Generator != "platform_rules" && plan.Generator != "model" && plan.Generator != "model_failed") || len(plan.Steps) < 2 || plan.EstimatedModelCalls < 1 || plan.EstimatedCreditHundredths < 0 || plan.GenerationCreditHundredths < 0 {
		return fmt.Errorf("%w: invalid Execution Plan", ErrInvalid)
	}
	if plan.State != "pending" && plan.State != "approved" && plan.State != "executing" && plan.State != "completed" && plan.State != "failed" && plan.State != "cancelled" && plan.State != "skipped" {
		return fmt.Errorf("%w: invalid Execution Plan state", ErrInvalid)
	}
	for index, step := range plan.Steps {
		if step.ID == "" || step.Position != index+1 || (step.State != "pending" && step.State != "running" && step.State != "completed" && step.State != "skipped" && step.State != "failed") {
			return fmt.Errorf("%w: invalid Execution Plan step", ErrInvalid)
		}
	}
	return nil
}

func planResources(stages []ExecutionStageSnapshot, knowledgeBaseIDs []string) ([]ExecutionPlanResource, int, bool) {
	resources := make([]ExecutionPlanResource, 0)
	seen := make(map[string]bool)
	externalCount := len(knowledgeBaseIDs)
	hasExternalSideEffect := false
	add := func(kind, id, name string) bool {
		key := kind + ":" + id
		if id == "" || seen[key] {
			return false
		}
		seen[key] = true
		resources = append(resources, ExecutionPlanResource{Kind: kind, ID: id, Name: name})
		return true
	}
	for _, id := range knowledgeBaseIDs {
		add("knowledge", id, "Knowledge Base")
	}
	for _, stage := range stages {
		if stage.Expert != nil {
			add("expert", stage.Expert.ID, stage.Expert.Name)
		}
		for _, server := range stage.MCPServers {
			if add("mcp", server.ID, server.Name) {
				externalCount++
			}
			hasExternalSideEffect = true
		}
		for _, connector := range stage.CLIConnectors {
			if add("connector", connector.ID, connector.Name) {
				externalCount++
			}
			if connectorMayWrite(connector.Capabilities) {
				hasExternalSideEffect = true
			}
		}
	}
	return resources, externalCount, hasExternalSideEffect
}

func connectorMayWrite(encoded json.RawMessage) bool {
	var capabilities []struct {
		Risk string `json:"risk"`
	}
	if len(encoded) == 0 || json.Unmarshal(encoded, &capabilities) != nil {
		return true
	}
	for _, capability := range capabilities {
		if capability.Risk == "high" {
			return true
		}
	}
	return false
}

func boundedPlanObjective(value string) string {
	runes := []rune(strings.Join(strings.Fields(value), " "))
	if len(runes) > 500 {
		runes = runes[:500]
	}
	if len(runes) == 0 {
		return "Use the selected input and resources"
	}
	return string(runes)
}

func containsPlanReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
