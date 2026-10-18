package gormrepo

import (
	"errors"

	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

func freezeTeamCreditBudget(tx *gorm.DB, ownerID string, coordination **domain.TeamCoordinationSnapshot) error {
	if *coordination == nil {
		return nil
	}
	var settings settingsRecord
	if err := tx.Select("team_credit_budget_hundredths").Where("user_id = ?", ownerID).Take(&settings).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	copy := **coordination
	copy.CreditBudgetHundredths = settings.TeamCreditBudgetHundredths
	*coordination = &copy
	return nil
}
