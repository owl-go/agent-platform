package application

import (
	"fmt"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// ExecutionSelection contains the already-authorized resources needed to plan
// exactly one anonymous, Expert, or Expert Team execution.
type ExecutionSelection struct {
	Anonymous *domain.ExecutionStageSnapshot
	Expert    *domain.ExecutionStageSnapshot
	Team      []domain.ExecutionStageSnapshot
}

// PlanExecution is the shared Session and Workflow boundary for producing the
// immutable ordered Stage representation.
func PlanExecution(common domain.ExecutionSnapshot, selection ExecutionSelection) (domain.ExecutionSnapshot, error) {
	selected := 0
	if selection.Anonymous != nil {
		selected++
	}
	if selection.Expert != nil {
		selected++
	}
	if selection.Team != nil {
		selected++
	}
	if selected != 1 || len(common.Stages) != 0 || common.RuntimeEngine != "" || common.ProviderModel.ID != "" || common.Expert != nil || common.ExpertTeam != nil {
		return domain.ExecutionSnapshot{}, fmt.Errorf("%w: execution planning requires exactly one selection and stage-free common data", domain.ErrInvalid)
	}

	var stages []domain.ExecutionStageSnapshot
	switch {
	case selection.Anonymous != nil:
		if selection.Anonymous.Expert != nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("%w: anonymous execution cannot contain an Expert", domain.ErrInvalid)
		}
		stages = []domain.ExecutionStageSnapshot{*selection.Anonymous}
	case selection.Expert != nil:
		if selection.Expert.Expert == nil {
			return domain.ExecutionSnapshot{}, fmt.Errorf("%w: Expert execution requires an Expert snapshot", domain.ErrInvalid)
		}
		stages = []domain.ExecutionStageSnapshot{*selection.Expert}
	default:
		if len(selection.Team) < 2 || len(selection.Team) > 10 {
			return domain.ExecutionSnapshot{}, fmt.Errorf("%w: Expert Team execution requires two to ten Stages", domain.ErrInvalid)
		}
		seen := make(map[string]struct{}, len(selection.Team))
		sharedRuntime := selection.Team[0].RuntimeEngine
		sharedModel := selection.Team[0].ProviderModel.ID
		for _, stage := range selection.Team {
			if stage.Expert == nil {
				return domain.ExecutionSnapshot{}, fmt.Errorf("%w: every Expert Team Stage requires an Expert snapshot", domain.ErrInvalid)
			}
			identity := stage.TeamMemberID
			if identity == "" {
				identity = stage.Expert.ID
			}
			if _, duplicate := seen[identity]; duplicate {
				return domain.ExecutionSnapshot{}, fmt.Errorf("%w: Expert Team Stages must contain distinct Member identities", domain.ErrInvalid)
			}
			seen[identity] = struct{}{}
			if stage.RuntimeEngine != sharedRuntime || stage.ProviderModel.ID != sharedModel {
				return domain.ExecutionSnapshot{}, fmt.Errorf("%w: every Expert Team Stage must share one execution configuration", domain.ErrInvalid)
			}
		}
		stages = append([]domain.ExecutionStageSnapshot(nil), selection.Team...)
	}

	common.SchemaVersion = 2
	common.Stages = stages
	if _, err := common.OrderedStages(); err != nil {
		return domain.ExecutionSnapshot{}, err
	}
	return common, nil
}
