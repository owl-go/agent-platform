package releasedependency

import (
	"context"
	"encoding/json"
	"fmt"

	agentapplication "agent-platform/backend/internal/biz/agentlifecycle/application"
	agentdomain "agent-platform/backend/internal/biz/agentlifecycle/domain"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BindingCatalog interface {
	Get(context.Context, string, string, string) (sourcedomain.RepositoryBinding, error)
	InspectCurrent(context.Context, string, string, string) (sourcedomain.RepositoryBinding, map[string]string, error)
}

type Resolver struct {
	db             *gorm.DB
	drafts         agentapplication.DraftValidator
	sourceBindings BindingCatalog
}

func New(db *gorm.DB, drafts agentapplication.DraftValidator, sourceBindings BindingCatalog) *Resolver {
	return &Resolver{db: db, drafts: drafts, sourceBindings: sourceBindings}
}

type runtimeProjection struct {
	ID           string          `gorm:"column:id"`
	Runtime      string          `gorm:"column:runtime"`
	CLIVersion   string          `gorm:"column:cli_version"`
	Adapter      string          `gorm:"column:adapter_version"`
	ImageDigest  string          `gorm:"column:image_digest"`
	Capabilities json.RawMessage `gorm:"column:capabilities"`
}

type modelProjection struct {
	ID                  string `gorm:"column:id"`
	Name                string `gorm:"column:name"`
	ModelID             string `gorm:"column:model_id"`
	Endpoint            string `gorm:"column:endpoint"`
	CredentialProfileID string `gorm:"column:credential_profile_id"`
}

func (resolver *Resolver) PrepareRelease(ctx context.Context, agent agentdomain.Agent, draft agentdomain.Draft) (agentdomain.ReleaseDependencies, map[string]string, error) {
	if resolver == nil || resolver.db == nil || resolver.drafts == nil || resolver.sourceBindings == nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("Agent Release dependency resolver is incomplete")
	}
	db := resolver.db.WithContext(ctx)
	if err := lockRows(db, "repository_bindings", []string{draft.Configuration.RepositoryBindingID}); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("lock Repository Binding for Agent Release: %w", err)
	}
	binding, err := resolver.sourceBindings.Get(ctx, agent.OrganizationID, agent.TeamID, draft.Configuration.RepositoryBindingID)
	if err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	if err := lockRows(db, "source_control_providers", []string{binding.SourceControlProviderID}); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	if err := lockRows(db, "configured_models", []string{draft.Configuration.ConfiguredModelID}); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	var model modelProjection
	if err := db.Table("configured_models").Where("id = ? AND organization_id = ?", draft.Configuration.ConfiguredModelID, agent.OrganizationID).Take(&model).Error; err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("load Configured Model for Agent Release: %w", err)
	}
	credentialIDs := append([]string{binding.SSHCredentialProfileID, model.CredentialProfileID}, binding.BuildCredentialProfileIDs...)
	if err := lockRows(db, "credential_profiles", credentialIDs); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	if err := lockRows(db, "runtime_images", binding.AllowedRuntimeImageIDs); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	errorsByField, err := resolver.drafts.Validate(ctx, agent, draft)
	if err != nil {
		return agentdomain.ReleaseDependencies{}, nil, err
	}
	binding, bindingErrors, err := resolver.sourceBindings.InspectCurrent(ctx, agent.OrganizationID, agent.TeamID, binding.ID)
	if err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("inspect Repository Binding for Agent Release: %w", err)
	}
	for field, message := range bindingErrors {
		errorsByField[field] = message
	}

	var runtime runtimeProjection
	if err := db.Table("runtime_images").Where("id = ? AND organization_id = ?", draft.Configuration.RuntimeImageID, agent.OrganizationID).Take(&runtime).Error; err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("load Runtime Image snapshot: %w", err)
	}
	if err := db.Table("configured_models").Where("id = ? AND organization_id = ?", draft.Configuration.ConfiguredModelID, agent.OrganizationID).Take(&model).Error; err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("load Configured Model snapshot: %w", err)
	}
	capabilities := make(map[string]bool)
	if err := json.Unmarshal(runtime.Capabilities, &capabilities); err != nil {
		return agentdomain.ReleaseDependencies{}, nil, fmt.Errorf("decode Runtime Image snapshot Capabilities: %w", err)
	}
	commands := make([]agentdomain.ReleaseQualityCommand, len(binding.QualityCommands))
	for index, command := range binding.QualityCommands {
		commands[index] = agentdomain.ReleaseQualityCommand{Name: command.Name, Kind: string(command.Kind), Executable: command.Executable, Arguments: append([]string(nil), command.Arguments...), TimeoutSeconds: command.TimeoutSeconds}
	}
	return agentdomain.ReleaseDependencies{
		RepositoryBinding: agentdomain.RepositoryBindingSnapshot{ID: binding.ID, Name: binding.Name, RepositorySSHURL: binding.RepositorySSHURL, DefaultBranch: binding.DefaultBranch, Instructions: binding.Instructions, QualityCommands: commands, EgressPolicy: binding.EgressPolicy.Mode, RequiredRuntimeCapabilities: append([]string(nil), binding.RequiredRuntimeCapabilities...)},
		RuntimeImage:      agentdomain.RuntimeImageSnapshot{ID: runtime.ID, Runtime: runtime.Runtime, CLIVersion: runtime.CLIVersion, AdapterVersion: runtime.Adapter, ImageDigest: runtime.ImageDigest, Capabilities: capabilities},
		ConfiguredModel:   agentdomain.ConfiguredModelSnapshot{ID: model.ID, Name: model.Name, ModelID: model.ModelID, Endpoint: model.Endpoint},
	}, errorsByField, nil
}

func lockRows(db *gorm.DB, table string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	var rows []struct{ ID string }
	return db.Table(table).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id IN ?", ids).Order("id").Find(&rows).Error
}
