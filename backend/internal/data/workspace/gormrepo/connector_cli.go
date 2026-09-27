package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/connectorpackage"
	"gorm.io/gorm"
)

type connectorCLIPolicy struct {
	LegacyProjection bool                          `json:"legacy_projection"`
	AuthMode         string                        `json:"auth_mode"`
	Metadata         connectorpackage.Metadata     `json:"metadata"`
	CLI              *connectorpackage.CLIManifest `json:"cli"`
	BundleObjectKey  string                        `json:"cli_bundle_object_key"`
	BundleSHA256     string                        `json:"cli_bundle_sha256"`
}

func connectorCLIServerSnapshot(tx *gorm.DB, ownerID, installationID string) (domain.CLIConnectorSnapshot, error) {
	var installation connectorInstallationRecord
	if err := tx.Where("id = ? AND owner_user_id = ? AND state = ?", installationID, ownerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
		return domain.CLIConnectorSnapshot{}, mapNotFound(err)
	}
	var revision connectorRevisionRecord
	if err := tx.Where("id = ? AND package_source = ? AND mode = ?", installation.ActiveRevisionID, installation.PackageSource, domain.ConnectorModeCLI).Take(&revision).Error; err != nil {
		return domain.CLIConnectorSnapshot{}, mapNotFound(err)
	}
	var policy connectorCLIPolicy
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.CLI == nil {
		if err == nil {
			err = fmt.Errorf("Connector CLI policy is incomplete")
		}
		return domain.CLIConnectorSnapshot{}, err
	}
	if policy.LegacyProjection {
		return domain.CLIConnectorSnapshot{}, domain.ErrNotFound
	}
	if policy.AuthMode != "none" {
		if installation.AuthorizationID == nil {
			return domain.CLIConnectorSnapshot{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
		}
		var authorization connectorAuthorizationRecord
		if err := tx.Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", *installation.AuthorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error; err != nil {
			return domain.CLIConnectorSnapshot{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
		}
	}
	if policy.BundleObjectKey == "" || len(policy.BundleSHA256) != 64 || policy.CLI.Executable == "" || len(policy.CLI.Capabilities) == 0 {
		return domain.CLIConnectorSnapshot{}, fmt.Errorf("%w: Connector CLI package has no verified bundle or reviewed capability", domain.ErrInvalid)
	}
	capabilities, err := connectorCLICapabilities(policy.CLI.Capabilities)
	if err != nil {
		return domain.CLIConnectorSnapshot{}, err
	}
	bundlePath := policy.CLI.BundlePath
	if bundlePath == "" {
		bundlePath = "bin/" + policy.CLI.Executable
	}
	authenticationDriver := policy.CLI.AuthenticationDriver
	if authenticationDriver == "" {
		authenticationDriver = "none"
		if policy.AuthMode != "none" {
			authenticationDriver = "connector_package"
		}
	}
	authorizationID := ""
	if installation.AuthorizationID != nil {
		authorizationID = *installation.AuthorizationID
	}
	name := policy.Metadata.Name
	if name == "" {
		name = installation.PackageSource
	}
	return domain.CLIConnectorSnapshot{ID: installation.ID, Name: name, Icon: connectorpackage.DisplayIcon(installation.PackageSource), Executable: policy.CLI.Executable, ExecutablePath: bundlePath, AuthenticationDriver: authenticationDriver, InstallationID: installation.ID, RevisionID: revision.ID, AuthorizationID: authorizationID, PackageSHA256: revision.PackageSHA256, PackageObjectKey: revision.ObjectKey, CPUMillis: policy.CLI.Limits.CPU, MemoryMiB: policy.CLI.Limits.MemoryMiB, ChildProcesses: policy.CLI.Limits.ChildProcesses, BundleObjectKey: policy.BundleObjectKey, BundleSHA256: policy.BundleSHA256, RuntimeDigests: []string{policy.CLI.Runtime.Digest}, Capabilities: capabilities, Version: installation.Version}, nil
}

func connectorCLICapabilities(items []connectorpackage.CLICapability) (json.RawMessage, error) {
	capabilities := make([]cliconnector.Capability, 0, len(items))
	for _, item := range items {
		identities := make([]cliconnector.Identity, 0, len(item.Identities))
		for _, identity := range item.Identities {
			identities = append(identities, cliconnector.Identity(identity))
		}
		capabilities = append(capabilities, cliconnector.Capability{ID: item.ID, ArgvPrefix: append([]string(nil), item.ArgvPrefix...), Risk: cliconnector.Risk(item.Risk), Identities: identities, Scopes: append([]string(nil), item.Scopes...), EgressHosts: append([]string(nil), item.EgressHosts...), Timeout: time.Duration(item.TimeoutSeconds) * time.Second})
	}
	encoded, err := json.Marshal(capabilities)
	return encoded, err
}

func (repository *Repository) ListConnectorPackageCLIDefinitions(ctx context.Context, ownerID string) ([]cliconnector.Definition, error) {
	var installations []connectorInstallationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND state <> ?", ownerID, domain.ConnectorInstallationUninstalled).Order("updated_at DESC, id DESC").Find(&installations).Error; err != nil {
		return nil, fmt.Errorf("list Connector CLI installations: %w", err)
	}
	items := make([]cliconnector.Definition, 0, len(installations))
	for _, installation := range installations {
		var revision connectorRevisionRecord
		if err := repository.db.WithContext(ctx).Where("id = ? AND package_source = ? AND mode = ?", installation.ActiveRevisionID, installation.PackageSource, domain.ConnectorModeCLI).Take(&revision).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return nil, err
		}
		var policy connectorCLIPolicy
		if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.CLI == nil {
			return nil, fmt.Errorf("decode Connector CLI policy: %w", err)
		}
		if policy.LegacyProjection {
			continue
		}
		capabilities, err := connectorCLICapabilities(policy.CLI.Capabilities)
		if err != nil {
			return nil, err
		}
		var parsed []cliconnector.Capability
		if err := json.Unmarshal(capabilities, &parsed); err != nil {
			return nil, err
		}
		authorized := policy.AuthMode == "none"
		if policy.AuthMode != "none" && installation.AuthorizationID != nil {
			var authorization connectorAuthorizationRecord
			authorized = repository.db.WithContext(ctx).Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", *installation.AuthorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error == nil
		}
		authenticationDriver := policy.CLI.AuthenticationDriver
		if authenticationDriver == "" {
			authenticationDriver = "none"
			if policy.AuthMode != "none" {
				authenticationDriver = "connector_package"
			}
		}
		state := cliconnector.StateAvailable
		failureReason := ""
		if installation.State != string(domain.ConnectorInstallationActive) {
			state, failureReason = cliconnector.StateDisabled, "connector installation is disabled"
		}
		if policy.BundleObjectKey == "" || len(policy.BundleSHA256) != 64 || len(parsed) == 0 {
			state, failureReason = cliconnector.StateDisabled, "connector package has no verified executable bundle or reviewed capability"
		}
		name := policy.Metadata.Name
		if name == "" {
			name = installation.PackageSource
		}
		items = append(items, cliconnector.Definition{ID: installation.ID, Name: name, Icon: connectorpackage.DisplayIcon(installation.PackageSource), Package: installation.PackageSource, Version: revision.Version, Executable: policy.CLI.Executable, AuthenticationDriver: authenticationDriver, State: state, FailureReason: failureReason, BundleObjectKey: policy.BundleObjectKey, BundleSHA256: policy.BundleSHA256, RuntimeDigests: []string{policy.CLI.Runtime.Digest}, Capabilities: parsed, VersionNumber: installation.Version, ManagedInstallation: true, InstallationAuthorized: authorized, CPUMillis: policy.CLI.Limits.CPU, MemoryMiB: policy.CLI.Limits.MemoryMiB, ChildProcesses: policy.CLI.Limits.ChildProcesses})
	}
	return items, nil
}
