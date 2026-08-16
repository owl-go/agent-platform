package draftvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"agent-platform/backend/internal/biz/agentlifecycle/domain"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"

	"gorm.io/gorm"
)

type Validator struct{ db *gorm.DB }

func New(db *gorm.DB) *Validator { return &Validator{db: db} }

type bindingProjection struct {
	OrganizationID         string          `gorm:"column:organization_id"`
	TeamID                 string          `gorm:"column:team_id"`
	DefaultModelID         string          `gorm:"column:default_model_id"`
	AllowedRuntimeImageIDs json.RawMessage `gorm:"column:allowed_runtime_image_ids"`
	ModelBudget            json.RawMessage `gorm:"column:model_budget"`
	ValidationReport       json.RawMessage `gorm:"column:validation_report"`
}

type runtimeProjection struct {
	Status       string          `gorm:"column:status"`
	Capabilities json.RawMessage `gorm:"column:capabilities"`
}

type modelProjection struct {
	OrganizationID string `gorm:"column:organization_id"`
	Enabled        bool   `gorm:"column:enabled"`
}

func (validator *Validator) Validate(ctx context.Context, agent domain.Agent, draft domain.Draft) (map[string]string, error) {
	if validator == nil || validator.db == nil {
		return nil, fmt.Errorf("Agent Draft validation database is required")
	}
	errorsByField := make(map[string]string)
	var binding bindingProjection
	err := validator.db.WithContext(ctx).Table("repository_bindings").Where("id = ?", draft.Configuration.RepositoryBindingID).Take(&binding).Error
	if err == gorm.ErrRecordNotFound {
		errorsByField["repository_binding_id"] = "Repository Binding does not exist"
		return errorsByField, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load Repository Binding for Draft validation: %w", err)
	}
	if binding.OrganizationID != agent.OrganizationID || binding.TeamID != agent.TeamID {
		errorsByField["repository_binding_id"] = "Repository Binding is outside the Agent Organization/Team scope"
	}
	var bindingReport sourcedomain.ValidationReport
	if len(binding.ValidationReport) == 0 || json.Unmarshal(binding.ValidationReport, &bindingReport) != nil || !bindingReport.Valid {
		errorsByField["repository_binding_id"] = "Repository Binding does not have a valid Validation Report"
	}
	var allowedRuntimes []string
	if err := json.Unmarshal(binding.AllowedRuntimeImageIDs, &allowedRuntimes); err != nil {
		return nil, fmt.Errorf("decode Repository Binding Runtime policy: %w", err)
	}
	if !contains(allowedRuntimes, draft.Configuration.RuntimeImageID) {
		errorsByField["runtime_image_id"] = "Runtime Image is not allowed by Repository Binding"
	}
	if binding.DefaultModelID != draft.Configuration.ConfiguredModelID {
		errorsByField["configured_model_id"] = "Configured Model does not match Repository Binding Model policy"
	}
	var bindingBudget sourcedomain.ModelBudget
	if err := json.Unmarshal(binding.ModelBudget, &bindingBudget); err != nil {
		return nil, fmt.Errorf("decode Repository Binding Model Budget: %w", err)
	}
	if draft.Configuration.ModelBudget.MaxInputTokens > bindingBudget.MaxInputTokens || draft.Configuration.ModelBudget.MaxOutputTokens > bindingBudget.MaxOutputTokens || decimalGreater(draft.Configuration.ModelBudget.MaxCostAmount, bindingBudget.MaxCostAmount) {
		errorsByField["model_budget"] = "Draft Model Budget exceeds Repository Binding limits"
	}

	var runtime runtimeProjection
	err = validator.db.WithContext(ctx).Table("runtime_images").Where("id = ?", draft.Configuration.RuntimeImageID).Take(&runtime).Error
	if err == gorm.ErrRecordNotFound {
		errorsByField["runtime_image_id"] = "Runtime Image does not exist"
	} else if err != nil {
		return nil, fmt.Errorf("load Runtime Image for Draft validation: %w", err)
	} else {
		if runtime.Status != "production" {
			errorsByField["runtime_image_id"] = "Runtime Image is not production"
		}
		var capabilities map[string]bool
		if err := json.Unmarshal(runtime.Capabilities, &capabilities); err != nil {
			return nil, fmt.Errorf("decode Runtime Image Capabilities: %w", err)
		}
		if draft.Configuration.NativeSubagents && !capabilities["subagents"] {
			errorsByField["native_subagents"] = "Runtime Image does not support native Subagents"
		}
	}

	var model modelProjection
	err = validator.db.WithContext(ctx).Table("configured_models").Where("id = ?", draft.Configuration.ConfiguredModelID).Take(&model).Error
	if err == gorm.ErrRecordNotFound {
		errorsByField["configured_model_id"] = "Configured Model does not exist"
	} else if err != nil {
		return nil, fmt.Errorf("load Configured Model for Draft validation: %w", err)
	} else if model.OrganizationID != agent.OrganizationID || !model.Enabled {
		errorsByField["configured_model_id"] = "Configured Model is outside the Agent Organization or disabled"
	}
	return errorsByField, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func decimalGreater(left, right string) bool {
	leftValue, leftOK := new(big.Rat).SetString(left)
	rightValue, rightOK := new(big.Rat).SetString(right)
	return !leftOK || !rightOK || leftValue.Cmp(rightValue) > 0
}
