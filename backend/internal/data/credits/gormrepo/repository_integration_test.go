package gormrepo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/credits/application"
	"agent-platform/backend/internal/biz/credits/domain"
	creditrepo "agent-platform/backend/internal/data/credits/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresCreditLifecycleIsAtomicAndIdempotent(t *testing.T) {
	dsn := os.Getenv("CREDITS_TEST_DSN")
	if dsn == "" {
		t.Skip("CREDITS_TEST_DSN is not set")
	}
	ctx := context.Background()
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	database, err := gormdb.Open(ctx, gormdb.Config{DSN: dsn, MaxOpenConnections: 8, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	userID := uuid.NewString()
	if err := database.ORM().Exec("INSERT INTO users (id, oidc_subject, username, email, display_name, administrator) VALUES (?, ?, ?, ?, ?, true)", userID, "subject", "admin", "admin@example.test", "Admin").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	repository := creditrepo.New(database.ORM())
	service, err := application.New(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	balance, err := service.Balance(ctx, userID, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if balance.Total != 60_000 || balance.DailyRemaining != 60_000 {
		t.Fatalf("initial balance = %+v", balance)
	}
	if _, err := service.Adjust(ctx, userID, userID, "debt-boundary", "Asia/Shanghai", -59_999, "test debt boundary"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Adjust(ctx, userID, userID, "debt-boundary", "Asia/Shanghai", -59_999, "test debt boundary"); err != nil {
		t.Fatal(err)
	}
	var adjustmentCount int64
	if err := database.ORM().Table("credit_ledger").Where("user_id = ? AND source = ? AND actor_user_id = ?", userID, "adjustment:debt-boundary", userID).Count(&adjustmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if adjustmentCount != 1 {
		t.Fatalf("idempotent audited adjustments = %d, want 1", adjustmentCount)
	}
	admission, err := service.Admit(ctx, application.AdmissionRequest{UserID: userID, ExecutionID: "session-message-1", StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Settle(ctx, application.SettlementRequest{Admission: admission, Usage: domain.Usage{}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Settle(ctx, application.SettlementRequest{Admission: admission, Usage: domain.Usage{}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Amount != 1_000 || second.Amount != first.Amount {
		t.Fatalf("idempotent settlement = %+v then %+v", first, second)
	}
	balance, err = service.Balance(ctx, userID, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if balance.Total != -999 || balance.TodayConsumed != 1_000 {
		t.Fatalf("settled balance = %+v", balance)
	}
	if _, err := service.Admit(ctx, application.AdmissionRequest{UserID: userID, ExecutionID: "session-message-2", StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-test"}); !errors.Is(err, domain.ErrInsufficientCredits) {
		t.Fatalf("second admission error = %v", err)
	}

	batch, err := service.CreateRedemptionBatch(ctx, userID, 2, 2_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Codes) != 2 || batch.Codes[0].Plaintext == "" {
		t.Fatalf("batch = %+v", batch)
	}
	voided, err := service.VoidRedemptionCode(ctx, batch.Codes[1].ID)
	if err != nil || voided.State != "void" || voided.Identifier == "" {
		t.Fatalf("voided code = %+v, %v", voided, err)
	}
	if _, err := service.Redeem(ctx, userID, "Asia/Shanghai", batch.Codes[1].Plaintext); !errors.Is(err, domain.ErrCodeUnavailable) {
		t.Fatalf("voided redemption error = %v", err)
	}
	codePage, err := service.ListRedemptionCodes(ctx, "", 1)
	if err != nil || len(codePage.Items) != 1 || codePage.NextCursor == "" {
		t.Fatalf("redemption code page = %+v, %v", codePage, err)
	}
	balance, err = service.Redeem(ctx, userID, "Asia/Shanghai", batch.Codes[0].Plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if balance.Total != 1_001 {
		t.Fatalf("redeemed balance = %+v", balance)
	}
	if _, err := service.Redeem(ctx, userID, "Asia/Shanghai", batch.Codes[0].Plaintext); !errors.Is(err, domain.ErrCodeUnavailable) {
		t.Fatalf("repeated redemption error = %v", err)
	}
	staleAdmission, err := service.Admit(ctx, application.AdmissionRequest{UserID: userID, ExecutionID: "crashed-worker", StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.ORM().Exec("UPDATE credit_execution_leases SET acquired_at = ? WHERE user_id = ?", now.Add(-2*time.Minute), userID).Error; err != nil {
		t.Fatal(err)
	}
	recoveredAdmission, err := service.Admit(ctx, application.AdmissionRequest{UserID: userID, ExecutionID: "recovered-worker", StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-test"})
	if err != nil {
		t.Fatalf("take over stale execution lease: %v", err)
	}
	if recoveredAdmission.Source == staleAdmission.Source {
		t.Fatalf("stale lease was not replaced: %+v", recoveredAdmission)
	}
	if err := service.Abort(ctx, recoveredAdmission); err != nil {
		t.Fatal(err)
	}
	atomicAdmission, err := service.Admit(ctx, application.AdmissionRequest{UserID: userID, ExecutionID: "atomic-terminal", StagePosition: 1, Timezone: "Asia/Shanghai", ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	forcedRollback := errors.New("terminal write failed")
	err = database.ORM().Transaction(func(tx *gorm.DB) error {
		_, settleErr := repository.SettleTx(tx, domain.Settlement{Admission: atomicAdmission, Source: atomicAdmission.Source, Amount: 100, Usage: domain.Usage{InputTokens: 10_000, Known: true}, SettledAt: now})
		if settleErr != nil {
			return settleErr
		}
		return forcedRollback
	})
	if !errors.Is(err, forcedRollback) {
		t.Fatalf("rollback error = %v", err)
	}
	balance, err = service.Balance(ctx, userID, "Asia/Shanghai")
	if err != nil || balance.Total != 1_001 {
		t.Fatalf("rolled-back settlement balance = %+v, %v", balance, err)
	}
	if err := service.Abort(ctx, atomicAdmission); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfigureDailyAllocation(ctx, userID, "Asia/Shanghai", 0); err != nil {
		t.Fatal(err)
	}
	now = time.Date(2026, 9, 4, 17, 0, 0, 0, time.UTC)
	balance, err = service.Balance(ctx, userID, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if balance.DailyAllocation != 0 || balance.DailyRemaining != 0 || balance.Persistent != -57_999 || balance.Total != -57_999 {
		t.Fatalf("next-day debt carry = %+v", balance)
	}
}
