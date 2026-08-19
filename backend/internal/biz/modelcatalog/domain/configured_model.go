package domain

import (
	"net/url"
	"strings"
	"time"
)

type ConfiguredModel struct {
	ID                  string
	OrganizationID      string
	Name                string
	ModelID             string
	Endpoint            string
	CredentialProfileID string
	Enabled             bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
}

type ModelRegistration struct {
	ID             string
	OrganizationID string
	Name           string
	ModelID        string
	Endpoint       string
	Credential     CredentialProfile
	Now            time.Time
}

func RegisterModel(registration ModelRegistration) (ConfiguredModel, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.OrganizationID) == "" || strings.TrimSpace(registration.Name) == "" || strings.TrimSpace(registration.ModelID) == "" {
		return ConfiguredModel{}, invalidf("Configured Model ID, Organization ID, name, and model ID are required")
	}
	if err := validateEndpoint(registration.Endpoint); err != nil {
		return ConfiguredModel{}, err
	}
	credential := registration.Credential
	if credential.ID == "" || credential.OrganizationID != registration.OrganizationID || credential.TeamID != nil || credential.Kind != ModelCredential || !credential.Enabled() {
		return ConfiguredModel{}, invalidf("Configured Model requires an enabled Organization-scoped model Credential Profile")
	}
	if registration.Now.IsZero() {
		return ConfiguredModel{}, invalidf("registration time is required")
	}
	now := registration.Now.UTC()
	return ConfiguredModel{
		ID: registration.ID, OrganizationID: registration.OrganizationID, Name: strings.TrimSpace(registration.Name),
		ModelID: strings.TrimSpace(registration.ModelID), Endpoint: registration.Endpoint,
		CredentialProfileID: credential.ID, Enabled: true, CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreModel(id, organizationID, name, modelID, endpoint, credentialProfileID string, enabled bool, createdAt, updatedAt time.Time, version int64) (ConfiguredModel, error) {
	if id == "" || organizationID == "" || name == "" || modelID == "" || credentialProfileID == "" || createdAt.IsZero() || updatedAt.IsZero() || version <= 0 {
		return ConfiguredModel{}, invalidf("invalid persisted Configured Model")
	}
	if err := validateEndpoint(endpoint); err != nil {
		return ConfiguredModel{}, err
	}
	return ConfiguredModel{
		ID: id, OrganizationID: organizationID, Name: name, ModelID: modelID, Endpoint: endpoint,
		CredentialProfileID: credentialProfileID, Enabled: enabled, CreatedAt: createdAt, UpdatedAt: updatedAt, Version: version,
	}, nil
}

func (model *ConfiguredModel) SetEnabled(enabled bool, credential CredentialProfile, now time.Time) error {
	if model.Enabled == enabled {
		return nil
	}
	if enabled && (credential.ID != model.CredentialProfileID || credential.OrganizationID != model.OrganizationID || credential.TeamID != nil || credential.Kind != ModelCredential || !credential.Enabled()) {
		return invalidf("Configured Model cannot be enabled without its enabled Organization-scoped model Credential Profile")
	}
	if now.IsZero() {
		return invalidf("status change time is required")
	}
	model.Enabled = enabled
	model.UpdatedAt = now.UTC()
	model.Version++
	return nil
}

func validateEndpoint(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return invalidf("Configured Model Endpoint must be an absolute HTTPS URL without user information")
	}
	return nil
}
