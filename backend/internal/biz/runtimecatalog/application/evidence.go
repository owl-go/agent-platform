package application

import (
	"context"
	"errors"

	"agent-platform/backend/internal/biz/runtimecatalog/domain"
)

var (
	ErrInvalidEvidence     = errors.New("invalid Conformance evidence")
	ErrEvidenceUnavailable = errors.New("Conformance evidence unavailable")
)

type VerifiedEvidence struct {
	Key    string
	SHA256 string
}

type EvidenceVerifier interface {
	Verify(context.Context, string, domain.RuntimeImage) (VerifiedEvidence, error)
}
