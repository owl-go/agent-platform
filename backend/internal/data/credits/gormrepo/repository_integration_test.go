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
	if _, err := service.Adjust(ctx, userID, "Asia/Shanghai", -59_999, "test debt boundary"); err != nil {
		t.Fatal(err)
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

	batch, err := service.CreateRedemptionBatch(ctx, userID, 1, 2_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Codes) != 1 || batch.Codes[0].Plaintext == "" {
		t.Fatalf("batch = %+v", batch)
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
