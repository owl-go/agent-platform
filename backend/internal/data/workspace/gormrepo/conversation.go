package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ application.ConversationRepository = (*Repository)(nil)

type conversationSelectionRecord struct {
	ID        string    `gorm:"column:id"`
	OwnerID   string    `gorm:"column:owner_user_id"`
	SessionID *string   `gorm:"column:session_id"`
	RunID     *string   `gorm:"column:run_id"`
	Snapshot  []byte    `gorm:"column:snapshot;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (conversationSelectionRecord) TableName() string { return "conversation_selections" }

func (repository *Repository) GetConversationSelection(ctx context.Context, owner string, scope domain.ConversationScope) (domain.ConversationSelection, error) {
	var result domain.ConversationSelection
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = currentConversationSelection(tx, owner, scope)
		return err
	})
	return result, err
}

func currentConversationSelection(tx *gorm.DB, owner string, scope domain.ConversationScope) (domain.ConversationSelection, error) {
	if err := scope.Validate(); err != nil {
		return domain.ConversationSelection{}, err
	}
	var selection domain.ConversationSelection
	var encoded []byte
	if scope.SessionID != "" {
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", scope.SessionID, owner).Take(&session).Error; err != nil {
			return selection, mapNotFound(err)
		}
		if session.SelectionID != nil {
			return readConversationSelection(tx, owner, scope, *session.SelectionID)
		}
		encoded = session.ExpertSnapshot
		if session.ExpertID != nil {
			selection.ExpertID = *session.ExpertID
		}
		if session.ExpertTeamID != nil {
			selection.ExpertTeamID = *session.ExpertTeamID
		}
	} else {
		var run runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND conversation_id = id AND workflow_id = ? AND owner_user_id = ?", scope.RunID, scope.WorkflowID, owner).Take(&run).Error; err != nil {
			return selection, mapNotFound(err)
		}
		if run.SelectionID != nil {
			return readConversationSelection(tx, owner, scope, *run.SelectionID)
		}
		encoded = run.WorkflowSnapshot
	}
	if len(encoded) > 0 && string(encoded) != "null" {
		var plan domain.ExecutionSnapshot
		if err := json.Unmarshal(encoded, &plan); err != nil {
			return selection, err
		}
		stages, err := plan.OrderedStages()
		if err != nil {
			return selection, err
		}
		selection.Defaults = stages
		if len(stages) == 1 && stages[0].Expert != nil {
			selection.ExpertID = stages[0].Expert.ID
			selection.Name, selection.Icon, selection.IconBackground = stages[0].Expert.Name, stages[0].Expert.Icon, stages[0].Expert.IconBackground
		} else if len(stages) > 1 {
			names := make([]string, 0, len(stages))
			for _, stage := range stages {
				names = append(names, stage.TeamMemberName)
			}
			selection.Name = strings.Join(names, " / ")
			if plan.TeamProfile != nil {
				selection.ExpertTeamID = plan.TeamProfile.ID
				selection.Name, selection.Icon, selection.IconBackground = plan.TeamProfile.Name, plan.TeamProfile.Icon, plan.TeamProfile.IconBackground
			}
		}
	} else if err := loadConversationSpecialist(tx, owner, &selection); err != nil {
		return selection, err
	}
	if err := saveConversationSelection(tx, owner, scope, &selection, true); err != nil {
		return selection, err
	}
	return selection, nil
}

func readConversationSelection(tx *gorm.DB, owner string, scope domain.ConversationScope, id string) (domain.ConversationSelection, error) {
	var row conversationSelectionRecord
	query := tx.Where("owner_user_id = ? AND id = ?", owner, id)
	if scope.SessionID != "" {
		query = query.Where("session_id = ?", scope.SessionID)
	} else {
		query = query.Where("run_id = ?", scope.RunID)
	}
	if err := query.Take(&row).Error; err != nil {
		return domain.ConversationSelection{}, mapNotFound(err)
	}
	var result domain.ConversationSelection
	if err := json.Unmarshal(row.Snapshot, &result); err != nil {
		return result, fmt.Errorf("decode conversation selection: %w", err)
	}
	result.ID = row.ID
	return result, nil
}

func saveConversationSelection(tx *gorm.DB, owner string, scope domain.ConversationScope, selection *domain.ConversationSelection, current bool) error {
	selection.ID = uuid.NewString()
	encoded, err := marshal(selection)
	if err != nil {
		return err
	}
	row := conversationSelectionRecord{ID: selection.ID, OwnerID: owner, Snapshot: encoded}
	if scope.SessionID != "" {
		row.SessionID = &scope.SessionID
	} else {
		row.RunID = &scope.RunID
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	if !current {
		return nil
	}
	if scope.SessionID != "" {
		return tx.Model(&sessionRecord{}).Where("id = ? AND owner_user_id = ?", scope.SessionID, owner).Update("selection_id", selection.ID).Error
	}
	return tx.Model(&runRecord{}).Where("id = ? AND owner_user_id = ? AND workflow_id = ?", scope.RunID, owner, scope.WorkflowID).Update("selection_id", selection.ID).Error
}

func (repository *Repository) ResolveConversationSelection(ctx context.Context, owner string, scope domain.ConversationScope, input domain.ConversationSelectionInput) (domain.ConversationSelection, error) {
	var selected domain.ConversationSelection
	if len(input.SkillIDs)+len(input.MCPServerIDs)+len(input.CLIConnectorIDs)+len(input.DisabledConnectors) > 100 {
		return selected, fmt.Errorf("%w: too many resources", domain.ErrInvalid)
	}
	if input.ExpertID != "" && input.ExpertTeamID != "" {
		return selected, fmt.Errorf("%w: choose one specialist", domain.ErrInvalid)
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockConversationScope(tx, owner, scope); err != nil {
			return err
		}
		var err error
		if input.PreviousID != "" {
			selected, err = readConversationSelection(tx, owner, scope, input.PreviousID)
		} else if !input.ChangeExpert {
			selected, err = currentConversationSelection(tx, owner, scope)
		}
		if err != nil {
			return err
		}
		if input.ChangeExpert {
			selected.ExpertID, selected.ExpertTeamID = input.ExpertID, input.ExpertTeamID
			if err := loadConversationSpecialist(tx, owner, &selected); err != nil {
				return err
			}
		}
		// Reuse frozen resources still explicitly selected; only new/reselected IDs
		// resolve current catalog revisions.
		keptSkills := map[string]domain.SkillSnapshot{}
		for _, item := range selected.Skills {
			keptSkills[item.ID] = item
		}
		keptMCP := map[string]domain.MCPServerSnapshot{}
		for _, item := range selected.MCPServers {
			keptMCP[item.ID] = item
		}
		keptCLI := map[string]domain.CLIConnectorSnapshot{}
		for _, item := range selected.CLIConnectors {
			keptCLI[item.ID] = item
		}
		for _, id := range input.RefreshIDs {
			delete(keptSkills, strings.TrimPrefix(id, "skill:"))
			delete(keptMCP, strings.TrimPrefix(id, "mcp:"))
			delete(keptCLI, strings.TrimPrefix(id, "cli:"))
		}
		var freshSkills, freshMCP, freshCLI []string
		selected.Skills, selected.MCPServers, selected.CLIConnectors = nil, nil, nil
		for _, id := range uniqueStrings(input.SkillIDs) {
			if item, ok := keptSkills[id]; ok {
				selected.Skills = append(selected.Skills, item)
			} else {
				freshSkills = append(freshSkills, id)
			}
		}
		for _, id := range uniqueStrings(input.MCPServerIDs) {
			if item, ok := keptMCP[id]; ok {
				selected.MCPServers = append(selected.MCPServers, item)
			} else {
				freshMCP = append(freshMCP, id)
			}
		}
		for _, id := range uniqueStrings(input.CLIConnectorIDs) {
			if item, ok := keptCLI[id]; ok {
				selected.CLIConnectors = append(selected.CLIConnectors, item)
			} else {
				freshCLI = append(freshCLI, id)
			}
		}
		resource, err := loadConversationResources(tx, owner, freshSkills, freshMCP, freshCLI)
		if err != nil {
			return err
		}
		selected.Skills = append(selected.Skills, resource.Skills...)
		selected.MCPServers = append(selected.MCPServers, resource.MCPServers...)
		selected.CLIConnectors = append(selected.CLIConnectors, resource.CLIConnectors...)
		selected.DisabledConnectors = uniqueStrings(input.DisabledConnectors)
		return saveConversationSelection(tx, owner, scope, &selected, false)
	})
	return selected, err
}

func lockConversationScope(tx *gorm.DB, owner string, scope domain.ConversationScope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	var row struct{ ID string }
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("owner_user_id = ?", owner)
	if scope.SessionID != "" {
		query = query.Table("sessions").Where("id = ?", scope.SessionID)
	} else {
		query = query.Table("runs").Where("id = ? AND conversation_id = id AND workflow_id = ?", scope.RunID, scope.WorkflowID)
	}
	return mapNotFound(query.Take(&row).Error)
}

func uniqueStrings(items []string) []string {
	var result []string
	seen := map[string]bool{}
	for _, id := range items {
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result
}

func loadConversationResources(tx *gorm.DB, owner string, skills, mcp, cli []string) (domain.ExpertMemberSnapshot, error) {
	encodedSkills, _ := marshal(skills)
	encodedMCP, _ := marshal(mcp)
	encodedCLI, _ := marshal(cli)
	return loadExpertMemberSnapshot(tx, owner, expertRecord{Introduction: "selection", CoreCapability: "selection", OperatingProcedure: "selection", OutputStandard: "selection", ExpertiseTags: []byte("[]"), SkillIDs: encodedSkills, MCPServerIDs: encodedMCP, CLIConnectorDefinitionIDs: encodedCLI}, 1)
}

func loadConversationSpecialist(tx *gorm.DB, owner string, selection *domain.ConversationSelection) error {
	selection.Defaults = nil
	selection.Name = ""
	selection.Icon = ""
	selection.IconBackground = ""
	if selection.ExpertID == "" && selection.ExpertTeamID == "" {
		return nil
	}
	var members []domain.ExpertTeamMemberInput
	if selection.ExpertTeamID != "" {
		var team expertTeamRecord
		if err := tx.Where("owner_user_id = ? AND id = ?", owner, selection.ExpertTeamID).Take(&team).Error; err != nil {
			return mapNotFound(err)
		}
		if err := json.Unmarshal(team.Members, &members); err != nil {
			return err
		}
		if len(members) < 2 || len(members) > 10 {
			return fmt.Errorf("%w: Expert Team requires 2-10 members", domain.ErrInvalid)
		}
		selection.Name, selection.Icon, selection.IconBackground = team.Name, team.Icon, team.IconBackground
	} else {
		members = []domain.ExpertTeamMemberInput{{ExpertID: selection.ExpertID}}
	}
	for index, member := range members {
		var expert expertRecord
		if err := tx.Where("owner_user_id IN (?) AND id = ? AND introduction <> '' AND core_capability <> '' AND operating_procedure <> '' AND output_standard <> ''", accessibleResourceOwnerIDs(tx, owner), member.ExpertID).Take(&expert).Error; err != nil {
			return fmt.Errorf("%w: Expert is incomplete or unavailable", domain.ErrInvalid)
		}
		resources, err := loadExpertMemberSnapshot(tx, owner, expert, index+1)
		if err != nil {
			return err
		}
		frozen := resources.ExpertSnapshot
		selection.Defaults = append(selection.Defaults, domain.ExecutionStageSnapshot{Position: index + 1, Expert: &frozen, TeamMemberID: member.ID, TeamMemberName: member.Name, TeamMemberLabels: member.Labels, Skills: resources.Skills, MCPServers: resources.MCPServers, CLIConnectors: resources.CLIConnectors})
		if selection.ExpertID != "" {
			selection.Name, selection.Icon, selection.IconBackground = expert.Name, expert.Icon, expert.IconBackground
		}
	}
	return nil
}

func retainConversationSelection(tx *gorm.DB, owner string, scope domain.ConversationScope, selection domain.ConversationSelection) error {
	selection.Skills = nil
	return saveConversationSelection(tx, owner, scope, &selection, true)
}
