package tokenverifier

import (
	"context"

	identitydomain "agent-platform/backend/internal/biz/identity/domain"
)

// RejectingVerifier is the explicit fail-closed verifier used until a concrete
// deployment configures its enterprise OIDC adapter. It is a real dependency,
// not a nil/optional authentication bypass.
type RejectingVerifier struct{}

func NewRejecting() *RejectingVerifier { return &RejectingVerifier{} }

func (*RejectingVerifier) Verify(context.Context, string) (identitydomain.VerifiedIdentity, error) {
	return identitydomain.VerifiedIdentity{}, identitydomain.ErrUnauthenticated
}
