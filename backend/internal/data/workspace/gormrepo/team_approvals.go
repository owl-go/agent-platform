package gormrepo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func teamExecutionState(stages []domain.ExpertStage, running string) string {
	active, waiting := 0, 0
	for _, stage := range stages {
		switch stage.State {
		case "running":
			active++
		case "waiting_for_user":
			active++
			waiting++
		}
	}
	if active > 0 && active == waiting {
		return "waiting_for_user"
	}
	return running
}

// The parent row lock serializes approval registration with invocation updates.
// Only actual executing invocations influence the aggregate wait state.
func transitionTeamApproval(tx *gorm.DB, request cliconnector.ApprovalRequest, resume bool, approvalID, eventType string) (bool, error) {
	var snapshot, encoded []byte
	var state string
	var messageID int64
	if request.ExecutionKind == "session" {
		var err error
		messageID, err = strconv.ParseInt(request.ExecutionID, 10, 64)
		if err != nil || messageID <= 0 {
			return true, domain.ErrInvalid
		}
		var row messageRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "state", "expert_stages", "response_snapshot").Where("id = ? AND session_id IN (SELECT id FROM sessions WHERE owner_user_id = ?)", messageID, request.OwnerID).Take(&row).Error; err != nil {
			return true, mapNotFound(err)
		}
		snapshot, encoded, state = row.ResponseSnapshot, row.ExpertStages, row.State
	} else {
		var row runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "state", "expert_stages", "workflow_snapshot").Where("id = ? AND owner_user_id = ?", request.ExecutionID, request.OwnerID).Take(&row).Error; err != nil {
			return true, mapNotFound(err)
		}
		snapshot, encoded, state = row.WorkflowSnapshot, row.ExpertStages, row.State
	}
	if !isCoordinatedSnapshot(snapshot) {
		return false, nil
	}
	running := "running"
	if request.ExecutionKind == "session" {
		running = "generating"
	}
	if state != running && state != "waiting_for_user" {
		if resume {
			return true, nil
		} // Closing an approval cannot revive terminal work.
		return true, domain.ErrConflict
	}
	var stages []domain.ExpertStage
	if err := json.Unmarshal(encoded, &stages); err != nil {
		return true, err
	}
	found := false
	for index := range stages {
		if stages[index].InvocationID != request.StageID {
			continue
		}
		found = true
		if resume {
			if stages[index].State == "waiting_for_user" {
				stages[index].State = "running"
			}
		} else {
			if stages[index].State != "running" {
				return true, domain.ErrConflict
			}
			stages[index].State = "waiting_for_user"
		}
		break
	}
	if !found {
		return true, fmt.Errorf("%w: approval does not match an actual Team invocation", domain.ErrConflict)
	}
	encoded, err := marshal(stages)
	if err != nil {
		return true, err
	}
	next := teamExecutionState(stages, running)
	if request.ExecutionKind == "session" {
		return true, tx.Model(&messageRecord{}).Where("id = ?", messageID).Updates(map[string]any{"state": next, "expert_stages": encoded, "progress_stage": "using_tool"}).Error
	}
	if err := tx.Model(&runRecord{}).Where("id = ?", request.ExecutionID).Updates(map[string]any{"state": next, "expert_stages": encoded, "version": gorm.Expr("version + 1")}).Error; err != nil {
		return true, err
	}
	payload, err := json.Marshal(map[string]string{"approval_id": approvalID})
	if err != nil {
		return true, err
	}
	var sequence int64
	if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", request.ExecutionID).Scan(&sequence).Error; err != nil {
		return true, err
	}
	if err := tx.Table("run_events").Create(map[string]any{"run_id": request.ExecutionID, "sequence": sequence + 1, "event_type": eventType, "payload": payload, "occurred_at": time.Now().UTC()}).Error; err != nil {
		return true, err
	}
	if !resume && next == "waiting_for_user" && state != next {
		return true, enqueueChannelWaiting(tx, request.ExecutionID)
	}
	return true, nil
}
