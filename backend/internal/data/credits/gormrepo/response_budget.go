package gormrepo

import (
	"agent-platform/backend/internal/biz/credits/domain"
	"gorm.io/gorm"
)

// Called under the User's Credit Account row lock, shared with reservations and
// settlements. The response limit stays frozen even if an unused call aborts.
func admitResponseBudget(tx *gorm.DB, admission domain.Admission) error {
	if admission.ResponseID == "" {
		return nil
	}
	if err := tx.Exec(`INSERT INTO credit_response_budgets(user_id,response_id,budget_hundredths) VALUES(?,?,?) ON CONFLICT DO NOTHING`, admission.UserID, admission.ResponseID, admission.ResponseBudget).Error; err != nil {
		return err
	}
	var budget domain.Amount
	if err := tx.Table("credit_response_budgets").Select("budget_hundredths").Where("user_id = ? AND response_id = ?", admission.UserID, admission.ResponseID).Scan(&budget).Error; err != nil {
		return err
	}
	if budget != admission.ResponseBudget {
		return domain.ErrConflict
	}
	if budget == 0 {
		return nil
	}
	var used domain.Amount
	if err := tx.Raw(`SELECT COALESCE(SUM(CASE WHEN call.settled_at IS NULL THEN call.reserved_hundredths ELSE -COALESCE(ledger.amount_hundredths,0) END),0)
        FROM credit_stage_admissions call LEFT JOIN credit_ledger ledger ON ledger.user_id = call.user_id AND ledger.source = call.source AND ledger.entry_type = 'consumption'
        WHERE call.user_id = ? AND call.response_id = ?`, admission.UserID, admission.ResponseID).Scan(&used).Error; err != nil {
		return err
	}
	if used >= budget || admission.Rate.Fallback > budget-used {
		return domain.ErrResponseBudgetExhausted
	}
	return nil
}
