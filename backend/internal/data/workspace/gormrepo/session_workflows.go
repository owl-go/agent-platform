package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ application.SessionWorkflowRepository = (*Repository)(nil)

func (repository *Repository) CreateWorkflowFromSession(ctx context.Context, ownerID, workflowID, sessionID string, messageID int64, name, goal string) (domain.SessionWorkflowCreation, error) {
	input := domain.WorkflowInput{Name: name, Goal: goal}
	var result domain.SessionWorkflowCreation
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ? AND archived_at IS NULL", ownerID, sessionID).Take(&session).Error; err != nil {
			return mapNotFound(err)
		}
		if existing, err := sessionWorkflowCreationOnTx(tx, ownerID, sessionID, messageID); err == nil {
			result, result.Replayed = existing, true
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := input.Validate(); err != nil {
			return err
		}

		var message messageRecord
		if err := tx.Where("session_id = ? AND id = ? AND role = 'assistant' AND state = 'completed'", sessionID, messageID).Take(&message).Error; err != nil {
			return fmt.Errorf("%w: select a successful assistant response", domain.ErrInvalid)
		}
		var response domain.ResponseSnapshot
		if len(message.ResponseSnapshot) == 0 || string(message.ResponseSnapshot) == "null" || json.Unmarshal(message.ResponseSnapshot, &response) != nil || len(response.Stages) == 0 {
			return fmt.Errorf("%w: successful response has no reusable execution snapshot", domain.ErrInvalid)
		}
		schemaVersion := response.SchemaVersion
		if schemaVersion == 0 {
			schemaVersion = 2
		}
		plan := domain.ExecutionSnapshot{SchemaVersion: schemaVersion, Stages: append([]domain.ExecutionStageSnapshot(nil), response.Stages...), Coordination: response.Coordination, TeamProfile: response.TeamProfile}
		if _, err := plan.OrderedStages(); err != nil {
			return fmt.Errorf("%w: response execution snapshot cannot be reused", domain.ErrInvalid)
		}
		input.ExpertID, input.ExpertTeamID = reusableSpecialist(tx, ownerID, session, response.Stages)
		row, err := workflowRecordForInput(workflowID, ownerID, "workflows/"+ownerID+"/"+workflowID, input, nil)
		if err != nil {
			return err
		}
		row.ExecutionTemplate, err = marshal(response.Stages)
		if err != nil {
			return err
		}
		row.ExecutionCoordination, err = marshal(response.Coordination)
		if err != nil {
			return err
		}
		row.ExecutionTeamProfile, err = marshal(response.TeamProfile)
		if err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		run, err := createRunOnTx(tx, ownerID, workflowID, "session_conversion", nil, nil, domain.PlanPreferenceAuto, true)
		if err != nil {
			return err
		}
		origin := workflowSessionOriginRecord{WorkflowID: workflowID, OwnerID: ownerID, SessionID: sessionID, MessageID: messageID, ValidationRunID: run.ID, CreatedAt: run.QueuedAt}
		if err := tx.Create(&origin).Error; err != nil {
			return err
		}
		workflow, err := workflowDomain(row)
		if err != nil {
			return err
		}
		link := originDomain(origin, row.Name)
		workflow.Origin = &link
		result = domain.SessionWorkflowCreation{Workflow: workflow, Run: runDomain(run), Link: link}
		return nil
	})
	if err != nil {
		return domain.SessionWorkflowCreation{}, fmt.Errorf("create Workflow from Session: %w", err)
	}
	return result, nil
}

func (repository *Repository) ListSessionWorkflowLinks(ctx context.Context, ownerID, sessionID string) ([]domain.SessionWorkflowLink, error) {
	var exists int64
	if err := repository.db.WithContext(ctx).Model(&sessionRecord{}).Where("owner_user_id = ? AND id = ?", ownerID, sessionID).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists != 1 {
		return nil, domain.ErrNotFound
	}
	var rows []struct {
		workflowSessionOriginRecord
		WorkflowName string `gorm:"column:workflow_name"`
	}
	if err := repository.db.WithContext(ctx).Table("workflow_session_origins origin").
		Select("origin.*, workflow.name AS workflow_name").
		Joins("JOIN workflows workflow ON workflow.id = origin.workflow_id AND workflow.owner_user_id = origin.owner_user_id").
		Where("origin.owner_user_id = ? AND origin.session_id = ?", ownerID, sessionID).
		Order("origin.created_at DESC, origin.message_id DESC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Session Workflow links: %w", err)
	}
	result := make([]domain.SessionWorkflowLink, 0, len(rows))
	for _, row := range rows {
		result = append(result, originDomain(row.workflowSessionOriginRecord, row.WorkflowName))
	}
	return result, nil
}

func sessionWorkflowCreationOnTx(tx *gorm.DB, ownerID, sessionID string, messageID int64) (domain.SessionWorkflowCreation, error) {
	var origin workflowSessionOriginRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND session_id = ? AND message_id = ?", ownerID, sessionID, messageID).Take(&origin).Error; err != nil {
		return domain.SessionWorkflowCreation{}, err
	}
	var workflow workflowRecord
	if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, origin.WorkflowID).Take(&workflow).Error; err != nil {
		return domain.SessionWorkflowCreation{}, err
	}
	var run runRecord
	if err := tx.Where("owner_user_id = ? AND id = ? AND workflow_id = ?", ownerID, origin.ValidationRunID, origin.WorkflowID).Take(&run).Error; err != nil {
		return domain.SessionWorkflowCreation{}, err
	}
	item, err := workflowDomain(workflow)
	if err != nil {
		return domain.SessionWorkflowCreation{}, err
	}
	link := originDomain(origin, workflow.Name)
	item.Origin = &link
	return domain.SessionWorkflowCreation{Workflow: item, Run: runDomain(run), Link: link}, nil
}

func originDomain(row workflowSessionOriginRecord, workflowName string) domain.SessionWorkflowLink {
	return domain.SessionWorkflowLink{SessionID: row.SessionID, MessageID: row.MessageID, WorkflowID: row.WorkflowID, WorkflowName: workflowName, ValidationRunID: row.ValidationRunID, CreatedAt: row.CreatedAt}
}

func reusableSpecialist(tx *gorm.DB, ownerID string, session sessionRecord, stages []domain.ExecutionStageSnapshot) (*string, *string) {
	if len(stages) == 1 && stages[0].Expert != nil && strings.TrimSpace(stages[0].Expert.ID) != "" {
		id := stages[0].Expert.ID
		var count int64
		if tx.Model(&expertRecord{}).Where("owner_user_id IN (?) AND id = ?", accessibleResourceOwnerIDs(tx, ownerID), id).Count(&count).Error == nil && count == 1 {
			return &id, nil
		}
		return nil, nil
	}
	if len(stages) < 2 || session.ExpertTeamID == nil {
		return nil, nil
	}
	var team expertTeamRecord
	if tx.Where("owner_user_id = ? AND id = ?", ownerID, *session.ExpertTeamID).Take(&team).Error != nil {
		return nil, nil
	}
	var members []domain.ExpertTeamMemberInput
	if json.Unmarshal(team.Members, &members) != nil || len(members) != len(stages) {
		return nil, nil
	}
	for index := range members {
		if stages[index].Expert == nil || members[index].ExpertID != stages[index].Expert.ID {
			return nil, nil
		}
	}
	id := team.ID
	return nil, &id
}
