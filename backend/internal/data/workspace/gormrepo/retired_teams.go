package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

func retiredPrivateTeam(tx *gorm.DB, owner, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	var count int64
	err := tx.Table("retired_private_expert_teams").Where("owner_user_id=? AND team_id=?", owner, id).Count(&count).Error
	return count > 0, err
}

func retiredSnapshotTeam(tx *gorm.DB, owner string, plan domain.ExecutionSnapshot) (bool, error) {
	if plan.TeamProfile != nil {
		return retiredPrivateTeam(tx, owner, plan.TeamProfile.ID)
	}
	if plan.ExpertTeam != nil {
		return retiredPrivateTeam(tx, owner, plan.ExpertTeam.ID)
	}
	return false, nil
}
