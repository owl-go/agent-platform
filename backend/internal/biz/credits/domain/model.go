package domain

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

var (
	ErrInvalid             = errors.New("Credits value is invalid")
	ErrInsufficientCredits = errors.New("Credit Balance is not positive")
	ErrCodeUnavailable     = errors.New("Redemption Code is unavailable")
	ErrConflict            = errors.New("Credits state conflicts with current state")
)

type InsufficientCreditsError struct {
	Balance          Amount
	NextAllocationAt time.Time
}

func (err *InsufficientCreditsError) Error() string { return ErrInsufficientCredits.Error() }
func (err *InsufficientCreditsError) Unwrap() error { return ErrInsufficientCredits }

// Amount stores hundredths of a Credit.
type Amount int64

const (
	DefaultDailyAllocation Amount = 60_000
	DefaultFallback        Amount = 1_000
	MultiplierScale               = int64(1_000_000)
)

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Known        bool
}

type ModelRateKey struct {
	ProviderType string
	Protocol     string
	ModelID      string
}

func (key ModelRateKey) Validate() error {
	if strings.TrimSpace(key.ProviderType) == "" || strings.TrimSpace(key.Protocol) == "" || strings.TrimSpace(key.ModelID) == "" {
		return fmt.Errorf("%w: Provider type, API Protocol, and Provider Model identifier are required", ErrInvalid)
	}
	return nil
}

type ModelCreditRate struct {
	RevisionID             string
	Key                    *ModelRateKey
	InputMultiplierMicros  int64
	OutputMultiplierMicros int64
	Fallback               Amount
	CreatedAt              time.Time
}

func (rate ModelCreditRate) Validate() error {
	if strings.TrimSpace(rate.RevisionID) == "" || rate.InputMultiplierMicros < 0 || rate.OutputMultiplierMicros < 0 || rate.Fallback < 0 {
		return fmt.Errorf("%w: Model Credit Rate is invalid", ErrInvalid)
	}
	return nil
}

type Admission struct {
	UserID        string
	ExecutionID   string
	StagePosition int
	Source        string
	Timezone      string
	CreditDay     string
	StartedAt     time.Time
	Rate          ModelCreditRate
}

type Settlement struct {
	Admission Admission
	Source    string
	Amount    Amount
	Estimated bool
	Usage     Usage
	SettledAt time.Time
}

type Consumption struct {
	Amount    Amount
	Estimated bool
	Usage     Usage
	Rate      ModelCreditRate
}

type Balance struct {
	UserID                 string
	CreditDay              string
	Timezone               string
	DailyAllocation        Amount
	DailyRemaining         Amount
	Persistent             Amount
	TodayConsumed          Amount
	Total                  Amount
	PendingDailyAllocation *Amount
	PendingEffectiveDay    string
	NextAllocationAt       time.Time
	Version                int64
}

type LedgerEntry struct {
	ID               string
	Type             string
	Amount           Amount
	ResultingBalance Amount
	CreditDay        string
	Reason           string
	CreatedAt        time.Time
}

type LedgerPage struct {
	Items      []LedgerEntry
	NextCursor string
}

type RateRevision struct {
	Rate         ModelCreditRate
	SupersededAt *time.Time
}

type RedemptionBatch struct {
	ID        string
	Count     int
	Value     Amount
	ExpiresAt *time.Time
	CreatedAt time.Time
	Codes     []RedemptionCode
}

type RedemptionCode struct {
	ID         string
	Identifier string
	Plaintext  string
	State      string
	RedeemedAt *time.Time
}

type RedemptionCodeStatus struct {
	ID, BatchID, Identifier, State  string
	Value                           Amount
	ExpiresAt, RedeemedAt, VoidedAt *time.Time
	CreatedAt                       time.Time
}

type RedemptionCodePage struct {
	Items      []RedemptionCodeStatus
	NextCursor string
}

func CalculateConsumption(usage Usage, rate ModelCreditRate) (Consumption, error) {
	if err := rate.Validate(); err != nil {
		return Consumption{}, err
	}
	if usage.InputTokens < 0 || usage.OutputTokens < 0 {
		return Consumption{}, fmt.Errorf("%w: Token Usage cannot be negative", ErrInvalid)
	}
	if !usage.Known {
		return Consumption{Amount: rate.Fallback, Estimated: true, Rate: rate}, nil
	}

	input := new(big.Int).Mul(big.NewInt(usage.InputTokens), big.NewInt(rate.InputMultiplierMicros))
	output := new(big.Int).Mul(big.NewInt(usage.OutputTokens), big.NewInt(rate.OutputMultiplierMicros))
	numerator := new(big.Int).Add(input, output)
	if numerator.Sign() == 0 {
		return Consumption{Usage: usage, Rate: rate}, nil
	}

	// Multipliers use six decimals; dividing by 100,000,000 yields hundredths
	// of a Credit because one Credit represents 10,000 Tokens.
	denominator := big.NewInt(100_000_000)
	numerator.Add(numerator, new(big.Int).Quo(denominator, big.NewInt(2)))
	hundredths := new(big.Int).Quo(numerator, denominator)
	if hundredths.Sign() == 0 {
		hundredths.SetInt64(1)
	}
	if !hundredths.IsInt64() {
		return Consumption{}, fmt.Errorf("%w: Credit Consumption overflows", ErrInvalid)
	}
	return Consumption{Amount: Amount(hundredths.Int64()), Usage: usage, Rate: rate}, nil
}
