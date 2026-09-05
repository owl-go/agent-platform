package workspace

import (
	"testing"
	"time"

	creditsdomain "agent-platform/backend/internal/biz/credits/domain"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

func TestInsufficientCreditsErrorIncludesOnlyRecoveryMetadata(t *testing.T) {
	next := time.Date(2026, 9, 5, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	err := kratoserrors.FromError(publicError(&creditsdomain.InsufficientCreditsError{Balance: -125, NextAllocationAt: next}))

	if err.Code != 429 || err.Reason != "insufficient_credits" {
		t.Fatalf("public error = %+v", err)
	}
	if err.Metadata["balance_hundredths"] != "-125" || err.Metadata["next_allocation_at"] != "2026-09-04T16:00:00Z" || len(err.Metadata) != 2 {
		t.Fatalf("public metadata = %#v", err.Metadata)
	}
}
