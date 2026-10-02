package gormrepo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/connectorpackage"
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
	EgressHosts    []string `json:"egress_hosts"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	Limits         struct {
		CPU            int `json:"cpu_millis"`
		MemoryMiB      int `json:"memory_mib"`
		Concurrency    int `json:"concurrency"`
		ChildProcesses int `json:"child_processes"`
	} `json:"resource_limits,omitempty"`
}

type connectorMCPPolicy struct {
	LegacyProjection bool                  `json:"legacy_projection"`
	AuthMode         string                `json:"auth_mode"`
	MCP              *connectorMCPManifest `json:"mcp"`
}

type connectorMCPConfig struct {
	Transport      string
	URL            string
	Runner         string
	Package        string
	PackageVersion string
	Arguments      []string
	Environment    []domain.EnvironmentVariable
	EgressHosts    []string
	TimeoutSeconds int
	CPUMillis      int
	MemoryMiB      int
	ChildProcesses int
}

func connectorMCPConfiguration(policy []byte) (connectorMCPConfig, string, error) {
	var parsed connectorMCPPolicy
	if err := json.Unmarshal(policy, &parsed); err != nil {
		return connectorMCPConfig{}, "", fmt.Errorf("decode Connector MCP policy: %w", err)
	}
	if parsed.AuthMode == "" || parsed.MCP == nil {
		return connectorMCPConfig{}, "", fmt.Errorf("Connector MCP policy is incomplete")
	}
	if parsed.LegacyProjection {
		return connectorMCPConfig{}, "", domain.ErrNotFound
	}
	manifest := parsed.MCP
	if manifest.Transport != "streamable_http" && manifest.Transport != "stdio" {
		return connectorMCPConfig{}, "", fmt.Errorf("Connector MCP transport is unsupported")
	}
	configuration := connectorMCPConfig{Transport: manifest.Transport, URL: manifest.URL, Runner: manifest.Runner, Package: manifest.Package, PackageVersion: manifest.PackageVersion, Arguments: append([]string(nil), manifest.Arguments...), EgressHosts: append([]string(nil), manifest.EgressHosts...), TimeoutSeconds: manifest.TimeoutSeconds, CPUMillis: manifest.Limits.CPU, MemoryMiB: manifest.Limits.MemoryMiB, ChildProcesses: manifest.Limits.ChildProcesses}
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
		secretAAD = authorization.CredentialAAD
		if secretAAD == "" {
			secretAAD = "connector-authorization:" + ownerID
		}
	}
	encoded, err := json.Marshal(map[string]any{"url": optionalConnectorString(configuration.URL), "runner": optionalConnectorString(configuration.Runner), "package": optionalConnectorString(configuration.Package), "package_version": optionalConnectorString(configuration.PackageVersion), "arguments": configuration.Arguments, "environment": configuration.Environment, "egress_hosts": configuration.EgressHosts, "timeout_seconds": configuration.TimeoutSeconds, "resource_limits": map[string]any{"cpu_millis": configuration.CPUMillis, "memory_mib": configuration.MemoryMiB, "child_processes": configuration.ChildProcesses}})
	if err != nil {
		return domain.MCPServerSnapshot{}, err
	}
	return domain.MCPServerSnapshot{ID: installation.ID, Name: installation.PackageSource, Icon: connectorpackage.DisplayIcon(installation.PackageSource), PackageObjectKey: revision.ObjectKey, PackageSHA256: revision.PackageSHA256, Transport: configuration.Transport, Configuration: encoded, SecretCiphertext: ciphertext, SecretOwnerID: ownerID, SecretAAD: secretAAD}, nil
}

func connectorMCPServerCatalog(tx *gorm.DB, ownerID string, installation connectorInstallationRecord) (domain.MCPServer, error) {
	var revision connectorRevisionRecord
	if err := tx.Where("id = ? AND package_source = ? AND mode = ?", installation.ActiveRevisionID, installation.PackageSource, domain.ConnectorModeMCP).Take(&revision).Error; err != nil {
		return domain.MCPServer{}, mapNotFound(err)
	}
	configuration, authMode, err := connectorMCPConfiguration(revision.RuntimePolicy)
	if err != nil {
		return domain.MCPServer{}, err
	}
	tested := installation.State == string(domain.ConnectorInstallationActive)
	testError := ""
	if authMode != "none" {
		tested = false
		if installation.AuthorizationID != nil {
			var authorization connectorAuthorizationRecord
			if err := tx.Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", *installation.AuthorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error; err == nil {
				tested = installation.State == string(domain.ConnectorInstallationActive)
			} else if err != gorm.ErrRecordNotFound {
				return domain.MCPServer{}, err
			}
		}
		if !tested {
			testError = "authorization required"
		}
	}
	return domain.MCPServer{ID: installation.ID, OwnerID: ownerID, Name: installation.PackageSource, Icon: connectorpackage.DisplayIcon(installation.PackageSource), Transport: configuration.Transport, URL: optionalConnectorString(configuration.URL), Runner: optionalConnectorString(configuration.Runner), Package: optionalConnectorString(configuration.Package), PackageVersion: optionalConnectorString(configuration.PackageVersion), Arguments: configuration.Arguments, Environment: configuration.Environment, TestError: testError, TestedAt: func() *time.Time {
		if tested {
			now := time.Now().UTC()
			return &now
		}
		return nil
	}(), UpdatedAt: installation.UpdatedAt, Version: installation.Version, ManagedInstallation: true}, nil
}

func optionalConnectorString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
