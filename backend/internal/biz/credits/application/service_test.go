package application_test

import (
	"context"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/credits/application"
	"agent-platform/backend/internal/biz/credits/domain"
)

func TestServiceSettlesReportedUsage(t *testing.T) {
	repository := &recordingRepository{
		rate: domain.ModelCreditRate{
			RevisionID:             "default-v1",
			InputMultiplierMicros:  1_000_000,
			OutputMultiplierMicros: 1_000_000,
			Fallback:               1000,
		},
	}
	service, err := application.New(repository, func() time.Time {
		return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}

	admission, err := service.Admit(context.Background(), application.AdmissionRequest{
		UserID: "user-1", ExecutionID: "message-42", StagePosition: 1, Timezone: "Asia/Shanghai",
		ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-5",
	})
	if err != nil {
		t.Fatal(err)
	}
	consumption, err := service.Settle(context.Background(), application.SettlementRequest{
		Admission: admission, Usage: domain.Usage{InputTokens: 12_345, OutputTokens: 5_000, Known: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	if consumption.Amount != 173 {
		t.Fatalf("consumption = %d hundredths, want 173", consumption.Amount)
	}
	if repository.settlement.Source != "message-42:1" || repository.settlement.Amount != 173 {
		t.Fatalf("settlement = %#v", repository.settlement)
	}
}

func TestAdmissionUsesFrozenStageRate(t *testing.T) {
	repository := &recordingRepository{rate: domain.ModelCreditRate{RevisionID: "current", Fallback: 999}}
	service, err := application.New(repository, func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	frozen := domain.ModelCreditRate{RevisionID: "frozen", InputMultiplierMicros: 2_000_000, OutputMultiplierMicros: 3_000_000, Fallback: 500}
	admission, err := service.Admit(context.Background(), application.AdmissionRequest{
		UserID: "user-1", ExecutionID: "run-1", StagePosition: 1, Timezone: "Asia/Shanghai",
		ProviderType: "openai", Protocol: "openai_responses", ModelID: "gpt-5", FrozenRate: &frozen,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repository.resolveCalls != 0 || admission.Rate.RevisionID != "frozen" {
		t.Fatalf("admission rate = %#v, resolve calls = %d", admission.Rate, repository.resolveCalls)
	}
}

type recordingRepository struct {
	rate         domain.ModelCreditRate
	resolveCalls int
	settlement   domain.Settlement
}

func (repository *recordingRepository) ResolveRate(context.Context, domain.ModelRateKey) (domain.ModelCreditRate, error) {
	repository.resolveCalls++
	return repository.rate, nil
}

func (repository *recordingRepository) Admit(_ context.Context, request domain.Admission) (domain.Admission, error) {
	return request, nil
}

func (repository *recordingRepository) Settle(_ context.Context, settlement domain.Settlement) (domain.Consumption, error) {
	repository.settlement = settlement
	return domain.Consumption{Amount: settlement.Amount, Estimated: settlement.Estimated}, nil
}

func (repository *recordingRepository) Abort(context.Context, domain.Admission) error { return nil }

func (repository *recordingRepository) Balance(context.Context, string, string, time.Time) (domain.Balance, error) {
	return domain.Balance{}, nil
}

func (repository *recordingRepository) Ledger(context.Context, string, string, int) (domain.LedgerPage, error) {
	return domain.LedgerPage{}, nil
}

func (repository *recordingRepository) ConfigureDailyAllocation(context.Context, string, domain.Amount, string, time.Time) (domain.Balance, error) {
	return domain.Balance{}, nil
}

func (repository *recordingRepository) Adjust(context.Context, string, domain.Amount, string, string, time.Time) (domain.Balance, error) {
	return domain.Balance{}, nil
}

func (repository *recordingRepository) CreateRedemptionBatch(context.Context, string, domain.Amount, *time.Time, []application.RedemptionSecret, time.Time) (domain.RedemptionBatch, error) {
	return domain.RedemptionBatch{}, nil
}

func (repository *recordingRepository) Redeem(context.Context, string, string, [32]byte, string, time.Time) (domain.Balance, error) {
	return domain.Balance{}, nil
}

func (repository *recordingRepository) ListRates(context.Context) ([]domain.RateRevision, error) {
	return nil, nil
}

func (repository *recordingRepository) CreateRateRevision(context.Context, string, domain.ModelCreditRate, string, time.Time) (domain.ModelCreditRate, error) {
	return domain.ModelCreditRate{}, nil
}
