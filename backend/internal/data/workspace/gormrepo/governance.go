package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) TransferGroupResources(ctx context.Context, actorUserID, groupID, fromUserID, toUserID, reason string) (int64, error) {
	reason = strings.TrimSpace(reason)
	for name, value := range map[string]string{"actor": actorUserID, "Group": groupID, "source User": fromUserID, "target User": toUserID} {
		if _, err := uuid.Parse(value); err != nil {
			return 0, fmt.Errorf("%w: invalid %s identifier", domain.ErrInvalid, name)
		}
	}
	if fromUserID == toUserID || reason == "" || len(reason) > 500 {
		return 0, fmt.Errorf("%w: transfer requires distinct Users and a reason", domain.ErrInvalid)
	}
	var transferred int64
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actor struct {
			Administrator bool `gorm:"column:administrator"`
		}
		if err := tx.Table("users").Where("id = ? AND disabled_at IS NULL", actorUserID).Take(&actor).Error; err != nil || !actor.Administrator {
			return domain.ErrInvalid
		}
		var group struct {
			ID string `gorm:"column:id"`
		}
		if err := tx.Table("identity_groups").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND deleted_at IS NULL AND department = true", groupID).Take(&group).Error; err != nil {
			return mapNotFound(err)
		}
		var source struct {
			DisabledAt *time.Time `gorm:"column:disabled_at"`
		}
		if err := tx.Table("users").Where("id = ?", fromUserID).Take(&source).Error; err != nil {
			return mapNotFound(err)
		}
		if source.DisabledAt == nil {
			return fmt.Errorf("%w: source User must be disabled before transfer", domain.ErrInvalid)
		}
		var target struct {
			ResourcePublisher bool `gorm:"column:resource_publisher"`
		}
		if err := tx.Table("users").Where("id = ? AND disabled_at IS NULL", toUserID).Take(&target).Error; err != nil {
			return mapNotFound(err)
		}
		if !target.ResourcePublisher {
			return fmt.Errorf("%w: target User is not a Resource Publisher", domain.ErrInvalid)
		}
		var targetMembership int64
		if err := tx.Table("identity_group_memberships").Where("group_id = ? AND user_id = ?", groupID, toUserID).Count(&targetMembership).Error; err != nil {
			return err
		}
		if targetMembership != 1 {
			return fmt.Errorf("%w: target User must belong to the Department", domain.ErrInvalid)
		}
		var conflicts int64
		if err := tx.Raw(`SELECT COUNT(*) FROM knowledge_bases source
			JOIN knowledge_bases target ON target.owner_user_id = ? AND target.name = source.name AND target.deleted_at IS NULL
			WHERE source.owner_user_id = ? AND source.group_id = ? AND source.scope_type = 'group' AND source.deleted_at IS NULL`, toUserID, fromUserID, groupID).Scan(&conflicts).Error; err != nil {
			return err
		}
		if conflicts > 0 {
			return fmt.Errorf("%w: target User already owns a resource with the same name", domain.ErrConflict)
		}
		result := tx.Table("knowledge_bases").Where("owner_user_id = ? AND group_id = ? AND scope_type = 'group' AND deleted_at IS NULL", fromUserID, groupID).Updates(map[string]any{"owner_user_id": toUserID, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		transferred = result.RowsAffected
		detail, _ := json.Marshal(map[string]int64{"knowledge_bases": transferred})
		return tx.Table("governance_audit_events").Create(map[string]any{"actor_user_id": actorUserID, "action": "group.resources.transferred", "target_type": "identity_group", "target_id": groupID, "reason": reason, "detail": detail}).Error
	})
	if err != nil {
		return 0, fmt.Errorf("transfer Group resources: %w", err)
	}
	return transferred, nil
}
