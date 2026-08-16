package domain

import (
	"fmt"
	"strings"
	"time"
)

type DraftState string

const (
	DraftStateDraft      DraftState = "draft"
	DraftStateValidating DraftState = "validating"
	DraftStateReady      DraftState = "ready"
	DraftStateBlocked    DraftState = "blocked"
)

type ReleaseRisk string

const (
	ReleaseRiskLow  ReleaseRisk = "low"
	ReleaseRiskHigh ReleaseRisk = "high"
)

type ModelBudget struct {
	MaxInputTokens  int64  `json:"max_input_tokens"`
	MaxOutputTokens int64  `json:"max_output_tokens"`
	MaxCostAmount   string `json:"max_cost_amount"`
}

type ExecutionLimits struct {
	TimeoutSeconds int64   `json:"timeout_seconds"`
	CPUs           float64 `json:"cpus"`
	MemoryBytes    int64   `json:"memory_bytes"`
	PIDs           int64   `json:"pids"`
	TempBytes      int64   `json:"temp_bytes"`
	Egress         string  `json:"egress"`
}

type Configuration struct {
	Instructions        string          `json:"instructions"`
	RepositoryBindingID string          `json:"repository_binding_id"`
	RuntimeImageID      string          `json:"runtime_image_id"`
	ConfiguredModelID   string          `json:"configured_model_id"`
	ModelBudget         ModelBudget     `json:"model_budget"`
	ExecutionLimits     ExecutionLimits `json:"execution_limits"`
	NativeSubagents     bool            `json:"native_subagents"`
}

type ValidationReport struct {
	Valid     bool              `json:"valid"`
	Errors    map[string]string `json:"errors"`
	CheckedAt time.Time         `json:"checked_at"`
}

type Draft struct {
	ID               string
	AgentID          string
	Revision         int64
	State            DraftState
	Configuration    Configuration
	ReleaseRisk      ReleaseRisk
	ValidationReport *ValidationReport
	CreatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

type DraftRegistration struct {
	ID            string
	AgentID       string
	Revision      int64
	Configuration Configuration
	ReleaseRisk   ReleaseRisk
	CreatedBy     string
	Now           time.Time
}

func CreateDraft(input DraftRegistration) (Draft, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.AgentID) == "" || strings.TrimSpace(input.CreatedBy) == "" || input.Revision <= 0 || input.Now.IsZero() {
		return Draft{}, invalidAgentf("Draft identity, Agent, revision, creator, and creation time are required")
	}
	if err := input.Configuration.Validate(); err != nil {
		return Draft{}, err
	}
	if input.ReleaseRisk != ReleaseRiskLow && input.ReleaseRisk != ReleaseRiskHigh {
		return Draft{}, invalidAgentf("Draft Release Risk must be low or high")
	}
	if input.Configuration.NativeSubagents && input.ReleaseRisk != ReleaseRiskHigh {
		return Draft{}, invalidAgentf("native Subagents require high Release Risk")
	}
	now := input.Now.UTC()
	return Draft{
		ID: input.ID, AgentID: input.AgentID, Revision: input.Revision, State: DraftStateDraft,
		Configuration: input.Configuration, ReleaseRisk: input.ReleaseRisk, CreatedBy: input.CreatedBy,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreDraft(input DraftRegistration, state DraftState, report *ValidationReport, createdAt, updatedAt time.Time, version int64) (Draft, error) {
	input.Now = createdAt
	draft, err := CreateDraft(input)
	if err != nil {
		return Draft{}, err
	}
	if version <= 0 || updatedAt.IsZero() || updatedAt.Before(createdAt) {
		return Draft{}, invalidAgentf("persisted Draft timestamps or Version are invalid")
	}
	switch state {
	case DraftStateDraft, DraftStateValidating:
		if report != nil {
			return Draft{}, invalidAgentf("persisted Draft state cannot carry a Validation Report")
		}
	case DraftStateReady, DraftStateBlocked:
		if report == nil || report.Valid != (state == DraftStateReady) || report.CheckedAt.IsZero() {
			return Draft{}, invalidAgentf("persisted Draft Validation Report is inconsistent")
		}
		copy := *report
		copy.Errors = copyErrors(report.Errors)
		draft.ValidationReport = &copy
	default:
		return Draft{}, invalidAgentf("persisted Draft state is invalid")
	}
	draft.State = state
	draft.UpdatedAt = updatedAt.UTC()
	draft.Version = version
	return draft, nil
}

func (configuration Configuration) Validate() error {
	if strings.TrimSpace(configuration.Instructions) == "" || len(configuration.Instructions) > 100_000 {
		return invalidAgentf("Draft instructions are required and cannot exceed 100000 characters")
	}
	if configuration.RepositoryBindingID == "" || configuration.RuntimeImageID == "" || configuration.ConfiguredModelID == "" {
		return invalidAgentf("Repository Binding, Runtime Image, and Configured Model are required")
	}
	if configuration.ModelBudget.MaxInputTokens <= 0 || configuration.ModelBudget.MaxOutputTokens <= 0 || !positiveDecimal(configuration.ModelBudget.MaxCostAmount) {
		return invalidAgentf("Draft Model Budget is invalid")
	}
	limits := configuration.ExecutionLimits
	if limits.TimeoutSeconds <= 0 || limits.TimeoutSeconds > 7200 || limits.CPUs <= 0 || limits.MemoryBytes <= 0 || limits.PIDs <= 0 || limits.TempBytes <= 0 || limits.Egress != "public" {
		return invalidAgentf("Draft Execution Limits are invalid")
	}
	return nil
}

func (draft *Draft) Edit(configuration Configuration, risk ReleaseRisk, now time.Time) error {
	if draft.State == DraftStateValidating || now.IsZero() || now.Before(draft.UpdatedAt) {
		return invalidAgentf("Draft cannot be edited while validating or with an invalid update time")
	}
	if err := configuration.Validate(); err != nil {
		return err
	}
	if risk != ReleaseRiskLow && risk != ReleaseRiskHigh {
		return invalidAgentf("Draft Release Risk must be low or high")
	}
	if configuration.NativeSubagents && risk != ReleaseRiskHigh {
		return invalidAgentf("native Subagents require high Release Risk")
	}
	draft.Configuration = configuration
	draft.ReleaseRisk = risk
	draft.State = DraftStateDraft
	draft.ValidationReport = nil
	draft.UpdatedAt = now.UTC()
	draft.Version++
	return nil
}

func (draft *Draft) StartValidation(now time.Time) error {
	if draft.State == DraftStateValidating || now.IsZero() || now.Before(draft.UpdatedAt) {
		return invalidAgentf("Draft cannot start validation")
	}
	draft.State = DraftStateValidating
	draft.ValidationReport = nil
	draft.UpdatedAt = now.UTC()
	draft.Version++
	return nil
}

func (draft *Draft) FinishValidation(report ValidationReport, now time.Time) error {
	if draft.State != DraftStateValidating || report.CheckedAt.IsZero() || now.IsZero() || now.Before(draft.UpdatedAt) {
		return invalidAgentf("Draft is not validating or validation timestamps are invalid")
	}
	if report.Valid && len(report.Errors) != 0 || !report.Valid && len(report.Errors) == 0 {
		return invalidAgentf("Draft Validation Report is inconsistent")
	}
	report.CheckedAt = report.CheckedAt.UTC()
	report.Errors = copyErrors(report.Errors)
	draft.ValidationReport = &report
	if report.Valid {
		draft.State = DraftStateReady
	} else {
		draft.State = DraftStateBlocked
	}
	draft.UpdatedAt = now.UTC()
	draft.Version++
	return nil
}

func positiveDecimal(value string) bool {
	if value == "" || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return false
	}
	whole, fraction, found := strings.Cut(value, ".")
	if whole == "" || found && (fraction == "" || len(fraction) > 8) {
		return false
	}
	for _, character := range whole + fraction {
		if character < '0' || character > '9' {
			return false
		}
	}
	return strings.Trim(value, "0.") != ""
}

func copyErrors(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func (draft Draft) CanRelease() error {
	if draft.State != DraftStateReady || draft.ValidationReport == nil || !draft.ValidationReport.Valid {
		return ErrDraftNotReady
	}
	if err := draft.Configuration.Validate(); err != nil {
		return fmt.Errorf("ready Draft configuration became invalid: %w", err)
	}
	return nil
}
