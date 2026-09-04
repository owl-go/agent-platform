package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/credits/domain"
)

type Repository interface {
	ResolveRate(context.Context, domain.ModelRateKey) (domain.ModelCreditRate, error)
	Admit(context.Context, domain.Admission) (domain.Admission, error)
	Settle(context.Context, domain.Settlement) (domain.Consumption, error)
	Abort(context.Context, domain.Admission) error
	Balance(context.Context, string, string, time.Time) (domain.Balance, error)
	Ledger(context.Context, string, string, int) (domain.LedgerPage, error)
	ConfigureDailyAllocation(context.Context, string, domain.Amount, string, time.Time) (domain.Balance, error)
	Adjust(context.Context, string, domain.Amount, string, string, time.Time) (domain.Balance, error)
	CreateRedemptionBatch(context.Context, string, domain.Amount, *time.Time, []RedemptionSecret, time.Time) (domain.RedemptionBatch, error)
	Redeem(context.Context, string, string, [32]byte, string, time.Time) (domain.Balance, error)
	ListRates(context.Context) ([]domain.RateRevision, error)
	CreateRateRevision(context.Context, string, domain.ModelCreditRate, string, time.Time) (domain.ModelCreditRate, error)
}

type RedemptionSecret struct {
	Identifier string
	Verifier   [32]byte
	Plaintext  string
}

type Clock func() time.Time

type Service struct {
	repository Repository
	now        Clock
}

func New(repository Repository, now Clock) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("Credits Repository is required")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, now: now}, nil
}

type AdmissionRequest struct {
	UserID        string
	ExecutionID   string
	StagePosition int
	Timezone      string
	ProviderType  string
	Protocol      string
	ModelID       string
	FrozenRate    *domain.ModelCreditRate
}

func (service *Service) Admit(ctx context.Context, request AdmissionRequest) (domain.Admission, error) {
	if strings.TrimSpace(request.UserID) == "" || strings.TrimSpace(request.ExecutionID) == "" || request.StagePosition < 1 {
		return domain.Admission{}, fmt.Errorf("%w: User, execution, and Stage position are required", domain.ErrInvalid)
	}
	location, err := time.LoadLocation(request.Timezone)
	if err != nil {
		return domain.Admission{}, fmt.Errorf("%w: invalid Credit time zone", domain.ErrInvalid)
	}
	key := domain.ModelRateKey{ProviderType: request.ProviderType, Protocol: request.Protocol, ModelID: request.ModelID}
	if err := key.Validate(); err != nil {
		return domain.Admission{}, err
	}
	var rate domain.ModelCreditRate
	if request.FrozenRate != nil {
		rate = *request.FrozenRate
		if err := rate.Validate(); err != nil {
			return domain.Admission{}, err
		}
	} else {
		rate, err = service.repository.ResolveRate(ctx, key)
		if err != nil {
			return domain.Admission{}, err
		}
	}
	startedAt := service.now().UTC()
	admission := domain.Admission{
		UserID: request.UserID, ExecutionID: request.ExecutionID, StagePosition: request.StagePosition,
		Source: fmt.Sprintf("%s:%d", request.ExecutionID, request.StagePosition), Timezone: request.Timezone,
		CreditDay: startedAt.In(location).Format(time.DateOnly), StartedAt: startedAt, Rate: rate,
	}
	return service.repository.Admit(ctx, admission)
}

type SettlementRequest struct {
	Admission domain.Admission
	Usage     domain.Usage
}

func (service *Service) Settle(ctx context.Context, request SettlementRequest) (domain.Consumption, error) {
	consumption, err := domain.CalculateConsumption(request.Usage, request.Admission.Rate)
	if err != nil {
		return domain.Consumption{}, err
	}
	return service.repository.Settle(ctx, domain.Settlement{
		Admission: request.Admission, Source: request.Admission.Source, Amount: consumption.Amount,
		Estimated: consumption.Estimated, Usage: request.Usage, SettledAt: service.now().UTC(),
	})
}

func (service *Service) Abort(ctx context.Context, admission domain.Admission) error {
	return service.repository.Abort(ctx, admission)
}

func (service *Service) Balance(ctx context.Context, userID, timezone string) (domain.Balance, error) {
	if strings.TrimSpace(userID) == "" {
		return domain.Balance{}, fmt.Errorf("%w: User is required", domain.ErrInvalid)
	}
	if timezone != "" {
		if _, err := time.LoadLocation(timezone); err != nil {
			return domain.Balance{}, fmt.Errorf("%w: invalid Credit time zone", domain.ErrInvalid)
		}
	}
	return service.repository.Balance(ctx, userID, timezone, service.now().UTC())
}

func (service *Service) RequirePositiveBalance(ctx context.Context, userID, timezone string) error {
	balance, err := service.Balance(ctx, userID, timezone)
	if err != nil {
		return err
	}
	if balance.Total <= 0 {
		return domain.ErrInsufficientCredits
	}
	return nil
}

func (service *Service) Ledger(ctx context.Context, userID, cursor string, limit int) (domain.LedgerPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return service.repository.Ledger(ctx, userID, cursor, limit)
}

func (service *Service) ConfigureDailyAllocation(ctx context.Context, userID, timezone string, amount domain.Amount) (domain.Balance, error) {
	if amount < 0 {
		return domain.Balance{}, fmt.Errorf("%w: Daily Credit Allocation cannot be negative", domain.ErrInvalid)
	}
	return service.repository.ConfigureDailyAllocation(ctx, userID, amount, timezone, service.now().UTC())
}

func (service *Service) Adjust(ctx context.Context, userID, timezone string, amount domain.Amount, reason string) (domain.Balance, error) {
	reason = strings.TrimSpace(reason)
	if amount == 0 || reason == "" {
		return domain.Balance{}, fmt.Errorf("%w: non-zero Credit Adjustment and reason are required", domain.ErrInvalid)
	}
	return service.repository.Adjust(ctx, userID, amount, reason, timezone, service.now().UTC())
}

func (service *Service) CreateRedemptionBatch(ctx context.Context, administratorID string, count int, value domain.Amount, expiresAt *time.Time) (domain.RedemptionBatch, error) {
	if count < 1 || count > 100 || value <= 0 {
		return domain.RedemptionBatch{}, fmt.Errorf("%w: batch count must be 1-100 and value must be positive", domain.ErrInvalid)
	}
	secrets := make([]RedemptionSecret, 0, count)
	for range count {
		random := make([]byte, 24)
		if _, err := rand.Read(random); err != nil {
			return domain.RedemptionBatch{}, fmt.Errorf("generate Redemption Code: %w", err)
		}
		plaintext := "AWC-" + base64.RawURLEncoding.EncodeToString(random)
		verifier := sha256.Sum256([]byte(plaintext))
		secrets = append(secrets, RedemptionSecret{Identifier: base64.RawURLEncoding.EncodeToString(verifier[:9]), Verifier: verifier, Plaintext: plaintext})
	}
	return service.repository.CreateRedemptionBatch(ctx, administratorID, value, expiresAt, secrets, service.now().UTC())
}

func (service *Service) Redeem(ctx context.Context, userID, timezone, plaintext string) (domain.Balance, error) {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" || len(plaintext) > 128 {
		return domain.Balance{}, domain.ErrCodeUnavailable
	}
	verifier := sha256.Sum256([]byte(plaintext))
	identifier := base64.RawURLEncoding.EncodeToString(verifier[:9])
	return service.repository.Redeem(ctx, userID, identifier, verifier, timezone, service.now().UTC())
}

func (service *Service) ListRates(ctx context.Context) ([]domain.RateRevision, error) {
	return service.repository.ListRates(ctx)
}

func (service *Service) CreateRateRevision(ctx context.Context, administratorID string, rate domain.ModelCreditRate, expectedRevision string) (domain.ModelCreditRate, error) {
	if strings.TrimSpace(administratorID) == "" || rate.InputMultiplierMicros < 0 || rate.OutputMultiplierMicros < 0 || rate.Fallback < 0 {
		return domain.ModelCreditRate{}, fmt.Errorf("%w: Model Credit Rate is invalid", domain.ErrInvalid)
	}
	if rate.Key != nil {
		if err := rate.Key.Validate(); err != nil {
			return domain.ModelCreditRate{}, err
		}
	}
	return service.repository.CreateRateRevision(ctx, administratorID, rate, expectedRevision, service.now().UTC())
}
