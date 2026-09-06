package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type ConversationScope struct {
	SessionID  string
	WorkflowID string
	RunID      string
}

func (scope ConversationScope) Validate() error {
	if (scope.SessionID != "" && scope.RunID == "" && scope.WorkflowID == "") || (scope.SessionID == "" && scope.RunID != "" && scope.WorkflowID != "") {
		return nil
	}
	return fmt.Errorf("%w: select one conversation", ErrInvalid)
}

// ConversationSelection is stored server-side. Clients receive only its opaque
// identity and display metadata, never frozen Connector configuration or secrets.
type ConversationSelection struct {
	ID                 string                   `json:"id"`
	ExpertID           string                   `json:"expert_id"`
	ExpertTeamID       string                   `json:"expert_team_id"`
	Name               string                   `json:"name"`
	Icon               string                   `json:"icon"`
	IconBackground     string                   `json:"icon_background"`
	Defaults           []ExecutionStageSnapshot `json:"defaults"`
	Skills             []SkillSnapshot          `json:"skills"`
	MCPServers         []MCPServerSnapshot      `json:"mcp_servers"`
	CLIConnectors      []CLIConnectorSnapshot   `json:"cli_connectors"`
	DisabledConnectors []string                 `json:"disabled_connectors"`
}

type ConversationSelectionInput struct {
	PreviousID         string
	ChangeExpert       bool
	ExpertID           string
	ExpertTeamID       string
	SkillIDs           []string
	MCPServerIDs       []string
	CLIConnectorIDs    []string
	DisabledConnectors []string
	RefreshIDs         []string
}

func (selection ConversationSelection) Apply(configuration ExecutionStageSnapshot) []ExecutionStageSnapshot {
	defaults := selection.Defaults
	if len(defaults) == 0 {
		defaults = []ExecutionStageSnapshot{{}}
	}
	stages := make([]ExecutionStageSnapshot, 0, len(defaults))
	for index, base := range defaults {
		stage := configuration
		stage.Position, stage.Expert = index+1, base.Expert
		stage.TeamMemberID, stage.TeamMemberName, stage.TeamMemberLabels = base.TeamMemberID, base.TeamMemberName, base.TeamMemberLabels
		stage.Skills = mergeSelectionResources(base.Skills, selection.Skills, func(item SkillSnapshot) string { return item.ID })
		stage.MCPServers = mergeSelectionResources(base.MCPServers, selection.MCPServers, func(item MCPServerSnapshot) string { return item.ID })
		stage.CLIConnectors = mergeSelectionResources(base.CLIConnectors, selection.CLIConnectors, func(item CLIConnectorSnapshot) string { return item.ID })
		stage.MCPServers = excludeSelectionResources(stage.MCPServers, selection.DisabledConnectors, func(item MCPServerSnapshot) string { return "mcp:" + item.ID })
		stage.CLIConnectors = excludeSelectionResources(stage.CLIConnectors, selection.DisabledConnectors, func(item CLIConnectorSnapshot) string { return "cli:" + item.ID })
		identity, _ := json.Marshal(struct {
			Expert *ExpertSnapshot
			Member string
			Skills []SkillSnapshot
			MCP    []MCPServerSnapshot
			CLI    []CLIConnectorSnapshot
		}{stage.Expert, stage.TeamMemberID, stage.Skills, stage.MCPServers, stage.CLIConnectors})
		digest := sha256.Sum256(identity)
		stage.SelectionKey = hex.EncodeToString(digest[:])
		stages = append(stages, stage)
	}
	return stages
}

func mergeSelectionResources[T any](defaults, explicit []T, identity func(T) string) []T {
	result := append([]T(nil), defaults...)
	indexes := map[string]int{}
	for index, item := range result {
		indexes[identity(item)] = index
	}
	for _, item := range explicit {
		if index, exists := indexes[identity(item)]; exists {
			result[index] = item
		} else {
			indexes[identity(item)] = len(result)
			result = append(result, item)
		}
	}
	return result
}

func excludeSelectionResources[T any](items []T, excluded []string, identity func(T) string) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		blocked := false
		for _, id := range excluded {
			if id == identity(item) {
				blocked = true
				break
			}
		}
		if !blocked {
			result = append(result, item)
		}
	}
	return result
}
