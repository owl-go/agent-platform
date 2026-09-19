package resourceaction

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	BeginMarker = "<platform-action>"
	EndMarker   = "</platform-action>"
	SkillKind   = "skill"
	ExpertKind  = "expert"
)

// Proposal is the only model output that the platform will treat as a write
// request. The marker keeps ordinary assistant prose inert.
type Proposal struct {
	Kind        string          `json:"kind"`
	UserMessage string          `json:"user_message"`
	Skill       *SkillProposal  `json:"skill,omitempty"`
	Expert      *ExpertProposal `json:"expert,omitempty"`
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
	Name                      string   `json:"name"`
	Introduction              string   `json:"introduction"`
	CoreCapability            string   `json:"core_capability"`
	OperatingProcedure        string   `json:"operating_procedure"`
	OutputStandard            string   `json:"output_standard"`
	Cautions                  string   `json:"cautions,omitempty"`
	Icon                      string   `json:"icon,omitempty"`
	IconBackground            string   `json:"icon_background,omitempty"`
	SkillIDs                  []string `json:"skill_ids,omitempty"`
	MCPServerIDs              []string `json:"mcp_server_ids,omitempty"`
	CLIConnectorDefinitionIDs []string `json:"cli_connector_definition_ids,omitempty"`
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
	if err := json.Unmarshal([]byte(strings.TrimSpace(content[start+len(BeginMarker):end])), &proposal); err != nil {
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
	if proposal.Kind != SkillKind && proposal.Kind != ExpertKind {
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
		if proposal.Expert == nil || strings.TrimSpace(proposal.Expert.Name) == "" {
			return fmt.Errorf("Expert proposal requires a name")
		}
		for field, value := range map[string]string{"introduction": proposal.Expert.Introduction, "core_capability": proposal.Expert.CoreCapability, "operating_procedure": proposal.Expert.OperatingProcedure, "output_standard": proposal.Expert.OutputStandard} {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("Expert proposal requires %s", field)
			}
		}
	}
	return nil
}

func (proposal Proposal) JSON() ([]byte, error) { return json.Marshal(proposal) }

func (proposal Proposal) NameAndDescription() (string, string) {
	if proposal.Kind == SkillKind && proposal.Skill != nil {
		return strings.TrimSpace(proposal.Skill.Name), strings.TrimSpace(proposal.Skill.Description)
	}
	if proposal.Expert != nil {
		return strings.TrimSpace(proposal.Expert.Name), strings.TrimSpace(proposal.Expert.Introduction)
	}
	return "", ""
}
