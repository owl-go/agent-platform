package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func decideExecutionPlan(plan *domain.ExecutionPlan, decision string, expectedVersion int64, now time.Time) error {
	if plan == nil || plan.State != "pending" || plan.Version != expectedVersion {
		return domain.ErrConflict
	}
	switch decision {
	case "start":
		plan.State = "approved"
	case "direct":
		if !plan.AllowsDirectAnswer() {
			return fmt.Errorf("%w: direct answer cannot bypass planned side effects", domain.ErrInvalid)
		}
		plan.State = "skipped"
	case "cancel":
		plan.State = "cancelled"
	default:
		return fmt.Errorf("%w: invalid Plan decision", domain.ErrInvalid)
	}
	plan.DecidedAt = &now
	plan.Version++
	return plan.Validate()
}

func (repository *Repository) DecideSessionExecutionPlan(ctx context.Context, ownerID, sessionID string, messageID int64, decision string, expectedVersion int64) (domain.Message, error) {
	var row messageRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT message.* FROM session_messages message
			JOIN sessions session ON session.id = message.session_id
			WHERE session.owner_user_id = ? AND message.session_id = ? AND message.id = ?
			  AND message.role = 'assistant' AND message.state = 'waiting_for_user'
			FOR UPDATE OF message`, ownerID, sessionID, messageID).Scan(&row).Error; err != nil {
			return err
		}
		if row.ID == 0 || len(row.ExecutionPlan) == 0 {
			return domain.ErrNotFound
		}
		var plan domain.ExecutionPlan
		if err := json.Unmarshal(row.ExecutionPlan, &plan); err != nil {
			return fmt.Errorf("decode Execution Plan: %w", err)
		}
		now := time.Now().UTC()
		if err := decideExecutionPlan(&plan, decision, expectedVersion, now); err != nil {
			return err
		}
		encoded, err := marshal(plan)
		if err != nil {
			return err
		}
		updates := map[string]any{"execution_plan": encoded}
		if decision == "cancel" {
			updates["state"], updates["progress_stage"], updates["cancel_requested_at"], updates["completed_at"] = "cancelled", "", now, now
			updates["elapsed_ms"] = gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now)
		} else {
			updates["state"], updates["progress_stage"] = "queued", "preparing"
		}
		if result := tx.Model(&messageRecord{}).Where("id = ? AND state = 'waiting_for_user'", row.ID).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return errorsOrConflict(result.Error)
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return domain.Message{}, fmt.Errorf("decide Session Execution Plan: %w", err)
	}
	return messageDomain(row), nil
}

func (repository *Repository) DecideRunExecutionPlan(ctx context.Context, ownerID, workflowID, runID, decision string, expectedVersion int64) (domain.Run, error) {
	var row runRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND workflow_id = ? AND id = ? AND state = 'waiting_for_user'", ownerID, workflowID, runID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if len(row.ExecutionPlan) == 0 {
			return domain.ErrNotFound
		}
		var plan domain.ExecutionPlan
		if err := json.Unmarshal(row.ExecutionPlan, &plan); err != nil {
			return fmt.Errorf("decode Execution Plan: %w", err)
		}
		now := time.Now().UTC()
		if err := decideExecutionPlan(&plan, decision, expectedVersion, now); err != nil {
			return err
		}
		encoded, err := marshal(plan)
		if err != nil {
			return err
		}
		updates := map[string]any{"execution_plan": encoded, "version": gorm.Expr("version + 1")}
		if decision == "cancel" {
			updates["state"], updates["cancel_requested_at"], updates["ended_at"] = "cancelled", now, now
		} else {
			updates["state"], updates["queued_at"] = "queued", now
		}
		if result := tx.Model(&runRecord{}).Where("id = ? AND state = 'waiting_for_user'", row.ID).Updates(updates); result.Error != nil || result.RowsAffected != 1 {
			return errorsOrConflict(result.Error)
		}
		eventType := "plan." + plan.State
		if err := appendRunEventPayload(tx, row.ID, eventType, map[string]any{"state": plan.State}, now); err != nil {
			return err
		}
		if decision == "cancel" {
			if err := appendRunEventPayload(tx, row.ID, "run.cancelled", map[string]any{"state": "cancelled"}, now); err != nil {
				return err
			}
		} else {
			var queued int64
			if err := tx.Model(&runRecord{}).Where("workflow_id = ? AND state = 'queued' AND (queued_at, id) <= (?, ?)", workflowID, now, row.ID).Count(&queued).Error; err != nil {
				return err
			}
			if err := appendRunEventPayload(tx, row.ID, "run.queued", map[string]any{"state": "queued", "queue_position": queued}, now); err != nil {
				return err
			}
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return domain.Run{}, fmt.Errorf("decide Run Execution Plan: %w", err)
	}
	item := runDomain(row)
	var positionErr error
	item.QueuePosition, positionErr = repository.runQueuePosition(ctx, item)
	return item, positionErr
}

func appendRunEventPayload(tx *gorm.DB, runID, eventType string, payload any, occurredAt time.Time) error {
	var sequence int64
	if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", runID).Scan(&sequence).Error; err != nil {
		return err
	}
	encoded, err := marshal(payload)
	if err != nil {
		return err
	}
	return tx.Table("run_events").Create(map[string]any{"run_id": runID, "sequence": sequence + 1, "event_type": eventType, "payload": encoded, "occurred_at": occurredAt}).Error
}

func errorsOrConflict(err error) error {
	if err != nil {
		return err
	}
	return domain.ErrConflict
}

func executionPlanStarted(encoded []byte) ([]byte, error) {
	if len(encoded) == 0 || string(encoded) == "null" {
		return nil, nil
	}
	var plan domain.ExecutionPlan
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return nil, fmt.Errorf("decode Execution Plan: %w", err)
	}
	if plan.State != "approved" && plan.State != "executing" {
		return nil, nil
	}
	plan.State = "executing"
	for index := range plan.Steps {
		plan.Steps[index].State = "pending"
		if plan.Steps[index].Kind == "review_input" {
			plan.Steps[index].State = "completed"
			continue
		}
		plan.Steps[index].State = "running"
		break
	}
	plan.Version++
	return marshal(plan)
}

func executionPlanStageUpdated(encoded []byte, position int, state string) ([]byte, error) {
	if len(encoded) == 0 || string(encoded) == "null" {
		return nil, nil
	}
	var plan domain.ExecutionPlan
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return nil, fmt.Errorf("decode Execution Plan: %w", err)
	}
	if plan.State != "executing" {
		return nil, nil
	}
	wanted, targetIndex := 0, -1
	for index := range plan.Steps {
		if plan.Steps[index].Kind == "execute_stage" {
			wanted++
			if wanted == position {
				targetIndex = index
				break
			}
		}
	}
	if targetIndex < 0 {
		return nil, nil
	}
	for index := 0; index < targetIndex; index++ {
		if plan.Steps[index].State == "pending" || plan.Steps[index].State == "running" {
			plan.Steps[index].State = "completed"
		}
	}
	step := &plan.Steps[targetIndex]
	switch state {
	case "running":
		step.State = "running"
	case "succeeded":
		step.State = "completed"
	case "failed", "cancelled":
		step.State = "failed"
	default:
		return nil, nil
	}
	if state == "succeeded" {
		for next := targetIndex + 1; next < len(plan.Steps); next++ {
			if plan.Steps[next].State == "pending" {
				plan.Steps[next].State = "running"
				break
			}
		}
	}
	plan.Version++
	return marshal(plan)
}

func executionPlanFinished(encoded []byte, outcome string) ([]byte, error) {
	if len(encoded) == 0 || string(encoded) == "null" {
		return nil, nil
	}
	var plan domain.ExecutionPlan
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return nil, fmt.Errorf("decode Execution Plan: %w", err)
	}
	if plan.State == "skipped" || plan.State == "cancelled" {
		return nil, nil
	}
	switch outcome {
	case "completed":
		plan.State = "completed"
		for index := range plan.Steps {
			plan.Steps[index].State = "completed"
		}
	case "failed":
		plan.State = "failed"
		marked := false
		for index := range plan.Steps {
			if !marked && (plan.Steps[index].State == "running" || plan.Steps[index].State == "pending") {
				plan.Steps[index].State, marked = "failed", true
			} else if plan.Steps[index].State == "pending" || plan.Steps[index].State == "running" {
				plan.Steps[index].State = "skipped"
			}
		}
	case "cancelled":
		plan.State = "cancelled"
		for index := range plan.Steps {
			if plan.Steps[index].State == "pending" || plan.Steps[index].State == "running" {
				plan.Steps[index].State = "skipped"
			}
		}
	default:
		return nil, fmt.Errorf("invalid Execution Plan outcome %q", outcome)
	}
	plan.Version++
	return marshal(plan)
}

func executionPlanDirectAnswer(encoded []byte) bool {
	var plan domain.ExecutionPlan
	return len(encoded) > 0 && json.Unmarshal(encoded, &plan) == nil && plan.State == "skipped"
}

func withoutExternalOperations(snapshot domain.ExecutionSnapshot) domain.ExecutionSnapshot {
	stages, err := snapshot.OrderedStages()
	if err != nil {
		return snapshot
	}
	for index := range stages {
		stages[index].MCPServers = nil
		stages[index].CLIConnectors = nil
	}
	snapshot.MCPServers = nil
	snapshot.CLIConnectors = nil
	if snapshot.SchemaVersion == 2 {
		snapshot.Stages = stages
	}
	return snapshot
}
