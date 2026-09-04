package domain_test

import (
	"testing"

	"agent-platform/backend/internal/biz/credits/domain"
)

func TestCalculateConsumption(t *testing.T) {
	rate := domain.ModelCreditRate{
		RevisionID:             "default-v1",
		InputMultiplierMicros:  1_000_000,
		OutputMultiplierMicros: 1_000_000,
		Fallback:               1_000,
	}
	tests := []struct {
		name      string
		usage     domain.Usage
		want      domain.Amount
		estimated bool
	}{
		{name: "one credit", usage: domain.Usage{InputTokens: 4_000, OutputTokens: 6_000, Known: true}, want: 100},
		{name: "round half up", usage: domain.Usage{InputTokens: 150, Known: true}, want: 2},
		{name: "minimum nonzero", usage: domain.Usage{InputTokens: 1, Known: true}, want: 1},
		{name: "real zero", usage: domain.Usage{Known: true}, want: 0},
		{name: "missing usage", usage: domain.Usage{}, want: 1_000, estimated: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := domain.CalculateConsumption(test.usage, rate)
			if err != nil {
				t.Fatal(err)
			}
			if result.Amount != test.want || result.Estimated != test.estimated {
				t.Fatalf("consumption = (%d, %t), want (%d, %t)", result.Amount, result.Estimated, test.want, test.estimated)
			}
		})
	}
}

func TestCalculateConsumptionAppliesDistinctMultipliersBeforeOneRounding(t *testing.T) {
	result, err := domain.CalculateConsumption(domain.Usage{InputTokens: 3_333, OutputTokens: 3_333, Known: true}, domain.ModelCreditRate{
		RevisionID:             "rate-v2",
		InputMultiplierMicros:  500_000,
		OutputMultiplierMicros: 2_000_000,
		Fallback:               250,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Amount != 83 {
		t.Fatalf("consumption = %d hundredths, want 83", result.Amount)
	}
}
