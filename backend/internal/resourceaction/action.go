package resourceaction

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/strictjson"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	BeginMarker   = "<platform-action>"
	EndMarker     = "</platform-action>"
	SkillKind     = "skill"
	ExpertKind    = "expert"
	TeamKind      = "expert_team"
	ConnectorKind = "connector"
)

// Proposal is the only model output that the platform will treat as a write
// request. The marker keeps ordinary assistant prose inert.
type Proposal struct {
	Kind        string             `json:"kind"`
	UserMessage string             `json:"user_message"`
	Skill       *SkillProposal     `json:"skill,omitempty"`
	Team        *TeamProposal      `json:"expert_team,omitempty"`
	Expert      *ExpertProposal    `json:"expert,omitempty"`
	Connector   *ConnectorProposal `json:"connector,omitempty"`
}

// ConnectorProposal contains the fields accepted by the guided private MCP
// Connector Package creation path. CLI packages require a reviewed bundle.
type ConnectorProposal struct {
	Source        string `json:"source"`
	Version       string `json:"version"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	AuthMode      string `json:"auth_mode"`
	MCPJSON       string `json:"mcp_json"`
	SkillName     string `json:"skill_name"`
	SkillMarkdown string `json:"skill_markdown"`
}

type SkillProposal struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	GitURL      string `json:"git_url,omitempty"`
	GitRef      string `json:"git_ref,omitempty"`
	Document    string `json:"document,omitempty"`
}

type ExpertProposal struct {
	Name                      string                             `json:"name"`
	Introduction              string                             `json:"introduction"`
	Guidance                  string                             `json:"guidance"`
	Icon                      string                             `json:"icon,omitempty"`
	IconBackground            string                             `json:"icon_background,omitempty"`
	StarterPrompts            []string                           `json:"starter_prompts,omitempty"`
	SkillIDs                  []string                           `json:"skill_ids,omitempty"`
	MCPServerIDs              []string                           `json:"mcp_server_ids,omitempty"`
	CLIConnectorDefinitionIDs []string                           `json:"cli_connector_definition_ids,omitempty"`
	Connectors                []domain.ExpertConnectorDependency `json:"connector_dependencies,omitempty"`
}
type TeamProposal struct {
	Name           string               `json:"name"`
	Introduction   string               `json:"introduction"`
	CoreCapability string               `json:"core_capability"`
	Icon           string               `json:"icon,omitempty"`
	IconBackground string               `json:"icon_background,omitempty"`
	StarterPrompts []string             `json:"starter_prompts,omitempty"`
	LeadMemberID   string               `json:"lead_member_id"`
	Members        []TeamMemberProposal `json:"members"`
}
type TeamMemberProposal struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Labels []string       `json:"labels,omitempty"`
	Expert ExpertProposal `json:"expert"`
}

func (p ExpertProposal) Input() domain.ExpertInput {
	return domain.ExpertInput{Name: p.Name, Introduction: p.Introduction, Guidance: p.Guidance, Icon: p.Icon, IconBackground: p.IconBackground, StarterPrompts: p.StarterPrompts, SkillIDs: p.SkillIDs, MCPServerIDs: p.MCPServerIDs, CLIConnectorDefinitionIDs: p.CLIConnectorDefinitionIDs, ConnectorDependencies: p.Connectors}
}
func (p TeamProposal) Input() domain.ExpertTeamInput {
	input := domain.ExpertTeamInput{Name: p.Name, Introduction: p.Introduction, CoreCapability: p.CoreCapability, Icon: p.Icon, IconBackground: p.IconBackground, StarterPrompts: p.StarterPrompts, LeadMemberID: p.LeadMemberID}
	for _, member := range p.Members {
		definition := member.Expert.Input()
		input.Members = append(input.Members, domain.ExpertTeamMemberInput{ID: member.ID, Name: member.Name, Labels: member.Labels, Definition: &definition})
	}
	return input
}

// Parse extracts and validates a platform proposal from assistant output.
// It returns the user-facing prose with the machine marker removed.
func Parse(content string) (Proposal, string, bool, error) {
	start := strings.Index(content, BeginMarker)
	if start < 0 {
		return Proposal{}, content, false, nil
	}
	end := strings.Index(content[start+len(BeginMarker):], EndMarker)
	if end < 0 {
		return Proposal{}, content, true, fmt.Errorf("resource action marker is incomplete")
	}
	end += start + len(BeginMarker)
	var proposal Proposal
	if len(content) > 4*1024*1024 || strings.Count(content, BeginMarker) != 1 || strings.Count(content, EndMarker) != 1 {
		return Proposal{}, content, true, fmt.Errorf("invalid resource proposal envelope")
	}
	if err := strictjson.Decode([]byte(strings.TrimSpace(content[start+len(BeginMarker):end])), &proposal); err != nil {
		return Proposal{}, content, true, fmt.Errorf("decode resource action: %w", err)
	}
	if err := proposal.Validate(); err != nil {
		return Proposal{}, content, true, err
	}
	visible := strings.TrimSpace(content[:start] + content[end+len(EndMarker):])
	if visible == "" {
		visible = strings.TrimSpace(proposal.UserMessage)
	}
	return proposal, visible, true, nil
}

func (proposal Proposal) Validate() error {
	if proposal.Kind != SkillKind && proposal.Kind != ExpertKind && proposal.Kind != ConnectorKind && proposal.Kind != TeamKind {
		return fmt.Errorf("unsupported resource action kind %q", proposal.Kind)
	}
	switch proposal.Kind {
	case SkillKind:
		if proposal.Skill == nil || strings.TrimSpace(proposal.Skill.Name) == "" {
			return fmt.Errorf("Skill proposal requires a name")
		}
		if proposal.Skill.Source == "" {
			proposal.Skill.Source = "generated"
		}
		if proposal.Skill.Source != "generated" && proposal.Skill.Source != "git" {
			return fmt.Errorf("Skill proposal source must be generated or git")
		}
		if proposal.Skill.Source == "git" && strings.TrimSpace(proposal.Skill.GitURL) == "" {
			return fmt.Errorf("Git Skill proposal requires git_url")
		}
		if proposal.Skill.Source == "generated" && strings.TrimSpace(proposal.Skill.Document) == "" {
			return fmt.Errorf("generated Skill proposal requires document")
		}
	case ExpertKind:
		if proposal.Expert == nil || proposal.Team != nil || proposal.Skill != nil || proposal.Connector != nil {
			return fmt.Errorf("Expert proposal requires one profile")
		}
		input := proposal.Expert.Input()
		if strings.TrimSpace(input.Guidance) == "" || strings.TrimSpace(input.Introduction) == "" {
			return fmt.Errorf("Expert needs Introduction and Markdown guidance")
		}
		return input.Validate()
	case TeamKind:
		if proposal.Team == nil || proposal.Expert != nil || proposal.Skill != nil || proposal.Connector != nil {
			return fmt.Errorf("Team proposal requires one profile")
		}
		input := proposal.Team.Input()
		if err := input.Validate(); err != nil {
			return err
		}
		if err := input.ValidateLead(); err != nil {
			return err
		}
		for _, member := range input.Members {
			if strings.TrimSpace(member.Definition.Guidance) == "" {
				return fmt.Errorf("member needs Markdown guidance")
			}
			if err := member.Definition.Validate(); err != nil {
				return err
			}
		}
		return nil
	case ConnectorKind:
		if proposal.Connector == nil {
			return fmt.Errorf("Connector proposal is required")
		}
		input := proposal.Connector
		for field, value := range map[string]string{"source": input.Source, "version": input.Version, "name": input.Name, "description": input.Description, "mcp_json": input.MCPJSON, "skill_name": input.SkillName, "skill_markdown": input.SkillMarkdown} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("Connector proposal requires %s", field)
			}
		}
		if input.AuthMode != "none" && input.AuthMode != "oauth" {
			return fmt.Errorf("Connector proposal auth_mode must be none or oauth")
		}
	}
	return nil
}

func (proposal Proposal) JSON() ([]byte, error) { return json.Marshal(proposal) }

func (proposal Proposal) NameAndDescription() (string, string) {
	if proposal.Kind == SkillKind && proposal.Skill != nil {
		return strings.TrimSpace(proposal.Skill.Name), strings.TrimSpace(proposal.Skill.Description)
	}
	if proposal.Kind == ConnectorKind && proposal.Connector != nil {
		return strings.TrimSpace(proposal.Connector.Name), strings.TrimSpace(proposal.Connector.Description)
	}
	if proposal.Team != nil {
		return strings.TrimSpace(proposal.Team.Name), strings.TrimSpace(proposal.Team.Introduction)
	}
	if proposal.Expert != nil {
		return strings.TrimSpace(proposal.Expert.Name), strings.TrimSpace(proposal.Expert.Introduction)
	}
	return "", ""
}
