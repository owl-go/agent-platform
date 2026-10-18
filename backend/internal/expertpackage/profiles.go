package expertpackage

import (
	"context"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type TeamProfile struct {
	Translations   *DisplayTranslations `json:"translations,omitempty"`
	StarterPrompts []string             `json:"starter_prompts,omitempty"`
	AvatarFile     string               `json:"avatar_file,omitempty"`
	Name           string               `json:"name"`
	Introduction   string               `json:"introduction"`
	Icon           string               `json:"icon,omitempty"`
	IconBackground string               `json:"icon_background,omitempty"`
	CoreCapability string               `json:"core_capability"`
	LeadMemberID   string               `json:"lead_member_id"`
	Members        []Member             `json:"members"`
}

type Member struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Labels []string `json:"labels,omitempty"`
	Expert Profile  `json:"expert"`
}

func readProfile(profile *Profile, files map[string][]byte, used map[string]bool) (domain.ExpertInput, error) {
	var result domain.ExpertInput
	if err := profile.Translations.validate(profile.Name, profile.Introduction, profile.StarterPrompts); err != nil {
		return result, err
	}
	guidance, ok := files[profile.GuidanceFile]
	if !ok || used[profile.GuidanceFile] || !strings.HasPrefix(profile.GuidanceFile, "agents/") || !strings.HasSuffix(profile.GuidanceFile, ".md") {
		return result, fmt.Errorf("%w: Expert needs its own referenced Markdown document", domain.ErrInvalid)
	}
	used[profile.GuidanceFile] = true
	if len(profile.SkillKeys) > 50 {
		return result, fmt.Errorf("%w: too many Skill dependencies", domain.ErrInvalid)
	}
	seen := map[string]bool{}
	for _, key := range profile.SkillKeys {
		if !packageIdentity.MatchString(key) || seen[key] {
			return result, fmt.Errorf("%w: invalid Skill dependency", domain.ErrInvalid)
		}
		seen[key] = true
	}
	for _, dep := range profile.Connectors {
		if dep.Version != "" && !validVersion(dep.Version) {
			return result, fmt.Errorf("%w: invalid Connector version", domain.ErrInvalid)
		}
	}
	profileIcon, err := readAvatar(profile.Icon, profile.AvatarFile, files, used)
	if err != nil {
		return result, err
	}
	result = domain.ExpertInput{ConnectorDependencies: profile.Connectors, StarterPrompts: profile.StarterPrompts, Name: profile.Name, Introduction: profile.Introduction, Icon: profileIcon, IconBackground: profile.IconBackground, Guidance: string(guidance)}
	if strings.TrimSpace(result.Introduction) == "" || strings.TrimSpace(result.Guidance) == "" {
		return result, fmt.Errorf("%w: Introduction and Markdown guidance are required", domain.ErrInvalid)
	}
	return result, result.Validate()
}

func ExportTeam(ctx context.Context, item domain.ExpertTeam) ([]byte, error) {
	return ExportTeamContent(ctx, item, nil)
}
