package gormrepo

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"slices"

	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

// Copying a catalog Expert creates a Team-owned definition. A later source
// change is never an implicit upgrade of an existing stable member identity.
func ownTeamMembers(tx *gorm.DB, owner string, input domain.ExpertTeamInput, previous []domain.ExpertTeamMemberInput) ([]domain.ExpertTeamMemberInput, error) {
	old := make(map[string]domain.ExpertTeamMemberInput, len(previous))
	for _, member := range previous {
		old[member.ID] = member
	}
	result := make([]domain.ExpertTeamMemberInput, 0, len(input.Members))
	for _, member := range input.Members {
		if member.Definition == nil {
			if saved, ok := old[member.ID]; ok && saved.ExpertID == member.ExpertID && saved.Definition != nil {
				member.Definition, member.Skills = saved.Definition, saved.Skills
			} else {
				var row expertRecord
				if err := tx.Where("id=? AND owner_user_id IN (?)", member.ExpertID, accessibleResourceOwnerIDs(tx, owner)).Take(&row).Error; err != nil {
					return nil, mapNotFound(err)
				}
				expert, err := expertDomain(row)
				if err != nil {
					return nil, err
				}
				if !expert.Available() {
					return nil, fmt.Errorf("%w: source Expert is incomplete", domain.ErrInvalid)
				}
				definition := domain.ExpertInput{ConnectorDependencies: expert.ConnectorDependencies, StarterPrompts: expert.StarterPrompts, BundledSkills: expert.BundledSkills, Name: expert.Name, Icon: expert.Icon, IconBackground: expert.IconBackground, Introduction: expert.Introduction, Guidance: expert.Guidance, CoreCapability: expert.CoreCapability, OperatingProcedure: expert.OperatingProcedure, OutputStandard: expert.OutputStandard, Cautions: expert.Cautions, ExecutionInstruction: expert.ExecutionInstruction, MCPServerIDs: expert.MCPServerIDs, SkillIDs: expert.SkillIDs, CLIConnectorDefinitionIDs: expert.CLIConnectorDefinitionIDs}
				definition.Guidance = definition.MarkdownGuidance()
				member.Definition = &definition
				resources, err := loadExpertMemberSnapshot(tx, owner, row, 1)
				if err != nil {
					return nil, err
				}
				member.Skills = resources.Skills
			}
		}
		definition := *member.Definition
		if definition.BundledSkills == nil && member.ExpertID != "" {
			if _, exists := old[member.ID]; !exists {
				var source expertRecord
				if err := tx.Where("id=? AND owner_user_id IN (?)", member.ExpertID, accessibleResourceOwnerIDs(tx, owner)).Take(&source).Error; err != nil {
					return nil, mapNotFound(err)
				}
				if len(source.BundledSkills) > 0 {
					if err := json.Unmarshal(source.BundledSkills, &definition.BundledSkills); err != nil {
						return nil, err
					}
				}
			}
		}
		if definition.BundledSkills == nil {
			if saved, ok := old[member.ID]; ok && saved.Definition != nil {
				definition.BundledSkills = saved.Definition.BundledSkills
			}
		}
		if definition.Name == "" {
			definition.Name = member.Name
		}
		definition.Guidance = definition.MarkdownGuidance()
		definition.CoreCapability, definition.OperatingProcedure, definition.OutputStandard, definition.Cautions, definition.ExecutionInstruction = "", "", "", "", ""
		definition.ProviderModelID, definition.RuntimeEngine, definition.ExpertiseTags = "", "", nil
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		// Revalidate bindings as the maintaining Administrator; execution resolves
		// Connector permissions independently for the executing User.
		if err := validateExpertReferences(tx, owner, definition); err != nil {
			return nil, err
		}
		if err := portableConnectorBindings(tx, owner, &definition); err != nil {
			return nil, err
		}
		if len(member.Skills) == 0 && len(definition.SkillIDs) > 0 {
			if saved, ok := old[member.ID]; ok && saved.Definition != nil && slices.Equal(saved.Definition.SkillIDs, definition.SkillIDs) {
				member.Skills = append([]domain.SkillSnapshot(nil), saved.Skills...)
			}
			if len(member.Skills) == 0 {
				for _, id := range definition.SkillIDs {
					var skill skillRecord
					if err := tx.Where("id=? AND owner_user_id IN (?)", id, accessibleResourceOwnerIDs(tx, owner)).Take(&skill).Error; err != nil {
						return nil, mapNotFound(err)
					}
					member.Skills = append(member.Skills, domain.SkillSnapshot{ID: skill.ID, Name: skill.Name, ObjectKey: skill.ObjectKey, SHA256: skill.SHA256})
				}
			}
		}
		member.Definition = &definition
		result = append(result, member)
	}
	return result, nil
}

func ownedMemberID(team, member string) string {
	return uuid.NewSHA1(uuid.Nil, []byte(team+":"+member)).String()
}

func ownedMemberExpert(team expertTeamRecord, member domain.ExpertTeamMemberInput) domain.Expert {
	definition := member.Definition
	return domain.Expert{ConnectorDependencies: definition.ConnectorDependencies, StarterPrompts: definition.StarterPrompts, BundledSkills: mergeBundledSkills(member.Skills, definition.BundledSkills), ID: ownedMemberID(team.ID, member.ID), OwnerID: team.OwnerID, Platform: true, Name: definition.Name, Icon: defaultString(definition.Icon, "sparkles"), IconBackground: defaultString(definition.IconBackground, "sage"), Introduction: definition.Introduction, Guidance: definition.Guidance, MCPServerIDs: definition.MCPServerIDs, SkillIDs: definition.SkillIDs, CLIConnectorDefinitionIDs: definition.CLIConnectorDefinitionIDs, Version: team.Version}
}

func decodeTeamMembers(row expertTeamRecord) ([]domain.ExpertTeamMemberInput, error) {
	var members []domain.ExpertTeamMemberInput
	if err := json.Unmarshal(row.Members, &members); err != nil {
		return nil, fmt.Errorf("decode Team members: %w", err)
	}
	return members, nil
}

func loadTeamSelection(tx *gorm.DB, owner, id string) (expertTeamRecord, []domain.ExecutionStageSnapshot, error) {
	var team expertTeamRecord
	if err := tx.Where("id=? AND owner_user_id IN (?)", id, platformResourceOwnerIDs(tx)).Take(&team).Error; err != nil {
		return team, nil, mapNotFound(err)
	}
	members, err := decodeTeamMembers(team)
	if err != nil {
		return team, nil, err
	}
	input := domain.ExpertTeamInput{LeadMemberID: team.LeadMemberID, Members: members}
	if err := input.ValidateLead(); err != nil {
		return team, nil, err
	}
	if len(members) < 2 || len(members) > 10 {
		return team, nil, fmt.Errorf("%w: Team requires 2-10 members", domain.ErrInvalid)
	}
	defaults := make([]domain.ExecutionStageSnapshot, 0, len(members))
	for index, member := range members {
		if member.Definition == nil {
			return team, nil, fmt.Errorf("%w: upgrade the Team member definitions", domain.ErrInvalid)
		}
		definition := member.Definition
		servers, _ := marshal(definition.MCPServerIDs)
		connectors, _ := marshal(definition.CLIConnectorDefinitionIDs)
		skills, _ := marshal(definition.SkillIDs)
		if len(member.Skills) > 0 {
			skills = []byte("[]")
		}
		resources, err := loadExpertMemberSnapshot(tx, owner, expertRecord{StarterPrompts: jsonBytes(nonNilStrings(definition.StarterPrompts)), ConnectorDependencies: jsonBytes(nonNilDependencies(definition.ConnectorDependencies)), ID: ownedMemberID(team.ID, member.ID), Name: definition.Name, Icon: definition.Icon, IconBackground: definition.IconBackground, Introduction: definition.Introduction, Guidance: definition.Guidance, Version: team.Version, ExpertiseTags: []byte("[]"), MCPServerIDs: servers, SkillIDs: skills, CLIConnectorDefinitionIDs: connectors}, index+1)
		if err != nil {
			return team, nil, err
		}
		for _, connector := range resources.MCPServers {
			if len(connector.SecretCiphertext) > 0 && connector.SecretOwnerID != owner {
				return team, nil, fmt.Errorf("%w: resolve the Connector authorization for the executing User", domain.ErrInvalid)
			}
		}
		if len(member.Skills) > 0 {
			resources.Skills = append([]domain.SkillSnapshot(nil), member.Skills...)
		}
		resources.Skills = mergeBundledSkills(resources.Skills, definition.BundledSkills)
		expert := resources.ExpertSnapshot
		defaults = append(defaults, domain.ExecutionStageSnapshot{Position: index + 1, TeamMemberID: member.ID, TeamMemberName: member.Name, TeamMemberLabels: member.Labels, Expert: &expert, Skills: resources.Skills, MCPServers: resources.MCPServers, CLIConnectors: resources.CLIConnectors})
	}
	return team, defaults, nil
}

func mergeBundledSkills(existing, bundled []domain.SkillSnapshot) []domain.SkillSnapshot {
	result := append([]domain.SkillSnapshot(nil), existing...)
	seen := map[string]bool{}
	for _, item := range existing {
		seen[item.ID] = true
	}
	for _, item := range bundled {
		if !seen[item.ID] {
			result = append(result, item)
			seen[item.ID] = true
		}
	}
	return result
}
