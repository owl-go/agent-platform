package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"fmt"
	"gorm.io/gorm"
)

func portableExpertResources(tx *gorm.DB, item domain.Expert) (domain.Expert, error) {
	input := domain.ExpertInput{ConnectorDependencies: item.ConnectorDependencies, MCPServerIDs: item.MCPServerIDs, CLIConnectorDefinitionIDs: item.CLIConnectorDefinitionIDs}
	if err := portableConnectorBindings(tx, item.OwnerID, &input); err != nil {
		return item, err
	}
	if len(input.MCPServerIDs)+len(input.CLIConnectorDefinitionIDs) > 0 {
		return item, fmt.Errorf("%w: legacy Connectors need a portable package source", domain.ErrInvalid)
	}
	item.ConnectorDependencies = input.ConnectorDependencies
	item.MCPServerIDs = nil
	item.CLIConnectorDefinitionIDs = nil
	known := map[string]bool{}
	for _, skill := range item.BundledSkills {
		known[skill.ID] = true
	}
	for _, id := range item.SkillIDs {
		if known[id] {
			continue
		}
		var skill skillRecord
		if err := tx.Where("id=? AND owner_user_id IN (?)", id, accessibleResourceOwnerIDs(tx, item.OwnerID)).Take(&skill).Error; err != nil {
			return item, mapNotFound(err)
		}
		item.BundledSkills = append(item.BundledSkills, domain.SkillSnapshot{ID: skill.ID, Name: skill.Name, ObjectKey: skill.ObjectKey, SHA256: skill.SHA256})
	}
	item.SkillIDs = nil
	return item, nil
}
func (repository *Repository) ExportableExpert(ctx context.Context, owner, id string) (domain.Expert, error) {
	item, err := repository.GetExpert(ctx, owner, id)
	if err != nil {
		return item, err
	}
	return portableExpertResources(repository.db.WithContext(ctx), item)
}
func (repository *Repository) ExportableExpertTeam(ctx context.Context, owner, id string) (domain.ExpertTeam, error) {
	item, err := repository.GetExpertTeam(ctx, owner, id)
	if err != nil {
		return item, err
	}
	for i, member := range item.Members {
		item.Members[i].Expert, err = portableExpertResources(repository.db.WithContext(ctx), member.Expert)
		if err != nil {
			return item, err
		}
	}
	return item, nil
}
