package bindingvalidator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/sourcecontrol/domain"

	"gorm.io/gorm"
)

type Validator struct{ db *gorm.DB }

func New(db *gorm.DB) *Validator { return &Validator{db: db} }

type providerProjection struct {
	OrganizationID string `gorm:"column:organization_id"`
	BaseURL        string `gorm:"column:base_url"`
	Enabled        bool   `gorm:"column:enabled"`
}

type credentialProjection struct {
	ID             string     `gorm:"column:id"`
	OrganizationID string     `gorm:"column:organization_id"`
	TeamID         *string    `gorm:"column:team_id"`
	Kind           string     `gorm:"column:kind"`
	DisabledAt     *time.Time `gorm:"column:disabled_at"`
}

type runtimeProjection struct {
	ID           string          `gorm:"column:id"`
	Status       string          `gorm:"column:status"`
	Capabilities json.RawMessage `gorm:"column:capabilities"`
}

type modelProjection struct {
	OrganizationID string `gorm:"column:organization_id"`
	Enabled        bool   `gorm:"column:enabled"`
}

type dependencies struct {
	provider   providerProjection
	ssh        credentialProjection
	build      []credentialProjection
	runtimes   []runtimeProjection
	model      modelProjection
	teamExists bool
}

func (validator *Validator) CheckReferences(ctx context.Context, binding domain.RepositoryBinding) error {
	loaded, err := validator.load(ctx, binding)
	if err != nil {
		return err
	}
	if !loaded.teamExists || loaded.provider.OrganizationID != binding.OrganizationID {
		return invalid("Team and Source Control Provider must belong to the Repository Binding Organization")
	}
	if err := checkCredentialScope(binding, loaded.ssh, "git_ssh"); err != nil {
		return err
	}
	if len(loaded.build) != len(binding.BuildCredentialProfileIDs) {
		return invalid("all build Credential Profiles must exist")
	}
	for _, credential := range loaded.build {
		if err := checkCredentialScope(binding, credential, "build"); err != nil {
			return err
		}
	}
	if len(loaded.runtimes) != len(binding.AllowedRuntimeImageIDs) {
		return invalid("all allowed Runtime Images must exist")
	}
	if loaded.model.OrganizationID != binding.OrganizationID {
		return invalid("Configured Model must belong to the Repository Binding Organization")
	}
	return nil
}

func (validator *Validator) Validate(ctx context.Context, binding domain.RepositoryBinding) (map[string]string, error) {
	loaded, err := validator.load(ctx, binding)
	if err != nil {
		return nil, err
	}
	errorsByField := make(map[string]string)
	for field, message := range binding.PolicyErrors() {
		errorsByField[field] = message
	}
	if !loaded.provider.Enabled {
		errorsByField["source_control_provider_id"] = "Source Control Provider is disabled"
	}
	providerURL, err := url.Parse(loaded.provider.BaseURL)
	if err != nil || !strings.EqualFold(providerURL.Hostname(), binding.RepositoryHost) {
		errorsByField["repository_ssh_url"] = "repository host does not match Source Control Provider"
	}
	if loaded.ssh.DisabledAt != nil {
		errorsByField["ssh_credential_profile_id"] = "SSH Credential Profile is disabled"
	}
	for _, credential := range loaded.build {
		if credential.DisabledAt != nil {
			errorsByField["build_credential_profile_ids"] = "one or more build Credential Profiles are disabled"
			break
		}
	}
	for _, runtime := range loaded.runtimes {
		if runtime.Status != "production" {
			errorsByField["allowed_runtime_image_ids"] = "all allowed Runtime Images must be production"
		}
		var capabilities map[string]bool
		if err := json.Unmarshal(runtime.Capabilities, &capabilities); err != nil {
			return nil, fmt.Errorf("decode Runtime Image Capabilities: %w", err)
		}
		for _, required := range binding.RequiredRuntimeCapabilities {
			if !capabilities[required] {
				errorsByField["required_runtime_capabilities"] = "all allowed Runtime Images must provide every required Runtime Capability"
				break
			}
		}
	}
	if !loaded.model.Enabled {
		errorsByField["default_model_id"] = "Configured Model is disabled"
	}
	return errorsByField, nil
}

func (validator *Validator) load(ctx context.Context, binding domain.RepositoryBinding) (dependencies, error) {
	if validator == nil || validator.db == nil {
		return dependencies{}, fmt.Errorf("Repository Binding validation database is required")
	}
	db := validator.db.WithContext(ctx)
	var loaded dependencies
	var teamCount int64
	if err := db.Table("teams").Where("id = ? AND organization_id = ?", binding.TeamID, binding.OrganizationID).Count(&teamCount).Error; err != nil {
		return dependencies{}, fmt.Errorf("validate Repository Binding Team: %w", err)
	}
	loaded.teamExists = teamCount == 1
	if err := db.Table("source_control_providers").Where("id = ?", binding.SourceControlProviderID).Take(&loaded.provider).Error; err != nil {
		return dependencies{}, referenceError("Source Control Provider", err)
	}
	if err := db.Table("credential_profiles").Where("id = ?", binding.SSHCredentialProfileID).Take(&loaded.ssh).Error; err != nil {
		return dependencies{}, referenceError("SSH Credential Profile", err)
	}
	if len(binding.BuildCredentialProfileIDs) > 0 {
		if err := db.Table("credential_profiles").Where("id IN ?", binding.BuildCredentialProfileIDs).Find(&loaded.build).Error; err != nil {
			return dependencies{}, fmt.Errorf("load build Credential Profiles: %w", err)
		}
	}
	if err := db.Table("runtime_images").Where("organization_id = ? AND id IN ?", binding.OrganizationID, binding.AllowedRuntimeImageIDs).Find(&loaded.runtimes).Error; err != nil {
		return dependencies{}, fmt.Errorf("load allowed Runtime Images: %w", err)
	}
	if err := db.Table("configured_models").Where("id = ?", binding.DefaultModelID).Take(&loaded.model).Error; err != nil {
		return dependencies{}, referenceError("Configured Model", err)
	}
	return loaded, nil
}

func checkCredentialScope(binding domain.RepositoryBinding, credential credentialProjection, kind string) error {
	teamAllowed := credential.TeamID == nil || *credential.TeamID == binding.TeamID
	if credential.OrganizationID != binding.OrganizationID || !teamAllowed || credential.Kind != kind {
		return invalid("Credential Profiles must have the required kind and belong to the Repository Binding Organization/Team scope")
	}
	return nil
}

func referenceError(name string, err error) error {
	if err == gorm.ErrRecordNotFound {
		return invalid("%s does not exist", name)
	}
	return fmt.Errorf("load %s: %w", name, err)
}

func invalid(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidBinding, fmt.Sprintf(format, arguments...))
}
