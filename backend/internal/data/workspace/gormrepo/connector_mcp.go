package gormrepo

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

type connectorMCPManifest struct {
	Transport      string   `json:"transport"`
	URL            string   `json:"url,omitempty"`
	Runner         string   `json:"runner,omitempty"`
	Package        string   `json:"package,omitempty"`
	PackageVersion string   `json:"package_version,omitempty"`
	Arguments      []string `json:"arguments,omitempty"`
	Environment    []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"environment,omitempty"`
}

type connectorMCPPolicy struct {
	AuthMode string                `json:"auth_mode"`
	MCP      *connectorMCPManifest `json:"mcp"`
}

type connectorMCPConfig struct {
	Transport      string
	URL            string
	Runner         string
	Package        string
	PackageVersion string
	Arguments      []string
	Environment    []domain.EnvironmentVariable
}

func connectorMCPConfiguration(policy []byte) (connectorMCPConfig, string, error) {
	var parsed connectorMCPPolicy
	if err := json.Unmarshal(policy, &parsed); err != nil {
		return connectorMCPConfig{}, "", fmt.Errorf("decode Connector MCP policy: %w", err)
	}
	if parsed.AuthMode == "" || parsed.MCP == nil {
		return connectorMCPConfig{}, "", fmt.Errorf("Connector MCP policy is incomplete")
	}
	manifest := parsed.MCP
	if manifest.Transport != "streamable_http" && manifest.Transport != "stdio" {
		return connectorMCPConfig{}, "", fmt.Errorf("Connector MCP transport is unsupported")
	}
	configuration := connectorMCPConfig{Transport: manifest.Transport, URL: manifest.URL, Runner: manifest.Runner, Package: manifest.Package, PackageVersion: manifest.PackageVersion, Arguments: append([]string(nil), manifest.Arguments...)}
	for _, variable := range manifest.Environment {
		configuration.Environment = append(configuration.Environment, domain.EnvironmentVariable{Name: variable.Name, Value: variable.Value, Configured: true})
	}
	if parsed.AuthMode != "none" {
		found := false
		for index := range configuration.Environment {
			if configuration.Environment[index].Name == "MCP_BEARER_TOKEN" {
				configuration.Environment[index].Secret = true
				found = true
				break
			}
		}
		if !found {
			configuration.Environment = append(configuration.Environment, domain.EnvironmentVariable{Name: "MCP_BEARER_TOKEN", Secret: true, Configured: true})
		}
	}
	return configuration, parsed.AuthMode, nil
}

func connectorMCPServerSnapshot(tx *gorm.DB, ownerID, installationID string) (domain.MCPServerSnapshot, error) {
	var installation connectorInstallationRecord
	if err := tx.Where("id = ? AND owner_user_id = ? AND state = ?", installationID, ownerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
		return domain.MCPServerSnapshot{}, mapNotFound(err)
	}
	var revision connectorRevisionRecord
	if err := tx.Where("id = ? AND package_source = ? AND mode = ?", installation.ActiveRevisionID, installation.PackageSource, domain.ConnectorModeMCP).Take(&revision).Error; err != nil {
		return domain.MCPServerSnapshot{}, mapNotFound(err)
	}
	configuration, authMode, err := connectorMCPConfiguration(revision.RuntimePolicy)
	if err != nil {
		return domain.MCPServerSnapshot{}, err
	}
	var ciphertext []byte
	secretAAD := ""
	if authMode != "none" {
		if installation.AuthorizationID == nil {
			return domain.MCPServerSnapshot{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
		}
		var authorization connectorAuthorizationRecord
		if err := tx.Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", *installation.AuthorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error; err != nil {
			return domain.MCPServerSnapshot{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
		}
		ciphertext = append([]byte(nil), authorization.CredentialCiphertext...)
		secretAAD = "connector-authorization:" + ownerID
	}
	encoded, err := json.Marshal(map[string]any{"url": optionalConnectorString(configuration.URL), "runner": optionalConnectorString(configuration.Runner), "package": optionalConnectorString(configuration.Package), "package_version": optionalConnectorString(configuration.PackageVersion), "arguments": configuration.Arguments, "environment": configuration.Environment})
	if err != nil {
		return domain.MCPServerSnapshot{}, err
	}
	return domain.MCPServerSnapshot{ID: installation.ID, Name: installation.PackageSource, Icon: "plug", Transport: configuration.Transport, Configuration: encoded, SecretCiphertext: ciphertext, SecretOwnerID: ownerID, SecretAAD: secretAAD}, nil
}

func optionalConnectorString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
