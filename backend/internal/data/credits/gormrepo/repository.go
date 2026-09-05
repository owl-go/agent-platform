package gormrepo

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/credits/application"
	"agent-platform/backend/internal/biz/credits/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

var _ application.Repository = (*Repository)(nil)

type accountRecord struct {
	UserID            string         `gorm:"column:user_id;primaryKey"`
	CreditDay         time.Time      `gorm:"column:credit_day;type:date"`
	Timezone          string         `gorm:"column:credit_day_timezone"`
	DailyAllocation   domain.Amount  `gorm:"column:daily_allocation_hundredths"`
	DailyRemaining    domain.Amount  `gorm:"column:daily_remaining_hundredths"`
	Persistent        domain.Amount  `gorm:"column:persistent_hundredths"`
	TodayConsumed     domain.Amount  `gorm:"column:today_consumed_hundredths"`
	PendingAllocation *domain.Amount `gorm:"column:pending_daily_allocation_hundredths"`
	PendingEffective  *time.Time     `gorm:"column:pending_effective_day;type:date"`
	UpdatedAt         time.Time      `gorm:"column:updated_at"`
	Version           int64          `gorm:"column:version"`
}

func (accountRecord) TableName() string { return "credit_accounts" }

type rateRecord struct {
	ID               string        `gorm:"column:id;primaryKey"`
	ProviderType     *string       `gorm:"column:provider_type"`
	Protocol         *string       `gorm:"column:api_protocol"`
	ModelID          *string       `gorm:"column:provider_model_id"`
	InputMultiplier  int64         `gorm:"column:input_multiplier_micros"`
	OutputMultiplier int64         `gorm:"column:output_multiplier_micros"`
	Fallback         domain.Amount `gorm:"column:fallback_hundredths"`
	PredecessorID    *string       `gorm:"column:predecessor_id"`
	CreatedBy        *string       `gorm:"column:created_by_user_id"`
	CreatedAt        time.Time     `gorm:"column:created_at"`
	SupersededAt     *time.Time    `gorm:"column:superseded_at"`
}

func (rateRecord) TableName() string { return "model_credit_rate_revisions" }

type admissionRecord struct {
	Source           string        `gorm:"column:source;primaryKey"`
	UserID           string        `gorm:"column:user_id"`
	ExecutionID      string        `gorm:"column:execution_id"`
	StagePosition    int           `gorm:"column:stage_position"`
	CreditDay        time.Time     `gorm:"column:credit_day;type:date"`
	Timezone         string        `gorm:"column:credit_day_timezone"`
	RateRevisionID   string        `gorm:"column:rate_revision_id"`
	InputMultiplier  int64         `gorm:"column:input_multiplier_micros"`
	OutputMultiplier int64         `gorm:"column:output_multiplier_micros"`
	Fallback         domain.Amount `gorm:"column:fallback_hundredths"`
	StartedAt        time.Time     `gorm:"column:started_at"`
	SettledAt        *time.Time    `gorm:"column:settled_at"`
}

func (admissionRecord) TableName() string { return "credit_stage_admissions" }

type ledgerRecord struct {
	ID               string        `gorm:"column:id;primaryKey"`
	UserID           string        `gorm:"column:user_id"`
	Type             string        `gorm:"column:entry_type"`
	Amount           domain.Amount `gorm:"column:amount_hundredths"`
	DailyDelta       domain.Amount `gorm:"column:daily_delta_hundredths"`
	PersistentDelta  domain.Amount `gorm:"column:persistent_delta_hundredths"`
	ResultingBalance domain.Amount `gorm:"column:resulting_balance_hundredths"`
	CreditDay        time.Time     `gorm:"column:credit_day;type:date"`
	Source           *string       `gorm:"column:source"`
	Reason           *string       `gorm:"column:reason"`
	ActorUserID      *string       `gorm:"column:actor_user_id"`
	Detail           []byte        `gorm:"column:detail;type:jsonb"`
	CreatedAt        time.Time     `gorm:"column:created_at"`
}

func (ledgerRecord) TableName() string { return "credit_ledger" }

func (repository *Repository) ResolveRate(ctx context.Context, key domain.ModelRateKey) (domain.ModelCreditRate, error) {
	var row rateRecord
	err := repository.db.WithContext(ctx).Where("provider_type = ? AND api_protocol = ? AND provider_model_id = ? AND superseded_at IS NULL", key.ProviderType, key.Protocol, key.ModelID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = repository.db.WithContext(ctx).Where("provider_type IS NULL AND superseded_at IS NULL").Take(&row).Error
	}
	if err != nil {
		return domain.ModelCreditRate{}, fmt.Errorf("resolve Model Credit Rate: %w", err)
	}
	return toRate(row), nil
}

func (repository *Repository) Admit(ctx context.Context, admission domain.Admission) (domain.Admission, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing admissionRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source = ?", admission.Source).Take(&existing).Error
		if err == nil {
			admission = toAdmission(existing)
			if existing.SettledAt != nil {
				return nil
			}
			return acquireLease(tx, admission.UserID, admission.Source, admission.StartedAt)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		account, err := repository.ensureAccountTx(tx, admission.UserID, admission.Timezone, admission.StartedAt)
		if err != nil {
			return err
		}
		if account.DailyRemaining+account.Persistent <= 0 {
			return domain.ErrInsufficientCredits
		}
		row, err := fromAdmission(admission)
		if err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return acquireLease(tx, admission.UserID, admission.Source, admission.StartedAt)
	})
	return admission, mapConflict(err)
}

func acquireLease(tx *gorm.DB, userID, source string, now time.Time) error {
	result := tx.Exec(`
		INSERT INTO credit_execution_leases (user_id, source, acquired_at) VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE
		SET source = EXCLUDED.source, acquired_at = EXCLUDED.acquired_at
		WHERE credit_execution_leases.acquired_at < EXCLUDED.acquired_at - INTERVAL '1 minute'`, userID, source, now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var current string
		if err := tx.Table("credit_execution_leases").Select("source").Where("user_id = ?", userID).Scan(&current).Error; err == nil && current == source {
			return nil
		}
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) Abort(ctx context.Context, admission domain.Admission) error {
	return repository.db.WithContext(ctx).Exec("DELETE FROM credit_execution_leases WHERE user_id = ? AND source = ?", admission.UserID, admission.Source).Error
}

func (repository *Repository) Settle(ctx context.Context, settlement domain.Settlement) (domain.Consumption, error) {
	var result domain.Consumption
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = repository.SettleTx(tx, settlement)
		return err
	})
	return result, mapConflict(err)
}

// SettleTx lets the Workspace adapter commit the terminal execution state and
// its Credit Ledger entry in the same PostgreSQL transaction.
func (repository *Repository) SettleTx(tx *gorm.DB, settlement domain.Settlement) (domain.Consumption, error) {
	var result domain.Consumption
	var admission admissionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source = ? AND user_id = ?", settlement.Source, settlement.Admission.UserID).Take(&admission).Error; err != nil {
		return result, err
	}
	var existing ledgerRecord
	if err := tx.Where("user_id = ? AND source = ?", admission.UserID, admission.Source).Take(&existing).Error; err == nil {
		return result, json.Unmarshal(existing.Detail, &result)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}
	var account accountRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", admission.UserID).Take(&account).Error; err != nil {
		return result, err
	}
	dailyDebit := settlement.Amount
	if dailyDebit > account.DailyRemaining {
		dailyDebit = account.DailyRemaining
	}
	persistentDebit := settlement.Amount - dailyDebit
	account.DailyRemaining -= dailyDebit
	account.Persistent -= persistentDebit
	account.TodayConsumed += settlement.Amount
	account.UpdatedAt, account.Version = settlement.SettledAt, account.Version+1
	result = domain.Consumption{Amount: settlement.Amount, Estimated: settlement.Estimated, Usage: settlement.Usage, Rate: settlement.Admission.Rate}
	detail, err := json.Marshal(result)
	if err != nil {
		return result, err
	}
	source := admission.Source
	entry := ledgerRecord{ID: uuid.NewString(), UserID: admission.UserID, Type: "consumption", Amount: -settlement.Amount, DailyDelta: -dailyDebit, PersistentDelta: -persistentDebit, ResultingBalance: account.DailyRemaining + account.Persistent, CreditDay: admission.CreditDay, Source: &source, Detail: detail, CreatedAt: settlement.SettledAt}
	if err := tx.Create(&entry).Error; err != nil {
		return result, err
	}
	if err := tx.Save(&account).Error; err != nil {
		return result, err
	}
	if err := tx.Model(&admission).Update("settled_at", settlement.SettledAt).Error; err != nil {
		return result, err
	}
	return result, tx.Exec("DELETE FROM credit_execution_leases WHERE user_id = ? AND source = ?", admission.UserID, admission.Source).Error
}

func (repository *Repository) Balance(ctx context.Context, userID, timezone string, now time.Time) (domain.Balance, error) {
	var balance domain.Balance
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := repository.ensureAccountTx(tx, userID, timezone, now)
		if err != nil {
			return err
		}
		balance, err = toBalance(account, now)
		return err
	})
	return balance, err
}

func (repository *Repository) ensureAccountTx(tx *gorm.DB, userID, requestedTimezone string, now time.Time) (accountRecord, error) {
	if requestedTimezone == "" {
		requestedTimezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(requestedTimezone)
	if err != nil {
		return accountRecord{}, err
	}
	requestedDay := dateAt(now, location)
	created := accountRecord{UserID: userID, CreditDay: requestedDay, Timezone: requestedTimezone, DailyAllocation: domain.DefaultDailyAllocation, DailyRemaining: domain.DefaultDailyAllocation, UpdatedAt: now, Version: 1}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created)
	if result.Error != nil {
		return accountRecord{}, result.Error
	}
	if result.RowsAffected == 1 {
		source := "daily:" + requestedDay.Format(time.DateOnly)
		reason := "daily allocation"
		entry := ledgerRecord{ID: uuid.NewString(), UserID: userID, Type: "daily_allocation", Amount: domain.DefaultDailyAllocation, DailyDelta: domain.DefaultDailyAllocation, ResultingBalance: domain.DefaultDailyAllocation, CreditDay: requestedDay, Source: &source, Reason: &reason, Detail: []byte(`{}`), CreatedAt: now}
		if err := tx.Create(&entry).Error; err != nil {
			return accountRecord{}, err
		}
		return created, nil
	}
	var account accountRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).Take(&account).Error; err != nil {
		return accountRecord{}, err
	}
	currentLocation, err := time.LoadLocation(account.Timezone)
	if err != nil {
		return accountRecord{}, err
	}
	if !dateAt(now, currentLocation).After(account.CreditDay) {
		return account, nil
	}
	newDay := requestedDay
	if !newDay.After(account.CreditDay) {
		newDay = account.CreditDay.AddDate(0, 0, 1)
	}
	allocation := account.DailyAllocation
	if account.PendingAllocation != nil && account.PendingEffective != nil && !newDay.Before(*account.PendingEffective) {
		allocation = *account.PendingAllocation
		account.PendingAllocation, account.PendingEffective = nil, nil
	}
	if account.DailyRemaining != 0 {
		reason := "unused Daily Credits expired"
		entry := ledgerRecord{ID: uuid.NewString(), UserID: userID, Type: "daily_expiry", Amount: -account.DailyRemaining, DailyDelta: -account.DailyRemaining, ResultingBalance: account.Persistent, CreditDay: account.CreditDay, Reason: &reason, Detail: []byte(`{}`), CreatedAt: now}
		if err := tx.Create(&entry).Error; err != nil {
			return accountRecord{}, err
		}
	}
	account.CreditDay, account.Timezone, account.DailyAllocation = newDay, requestedTimezone, allocation
	account.DailyRemaining, account.TodayConsumed = allocation, 0
	account.UpdatedAt, account.Version = now, account.Version+1
	source := "daily:" + newDay.Format(time.DateOnly)
	reason := "daily allocation"
	entry := ledgerRecord{ID: uuid.NewString(), UserID: userID, Type: "daily_allocation", Amount: allocation, DailyDelta: allocation, ResultingBalance: allocation + account.Persistent, CreditDay: newDay, Source: &source, Reason: &reason, Detail: []byte(`{}`), CreatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
		return accountRecord{}, err
	}
	if err := tx.Save(&account).Error; err != nil {
		return accountRecord{}, err
	}
	return account, nil
}

func (repository *Repository) Ledger(ctx context.Context, userID, cursor string, limit int) (domain.LedgerPage, error) {
	query := repository.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC, id DESC").Limit(limit + 1)
	if cursor != "" {
		var anchor ledgerRecord
		if err := repository.db.WithContext(ctx).Select("id", "created_at").Where("id = ? AND user_id = ?", cursor, userID).Take(&anchor).Error; err != nil {
			return domain.LedgerPage{}, err
		}
		query = query.Where("(created_at, id) < (?, ?)", anchor.CreatedAt, anchor.ID)
	}
	var rows []ledgerRecord
	if err := query.Find(&rows).Error; err != nil {
		return domain.LedgerPage{}, err
	}
	page := domain.LedgerPage{Items: make([]domain.LedgerEntry, 0, min(limit, len(rows)))}
	if len(rows) > limit {
		page.NextCursor = rows[limit-1].ID
		rows = rows[:limit]
	}
	for _, row := range rows {
		page.Items = append(page.Items, toLedger(row))
	}
	return page, nil
}

func (repository *Repository) ConfigureDailyAllocation(ctx context.Context, userID string, amount domain.Amount, timezone string, now time.Time) (domain.Balance, error) {
	var balance domain.Balance
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := repository.ensureAccountTx(tx, userID, timezone, now)
		if err != nil {
			return err
		}
		location, _ := time.LoadLocation(account.Timezone)
		effective := dateAt(nextMidnight(now, location), location)
		account.PendingAllocation, account.PendingEffective = &amount, &effective
		account.UpdatedAt, account.Version = now, account.Version+1
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		balance, err = toBalance(account, now)
		return err
	})
	return balance, err
}

func (repository *Repository) Adjust(ctx context.Context, userID, administratorID, requestID string, amount domain.Amount, reason, timezone string, now time.Time) (domain.Balance, error) {
	var balance domain.Balance
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := repository.ensureAccountTx(tx, userID, timezone, now)
		if err != nil {
			return err
		}
		source := "adjustment:" + requestID
		var existing int64
		if err := tx.Model(&ledgerRecord{}).Where("user_id = ? AND source = ?", userID, source).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			balance, err = toBalance(account, now)
			return err
		}
		account.Persistent += amount
		account.UpdatedAt, account.Version = now, account.Version+1
		entry := ledgerRecord{ID: uuid.NewString(), UserID: userID, Type: "adjustment", Amount: amount, PersistentDelta: amount, ResultingBalance: account.DailyRemaining + account.Persistent, CreditDay: account.CreditDay, Source: &source, Reason: &reason, ActorUserID: &administratorID, Detail: []byte(`{}`), CreatedAt: now}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		balance, err = toBalance(account, now)
		return err
	})
	return balance, err
}

type batchRecord struct {
	ID        string        `gorm:"column:id;primaryKey"`
	CreatedBy string        `gorm:"column:created_by_user_id"`
	Count     int           `gorm:"column:code_count"`
	Value     domain.Amount `gorm:"column:value_hundredths"`
	ExpiresAt *time.Time    `gorm:"column:expires_at"`
	CreatedAt time.Time     `gorm:"column:created_at"`
}

func (batchRecord) TableName() string { return "redemption_code_batches" }

type codeRecord struct {
	ID         string     `gorm:"column:id;primaryKey"`
	BatchID    string     `gorm:"column:batch_id"`
	Identifier string     `gorm:"column:code_identifier"`
	Verifier   []byte     `gorm:"column:verifier"`
	VoidedAt   *time.Time `gorm:"column:voided_at"`
	RedeemedBy *string    `gorm:"column:redeemed_by_user_id"`
	RedeemedAt *time.Time `gorm:"column:redeemed_at"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
}

func (codeRecord) TableName() string { return "redemption_codes" }

func (repository *Repository) CreateRedemptionBatch(ctx context.Context, administratorID string, value domain.Amount, expiresAt *time.Time, secrets []application.RedemptionSecret, now time.Time) (domain.RedemptionBatch, error) {
	batch := domain.RedemptionBatch{ID: uuid.NewString(), Count: len(secrets), Value: value, ExpiresAt: expiresAt, CreatedAt: now, Codes: make([]domain.RedemptionCode, 0, len(secrets))}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batchRecord{ID: batch.ID, CreatedBy: administratorID, Count: batch.Count, Value: value, ExpiresAt: expiresAt, CreatedAt: now}).Error; err != nil {
			return err
		}
		for _, secret := range secrets {
			id := uuid.NewString()
			if err := tx.Create(&codeRecord{ID: id, BatchID: batch.ID, Identifier: secret.Identifier, Verifier: secret.Verifier[:], CreatedAt: now}).Error; err != nil {
				return err
			}
			batch.Codes = append(batch.Codes, domain.RedemptionCode{ID: id, Identifier: secret.Identifier, Plaintext: secret.Plaintext, State: "available"})
		}
		return nil
	})
	return batch, err
}

func (repository *Repository) Redeem(ctx context.Context, userID, identifier string, verifier [32]byte, timezone string, now time.Time) (domain.Balance, error) {
	var balance domain.Balance
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var code codeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_identifier = ?", identifier).Take(&code).Error; err != nil {
			return domain.ErrCodeUnavailable
		}
		var batch batchRecord
		if err := tx.Where("id = ?", code.BatchID).Take(&batch).Error; err != nil {
			return domain.ErrCodeUnavailable
		}
		if subtle.ConstantTimeCompare(code.Verifier, verifier[:]) != 1 || code.VoidedAt != nil || code.RedeemedAt != nil || (batch.ExpiresAt != nil && !now.Before(*batch.ExpiresAt)) {
			return domain.ErrCodeUnavailable
		}
		account, err := repository.ensureAccountTx(tx, userID, timezone, now)
		if err != nil {
			return err
		}
		account.Persistent += batch.Value
		account.UpdatedAt, account.Version = now, account.Version+1
		code.RedeemedBy, code.RedeemedAt = &userID, &now
		source := "redemption:" + code.ID
		reason := "Redemption Code"
		entry := ledgerRecord{ID: uuid.NewString(), UserID: userID, Type: "redemption", Amount: batch.Value, PersistentDelta: batch.Value, ResultingBalance: account.DailyRemaining + account.Persistent, CreditDay: account.CreditDay, Source: &source, Reason: &reason, Detail: []byte(`{}`), CreatedAt: now}
		if err := tx.Save(&code).Error; err != nil {
			return err
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		if err := tx.Save(&account).Error; err != nil {
			return err
		}
		balance, err = toBalance(account, now)
		return err
	})
	if errors.Is(err, domain.ErrCodeUnavailable) || errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Balance{}, domain.ErrCodeUnavailable
	}
	return balance, err
}

type codeStatusRecord struct {
	ID, BatchID, Identifier         string
	Value                           domain.Amount
	ExpiresAt, VoidedAt, RedeemedAt *time.Time
	CreatedAt                       time.Time
}

func (repository *Repository) ListRedemptionCodes(ctx context.Context, cursor string, limit int, now time.Time) (domain.RedemptionCodePage, error) {
	query := repository.db.WithContext(ctx).Table("redemption_codes code").
		Select("code.id, code.batch_id, code.code_identifier AS identifier, batch.value_hundredths AS value, batch.expires_at, code.voided_at, code.redeemed_at, code.created_at").
		Joins("JOIN redemption_code_batches batch ON batch.id = code.batch_id").
		Order("code.created_at DESC, code.id DESC").Limit(limit + 1)
	if cursor != "" {
		var anchor codeRecord
		if err := repository.db.WithContext(ctx).Select("id", "created_at").Where("id = ?", cursor).Take(&anchor).Error; err != nil {
			return domain.RedemptionCodePage{}, err
		}
		query = query.Where("(code.created_at, code.id) < (?, ?)", anchor.CreatedAt, anchor.ID)
	}
	var rows []codeStatusRecord
	if err := query.Scan(&rows).Error; err != nil {
		return domain.RedemptionCodePage{}, err
	}
	page := domain.RedemptionCodePage{Items: make([]domain.RedemptionCodeStatus, 0, min(limit, len(rows)))}
	if len(rows) > limit {
		page.NextCursor = rows[limit-1].ID
		rows = rows[:limit]
	}
	for _, row := range rows {
		page.Items = append(page.Items, toRedemptionCodeStatus(row, now))
	}
	return page, nil
}

func (repository *Repository) VoidRedemptionCode(ctx context.Context, codeID string, now time.Time) (domain.RedemptionCodeStatus, error) {
	var status domain.RedemptionCodeStatus
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var code codeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", codeID).Take(&code).Error; err != nil {
			return domain.ErrCodeUnavailable
		}
		if code.RedeemedAt != nil {
			return domain.ErrCodeUnavailable
		}
		if code.VoidedAt == nil {
			code.VoidedAt = &now
			if err := tx.Save(&code).Error; err != nil {
				return err
			}
		}
		var row codeStatusRecord
		if err := tx.Table("redemption_codes code").Select("code.id, code.batch_id, code.code_identifier AS identifier, batch.value_hundredths AS value, batch.expires_at, code.voided_at, code.redeemed_at, code.created_at").Joins("JOIN redemption_code_batches batch ON batch.id = code.batch_id").Where("code.id = ?", codeID).Take(&row).Error; err != nil {
			return err
		}
		status = toRedemptionCodeStatus(row, now)
		return nil
	})
	return status, err
}

func toRedemptionCodeStatus(row codeStatusRecord, now time.Time) domain.RedemptionCodeStatus {
	state := "available"
	if row.VoidedAt != nil {
		state = "void"
	} else if row.RedeemedAt != nil {
		state = "redeemed"
	} else if row.ExpiresAt != nil && !now.Before(*row.ExpiresAt) {
		state = "expired"
	}
	return domain.RedemptionCodeStatus{ID: row.ID, BatchID: row.BatchID, Identifier: row.Identifier, State: state, Value: row.Value, ExpiresAt: row.ExpiresAt, RedeemedAt: row.RedeemedAt, VoidedAt: row.VoidedAt, CreatedAt: row.CreatedAt}
}

func (repository *Repository) ListRates(ctx context.Context) ([]domain.RateRevision, error) {
	var rows []rateRecord
	if err := repository.db.WithContext(ctx).Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.RateRevision, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.RateRevision{Rate: toRate(row), SupersededAt: row.SupersededAt})
	}
	return result, nil
}

func (repository *Repository) CreateRateRevision(ctx context.Context, administratorID string, rate domain.ModelCreditRate, expectedRevision string, now time.Time) (domain.ModelCreditRate, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("superseded_at IS NULL")
		if rate.Key == nil {
			query = query.Where("provider_type IS NULL")
		} else {
			query = query.Where("provider_type = ? AND api_protocol = ? AND provider_model_id = ?", rate.Key.ProviderType, rate.Key.Protocol, rate.Key.ModelID)
		}
		var current rateRecord
		err := query.Take(&current).Error
		if err == nil {
			if expectedRevision == "" || current.ID != expectedRevision {
				return domain.ErrConflict
			}
			if err := tx.Model(&current).Update("superseded_at", now).Error; err != nil {
				return err
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		} else if expectedRevision != "" {
			return domain.ErrConflict
		}
		row := rateRecord{ID: uuid.NewString(), InputMultiplier: rate.InputMultiplierMicros, OutputMultiplier: rate.OutputMultiplierMicros, Fallback: rate.Fallback, CreatedBy: &administratorID, CreatedAt: now}
		if current.ID != "" {
			row.PredecessorID = &current.ID
		}
		if rate.Key != nil {
			row.ProviderType, row.Protocol, row.ModelID = &rate.Key.ProviderType, &rate.Key.Protocol, &rate.Key.ModelID
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		rate = toRate(row)
		return nil
	})
	return rate, mapConflict(err)
}

func fromAdmission(value domain.Admission) (admissionRecord, error) {
	day, err := time.Parse(time.DateOnly, value.CreditDay)
	if err != nil {
		return admissionRecord{}, err
	}
	return admissionRecord{Source: value.Source, UserID: value.UserID, ExecutionID: value.ExecutionID, StagePosition: value.StagePosition, CreditDay: day, Timezone: value.Timezone, RateRevisionID: value.Rate.RevisionID, InputMultiplier: value.Rate.InputMultiplierMicros, OutputMultiplier: value.Rate.OutputMultiplierMicros, Fallback: value.Rate.Fallback, StartedAt: value.StartedAt}, nil
}

func toAdmission(row admissionRecord) domain.Admission {
	return domain.Admission{UserID: row.UserID, ExecutionID: row.ExecutionID, StagePosition: row.StagePosition, Source: row.Source, Timezone: row.Timezone, CreditDay: row.CreditDay.Format(time.DateOnly), StartedAt: row.StartedAt, Rate: domain.ModelCreditRate{RevisionID: row.RateRevisionID, InputMultiplierMicros: row.InputMultiplier, OutputMultiplierMicros: row.OutputMultiplier, Fallback: row.Fallback}}
}

func toRate(row rateRecord) domain.ModelCreditRate {
	rate := domain.ModelCreditRate{RevisionID: row.ID, InputMultiplierMicros: row.InputMultiplier, OutputMultiplierMicros: row.OutputMultiplier, Fallback: row.Fallback, CreatedAt: row.CreatedAt}
	if row.ProviderType != nil {
		rate.Key = &domain.ModelRateKey{ProviderType: *row.ProviderType, Protocol: *row.Protocol, ModelID: *row.ModelID}
	}
	return rate
}

func toBalance(row accountRecord, now time.Time) (domain.Balance, error) {
	location, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return domain.Balance{}, err
	}
	balance := domain.Balance{UserID: row.UserID, CreditDay: row.CreditDay.Format(time.DateOnly), Timezone: row.Timezone, DailyAllocation: row.DailyAllocation, DailyRemaining: row.DailyRemaining, Persistent: row.Persistent, TodayConsumed: row.TodayConsumed, Total: row.DailyRemaining + row.Persistent, PendingDailyAllocation: row.PendingAllocation, Version: row.Version, NextAllocationAt: nextMidnight(now, location)}
	if row.PendingEffective != nil {
		balance.PendingEffectiveDay = row.PendingEffective.Format(time.DateOnly)
	}
	return balance, nil
}

func toLedger(row ledgerRecord) domain.LedgerEntry {
	reason := ""
	if row.Reason != nil {
		reason = *row.Reason
	}
	return domain.LedgerEntry{ID: row.ID, Type: row.Type, Amount: row.Amount, ResultingBalance: row.ResultingBalance, CreditDay: row.CreditDay.Format(time.DateOnly), Reason: reason, CreatedAt: row.CreatedAt}
}

func dateAt(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}

func nextMidnight(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location).UTC()
}

func mapConflict(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, gorm.ErrForeignKeyViolated) {
		return domain.ErrConflict
	}
	return err
}
