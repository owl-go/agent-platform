package domain

import (
	"agent-platform/backend/internal/strictjson"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

type TeamCoordinationSnapshot struct {
	LeadMemberID           string `json:"lead_member_id"`
	MaxInvocations         int    `json:"max_invocations,omitempty"`
	MaxParallel            int    `json:"max_parallel,omitempty"`
	ActiveTimeoutSeconds   int    `json:"active_timeout_seconds,omitempty"`
	CreditBudgetHundredths int64  `json:"credit_budget_hundredths,omitempty"`
}

func (plan TeamCoordinationSnapshot) Limits() TeamCoordinationSnapshot {
	if plan.MaxInvocations == 0 {
		plan.MaxInvocations = 20
	}
	if plan.MaxParallel == 0 {
		plan.MaxParallel = 3
	}
	if plan.ActiveTimeoutSeconds == 0 {
		plan.ActiveTimeoutSeconds = 7200
	}
	return plan
}

func (plan TeamCoordinationSnapshot) Validate(stages []ExecutionStageSnapshot) error {
	limits := plan.Limits()
	if limits.MaxInvocations < 1 || limits.MaxInvocations > 20 || limits.MaxParallel < 1 || limits.MaxParallel > 3 || limits.ActiveTimeoutSeconds < 1 || limits.ActiveTimeoutSeconds > 7200 || limits.CreditBudgetHundredths < 0 || len(stages) < 2 || len(stages) > 10 {
		return fmt.Errorf("%w: invalid coordination limits or roster", ErrInvalid)
	}
	seen := map[string]bool{}
	lead := false
	for _, stage := range stages {
		if !teamMemberID.MatchString(stage.TeamMemberID) || seen[stage.TeamMemberID] || stage.Expert == nil {
			return fmt.Errorf("%w: invalid frozen Team member", ErrInvalid)
		}
		seen[stage.TeamMemberID] = true
		lead = lead || stage.TeamMemberID == plan.LeadMemberID
	}
	if !lead {
		return fmt.Errorf("%w: frozen Team Lead is absent", ErrInvalid)
	}
	return nil
}

type TeamDelegation struct {
	ID          string `json:"id"`
	MemberID    string `json:"member_id"`
	Instruction string `json:"instruction"`
	Required    bool   `json:"required"`
	RepairOf    string `json:"repair_of,omitempty"`
}

type TeamAction struct {
	Action      string               `json:"action"`
	Tasks       []TeamDelegation     `json:"tasks,omitempty"`
	Response    string               `json:"response,omitempty"`
	Resolutions []TeamFileResolution `json:"resolutions,omitempty"`
}

type TeamFileResolution struct {
	Path         string `json:"path"`
	SourceTaskID string `json:"source_task_id"`
}

type TeamWorkspaceConflict struct {
	Path                   string   `json:"path"`
	TaskIDs                []string `json:"task_ids"`
	State                  string   `json:"state"`
	ResolutionSourceTaskID string   `json:"resolution_source_task_id,omitempty"`
}

// ParseTeamAction accepts an entire completed invocation, never infers actions
// from prose, and does not modify scheduling or persistent state.
func ParseTeamAction(content string, roster []ExecutionStageSnapshot) (TeamAction, error) {
	var result TeamAction
	if len(content) > 100_000 {
		return result, fmt.Errorf("%w: coordination action exceeds limits", ErrInvalid)
	}
	if err := strictjson.Decode([]byte(content), &result); err != nil {
		return result, fmt.Errorf("%w: invalid Team Lead action: %v", ErrInvalid, err)
	}

	if result.Action == "delegate" {
		var envelope struct {
			Tasks []map[string]json.RawMessage `json:"tasks"`
		}
		_ = json.Unmarshal([]byte(content), &envelope)
		for _, task := range envelope.Tasks {
			if value, ok := task["required"]; !ok || string(value) == "null" {
				return result, fmt.Errorf("%w: each delegation requires an explicit required flag", ErrInvalid)
			}
		}
	}
	if len(result.Resolutions) > 200 {
		return result, fmt.Errorf("%w: too many file resolutions", ErrInvalid)
	}
	paths := map[string]bool{}
	for _, resolution := range result.Resolutions {
		if !fs.ValidPath(resolution.Path) || strings.ContainsAny(resolution.Path, "\\:\x00") || resolution.Path == ".git" || strings.HasPrefix(resolution.Path, ".git/") || paths[resolution.Path] || (resolution.SourceTaskID != "$lead" && !teamMemberID.MatchString(resolution.SourceTaskID)) {
			return result, fmt.Errorf("%w: invalid or duplicate file resolution", ErrInvalid)
		}
		paths[resolution.Path] = true
	}
	switch result.Action {

	case "complete":
		if strings.TrimSpace(result.Response) == "" || len(result.Tasks) != 0 {
			return result, fmt.Errorf("%w: completion needs only an official response", ErrInvalid)
		}
	case "delegate":
		if result.Response != "" || len(result.Tasks) < 1 || len(result.Tasks) > 20 {
			return result, fmt.Errorf("%w: delegation needs one to twenty tasks", ErrInvalid)
		}
		members := map[string]bool{}
		for _, stage := range roster {
			members[stage.TeamMemberID] = true
		}
		ids := map[string]bool{}
		for _, task := range result.Tasks {
			if !teamMemberID.MatchString(task.ID) || ids[task.ID] || !members[task.MemberID] || strings.TrimSpace(task.Instruction) == "" || len(task.Instruction) > 20_000 {
				return result, fmt.Errorf("%w: invalid delegation or unknown frozen member", ErrInvalid)
			}
			ids[task.ID] = true
		}
	default:
		return result, fmt.Errorf("%w: unknown coordination action", ErrInvalid)
	}
	return result, nil
}
