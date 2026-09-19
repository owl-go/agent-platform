package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/resourceaction"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ResourceCreationActionRepository interface {
	CreateResourceCreationAction(context.Context, string, string, int64, resourceaction.Proposal) (workspacedomain.ResourceCreationAction, error)
	GetResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error)
	ClaimResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error)
	CancelResourceCreationAction(context.Context, string, string) (workspacedomain.ResourceCreationAction, error)
	CompleteResourceCreationAction(context.Context, string, string, string, string, error) (workspacedomain.ResourceCreationAction, error)
}

func (repository *Repository) CancelResourceCreationAction(ctx context.Context, ownerID, actionID string) (workspacedomain.ResourceCreationAction, error) {
	var row resourceCreationActionRecord
	changed := false
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, actionID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == "cancelled" || row.State == "expired" {
			return nil
		}
		if row.State != "pending" {
			return workspacedomain.ErrConflict
		}
		changed = true
		return tx.Model(&resourceCreationActionRecord{}).Where("id = ? AND state = 'pending' AND version = ?", row.ID, row.Version).Updates(map[string]any{"state": "cancelled", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error
	}); err != nil {
		return workspacedomain.ResourceCreationAction{}, err
	}
	if changed {
		row.State = "cancelled"
		row.Version++
	}
	return resourceCreationActionDomain(row), nil
}

func (repository *Repository) CreateResourceCreationAction(ctx context.Context, ownerID, sessionID string, messageID int64, proposal resourceaction.Proposal) (workspacedomain.ResourceCreationAction, error) {
	payload, err := proposal.JSON()
	if err != nil {
		return workspacedomain.ResourceCreationAction{}, err
	}
	name, description := proposal.NameAndDescription()
	now := time.Now().UTC()
	row := resourceCreationActionRecord{ID: uuid.NewString(), OwnerID: ownerID, SessionID: sessionID, MessageID: messageID, Kind: proposal.Kind, State: "pending", Name: name, Description: description, Payload: payload, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now, Version: 1}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&messageRecord{}).Where("id = ? AND session_id = ?", messageID, sessionID).Update("resource_creation_action_id", row.ID).Error
	}); err != nil {
		return workspacedomain.ResourceCreationAction{}, fmt.Errorf("persist resource creation action: %w", err)
	}
	return resourceCreationActionDomain(row), nil
}

func (repository *Repository) GetResourceCreationAction(ctx context.Context, ownerID, actionID string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error) {
	var row resourceCreationActionRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, actionID).Take(&row).Error; err != nil {
		return workspacedomain.ResourceCreationAction{}, resourceaction.Proposal{}, mapNotFound(err)
	}
	proposal, err := decodeActionProposal(row.Payload)
	if err != nil {
		return workspacedomain.ResourceCreationAction{}, resourceaction.Proposal{}, err
	}
	return resourceCreationActionDomain(row), proposal, nil
}

func (repository *Repository) ClaimResourceCreationAction(ctx context.Context, ownerID, actionID string) (workspacedomain.ResourceCreationAction, resourceaction.Proposal, error) {
	var row resourceCreationActionRecord
	expired := false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ?", ownerID, actionID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == "confirmed" {
			return nil
		}
		if row.State != "pending" {
			return workspacedomain.ErrConflict
		}
		if !row.ExpiresAt.After(time.Now().UTC()) {
			expired = true
			return tx.Model(&row).Updates(map[string]any{"state": "expired", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error
		}
		result := tx.Model(&resourceCreationActionRecord{}).Where("id = ? AND state = 'pending' AND version = ?", row.ID, row.Version).Updates(map[string]any{"state": "processing", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return workspacedomain.ErrConflict
		}
		row.State = "processing"
		row.Version++
		return nil
	})
	if err != nil {
		return workspacedomain.ResourceCreationAction{}, resourceaction.Proposal{}, err
	}
	if expired {
		return workspacedomain.ResourceCreationAction{}, resourceaction.Proposal{}, workspacedomain.ErrConflict
	}
	proposal, err := decodeActionProposal(row.Payload)
	if err != nil {
		return workspacedomain.ResourceCreationAction{}, resourceaction.Proposal{}, err
	}
	return resourceCreationActionDomain(row), proposal, nil
}

func (repository *Repository) CompleteResourceCreationAction(ctx context.Context, ownerID, actionID, state, resourceID string, actionErr error) (workspacedomain.ResourceCreationAction, error) {
	updates := map[string]any{"state": state, "resource_id": nil, "error": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	if resourceID != "" {
		updates["resource_id"] = resourceID
	}
	if actionErr != nil {
		message := actionErr.Error()
		updates["error"] = message
	}
	var row resourceCreationActionRecord
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_user_id = ? AND id = ?", ownerID, actionID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == "confirmed" || row.State == "failed" {
			return nil
		}
		result := tx.Model(&resourceCreationActionRecord{}).Where("id = ? AND state = 'processing' AND version = ?", row.ID, row.Version).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return workspacedomain.ErrConflict
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	}); err != nil {
		return workspacedomain.ResourceCreationAction{}, err
	}
	return resourceCreationActionDomain(row), nil
}

func decodeActionProposal(payload []byte) (resourceaction.Proposal, error) {
	var proposal resourceaction.Proposal
	if err := json.Unmarshal(payload, &proposal); err != nil {
		return resourceaction.Proposal{}, fmt.Errorf("decode resource creation action: %w", err)
	}
	return proposal, proposal.Validate()
}

func resourceCreationActionDomain(row resourceCreationActionRecord) workspacedomain.ResourceCreationAction {
	item := workspacedomain.ResourceCreationAction{ID: row.ID, Kind: row.Kind, State: row.State, Name: row.Name, Description: row.Description, ExpiresAt: row.ExpiresAt, Version: row.Version}
	if row.ResourceID != nil {
		item.ResourceID = *row.ResourceID
	}
	if row.Error != nil {
		item.Error = *row.Error
	}
	return item
}
