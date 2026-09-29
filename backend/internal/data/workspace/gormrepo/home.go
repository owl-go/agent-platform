package gormrepo

import (
	"context"
	"fmt"
	"sort"

	"agent-platform/backend/internal/biz/workspace/domain"
)

func (repository *Repository) GetHomeOverview(ctx context.Context, ownerID string) (domain.HomeOverview, error) {
	overview := domain.HomeOverview{RecentTasks: []domain.HomeTask{}, CommonWorkflows: []domain.HomeWorkflow{}, ActionItems: []domain.HomeAction{}}
	if err := repository.db.WithContext(ctx).Raw(`
		SELECT 'session' AS kind, session.id::text, ''::text AS parent_id, session.title, COALESCE(message.state, 'idle') AS state, session.updated_at
		FROM sessions session
		LEFT JOIN LATERAL (SELECT state FROM session_messages WHERE session_id = session.id ORDER BY id DESC LIMIT 1) message ON true
		WHERE session.owner_user_id = ? AND session.archived_at IS NULL AND NOT session.external
		UNION ALL
		SELECT 'run', run.id::text, workflow.id::text, workflow.name, run.state, COALESCE(run.ended_at, run.started_at, run.queued_at)
		FROM runs run JOIN workflows workflow ON workflow.id = run.workflow_id
		WHERE run.owner_user_id = ? AND workflow.deleted_at IS NULL
		ORDER BY updated_at DESC LIMIT 8`, ownerID, ownerID).Scan(&overview.RecentTasks).Error; err != nil {
		return domain.HomeOverview{}, fmt.Errorf("list Home recent tasks: %w", err)
	}
	if err := repository.db.WithContext(ctx).Raw(`
		SELECT workflow.id, workflow.name, count(run.id) AS run_count, GREATEST(workflow.updated_at, COALESCE(max(run.queued_at), workflow.updated_at)) AS updated_at
		FROM workflows workflow LEFT JOIN runs run ON run.workflow_id = workflow.id AND run.owner_user_id = workflow.owner_user_id
		WHERE workflow.owner_user_id = ? AND workflow.deleted_at IS NULL
		GROUP BY workflow.id, workflow.name, workflow.updated_at
		ORDER BY run_count DESC, updated_at DESC LIMIT 4`, ownerID).Scan(&overview.CommonWorkflows).Error; err != nil {
		return domain.HomeOverview{}, fmt.Errorf("list Home common Workflows: %w", err)
	}
	var actions []domain.HomeAction
	if err := repository.db.WithContext(ctx).Raw(`
		SELECT 'approval' AS kind, approval.id::text, approval.execution_kind, approval.execution_id,
		       CASE WHEN approval.execution_kind = 'session' THEN session.id::text ELSE workflow.id::text END AS parent_id,
		       approval.connector_name AS title, approval.state, approval.created_at
		FROM cli_command_approvals approval
		LEFT JOIN session_messages message ON approval.execution_kind = 'session' AND message.id::text = approval.execution_id
		LEFT JOIN sessions session ON session.id = message.session_id AND session.owner_user_id = approval.owner_user_id AND session.archived_at IS NULL AND NOT session.external
		LEFT JOIN runs run ON approval.execution_kind = 'run' AND run.id::text = approval.execution_id AND run.owner_user_id = approval.owner_user_id
		LEFT JOIN workflows workflow ON workflow.id = run.workflow_id AND workflow.deleted_at IS NULL
		WHERE approval.owner_user_id = ? AND approval.state = 'pending' AND approval.expires_at > now()
		UNION ALL
		SELECT 'plan', message.id::text, 'session', message.id::text, session.id::text, session.title, message.state, message.created_at
		FROM session_messages message JOIN sessions session ON session.id = message.session_id
		WHERE session.owner_user_id = ? AND session.archived_at IS NULL AND NOT session.external AND message.role = 'assistant' AND message.state = 'waiting_for_user' AND message.execution_plan IS NOT NULL
		UNION ALL
		SELECT 'plan', run.id::text, 'run', run.id::text, workflow.id::text, workflow.name, run.state, run.queued_at
		FROM runs run JOIN workflows workflow ON workflow.id = run.workflow_id
		WHERE run.owner_user_id = ? AND workflow.deleted_at IS NULL AND run.state = 'waiting_for_user' AND run.execution_plan IS NOT NULL
		UNION ALL
		SELECT 'failed', message.id::text, 'session', message.id::text, session.id::text, session.title, message.state, message.created_at
		FROM session_messages message JOIN sessions session ON session.id = message.session_id
		WHERE session.owner_user_id = ? AND session.archived_at IS NULL AND NOT session.external AND message.role = 'assistant' AND message.state = 'failed'
		UNION ALL
		SELECT 'failed', run.id::text, 'run', run.id::text, workflow.id::text, workflow.name, run.state, run.queued_at
		FROM runs run JOIN workflows workflow ON workflow.id = run.workflow_id
		WHERE run.owner_user_id = ? AND workflow.deleted_at IS NULL AND run.state = 'failed'
		ORDER BY created_at DESC LIMIT 24`, ownerID, ownerID, ownerID, ownerID, ownerID).Scan(&actions).Error; err != nil {
		return domain.HomeOverview{}, fmt.Errorf("list Home action items: %w", err)
	}
	byExecution := map[string]domain.HomeAction{}
	for _, action := range actions {
		key := action.ExecutionKind + ":" + action.ExecutionID
		if action.ParentID == "" {
			continue
		}
		current, exists := byExecution[key]
		if !exists || homeActionPriority(action.Kind) > homeActionPriority(current.Kind) {
			byExecution[key] = action
		}
	}
	for _, action := range byExecution {
		overview.ActionItems = append(overview.ActionItems, action)
	}
	sort.SliceStable(overview.ActionItems, func(left, right int) bool {
		leftPriority := homeActionPriority(overview.ActionItems[left].Kind)
		rightPriority := homeActionPriority(overview.ActionItems[right].Kind)
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		return overview.ActionItems[left].CreatedAt.After(overview.ActionItems[right].CreatedAt)
	})
	if len(overview.ActionItems) > 8 {
		overview.ActionItems = overview.ActionItems[:8]
	}
	return overview, nil
}

func homeActionPriority(kind string) int {
	switch kind {
	case "approval":
		return 3
	case "plan":
		return 2
	default:
		return 1
	}
}
