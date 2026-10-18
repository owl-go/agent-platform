package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/resourceaction"
	"context"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// Expert creation and its confirmation commit together, so a lost HTTP reply
// can be replayed without creating a second resource.
func (repository *Repository) ConfirmExpertCreationAction(ctx context.Context, owner, id string) (domain.ResourceCreationAction, error) {
	var row resourceCreationActionRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=?", id, owner).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Kind == resourceaction.TeamKind {
			if err := requireTeamAdministrator(tx, owner); err != nil {
				return err
			}
		}
		if row.Kind != resourceaction.ExpertKind && row.Kind != resourceaction.TeamKind {
			return domain.ErrInvalid
		}
		if row.State == "confirmed" {
			return nil
		}
		if row.State != "pending" || !row.ExpiresAt.After(time.Now().UTC()) {
			return domain.ErrConflict
		}
		proposal, err := decodeActionProposal(row.Payload)
		if err != nil {
			return fmt.Errorf("%w: invalid Expert preview", domain.ErrInvalid)
		}
		child := New(tx, repository.credits)
		var resourceID string
		if row.Kind == resourceaction.ExpertKind {
			item, err := child.CreateExpert(ctx, owner, proposal.Expert.Input())
			if err != nil {
				return err
			}
			resourceID = item.ID
		} else {
			item, err := child.CreateExpertTeam(ctx, owner, proposal.Team.Input())
			if err != nil {
				return err
			}
			resourceID = item.ID
		}
		result := tx.Model(&resourceCreationActionRecord{}).Where("id=? AND state='pending' AND version=?", id, row.Version).Updates(map[string]any{"state": "confirmed", "resource_id": resourceID, "error": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version+1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		row.State = "confirmed"
		row.ResourceID = &resourceID
		row.Version++
		return nil
	})
	return resourceCreationActionDomain(row), err
}
func (repository *Repository) ReviseExpertCreationAction(ctx context.Context, owner, id string, proposal resourceaction.Proposal, version int64) (domain.ResourceCreationAction, error) {
	if (proposal.Kind != resourceaction.ExpertKind && proposal.Kind != resourceaction.TeamKind) || proposal.Validate() != nil {
		return domain.ResourceCreationAction{}, domain.ErrInvalid
	}
	payload, err := proposal.JSON()
	if err != nil {
		return domain.ResourceCreationAction{}, err
	}
	var row resourceCreationActionRecord
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=?", id, owner).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Kind != proposal.Kind {
			return domain.ErrInvalid
		}
		if row.Kind == resourceaction.TeamKind {
			if err := requireTeamAdministrator(tx, owner); err != nil {
				return err
			}
		}
		if row.Version != version || (row.State != "pending" && row.State != "failed") {
			return domain.ErrConflict
		}
		name, description := proposal.NameAndDescription()
		result := tx.Model(&row).Updates(map[string]any{"payload": payload, "name": name, "description": description, "state": "pending", "error": nil, "expires_at": time.Now().UTC().Add(15 * time.Minute), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version+1")})
		if result.Error != nil {
			return result.Error
		}
		return tx.Where("id=?", id).Take(&row).Error
	})
	return resourceCreationActionDomain(row), err
}
