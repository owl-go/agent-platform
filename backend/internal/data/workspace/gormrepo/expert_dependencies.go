package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
)

// Resolve only the executing User's installation; a declaration is never a grant.
func resolveExpertDependencies(tx *gorm.DB, owner string, dependencies []domain.ExpertConnectorDependency) (mcp, cli []string, err error) {
	for _, dep := range dependencies {
		var installed connectorInstallationRecord
		if err = tx.Where("owner_user_id=? AND package_source=? AND state=?", owner, dep.Source, domain.ConnectorInstallationActive).Take(&installed).Error; err != nil {
			return nil, nil, fmt.Errorf("%w: install and authorize the required Connector", domain.ErrInvalid)
		}
		var revision connectorRevisionRecord
		if err = tx.Where("id=? AND package_source=? AND mode=?", installed.ActiveRevisionID, dep.Source, dep.Kind).Take(&revision).Error; err != nil {
			return nil, nil, fmt.Errorf("%w: Connector type is unavailable", domain.ErrInvalid)
		}
		if dep.Version != "" && revision.Version != dep.Version {
			return nil, nil, fmt.Errorf("%w: required Connector version is unavailable", domain.ErrInvalid)
		}
		if dep.Kind == "mcp" {
			if _, err = connectorMCPServerSnapshot(tx, owner, installed.ID); err != nil {
				return nil, nil, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrInvalid)
			}
			mcp = append(mcp, installed.ID)
		} else {
			if _, err = connectorCLIServerSnapshot(tx, owner, installed.ID); err != nil {
				return nil, nil, fmt.Errorf("%w: Connector authorization or conformance is unavailable", domain.ErrInvalid)
			}
			cli = append(cli, installed.ID)
		}
	}
	return
}
func (repository *Repository) ExpertDependenciesAvailable(ctx context.Context, owner string, expert domain.Expert) error {
	_, _, err := resolveExpertDependencies(repository.db.WithContext(ctx), owner, expert.ConnectorDependencies)
	return err
}
func portableConnectorBindings(tx *gorm.DB, owner string, input *domain.ExpertInput) error {
	for _, kind := range []string{"mcp", "cli"} {
		ids := input.MCPServerIDs
		if kind == "cli" {
			ids = input.CLIConnectorDefinitionIDs
		}
		retained := []string{}
		for _, id := range ids {
			var installation connectorInstallationRecord
			err := tx.Where("id=? AND owner_user_id=?", id, owner).Take(&installation).Error
			if err == gorm.ErrRecordNotFound {
				retained = append(retained, id)
				continue
			}
			if err != nil {
				return err
			}
			var revision connectorRevisionRecord
			if err := tx.Where("id=? AND mode=?", installation.ActiveRevisionID, kind).Take(&revision).Error; err != nil {
				return err
			}
			input.ConnectorDependencies = append(input.ConnectorDependencies, domain.ExpertConnectorDependency{Source: installation.PackageSource, Kind: kind, Version: revision.Version})
		}
		if kind == "mcp" {
			input.MCPServerIDs = retained
		} else {
			input.CLIConnectorDefinitionIDs = retained
		}
	}
	return domain.ValidateExpertDependencies(input.ConnectorDependencies)
}
func decodeExpertDependencies(data []byte) []domain.ExpertConnectorDependency {
	var out []domain.ExpertConnectorDependency
	_ = json.Unmarshal(data, &out)
	return out
}
