package tokenverifier

import (
	"context"

	accountdomain "agent-platform/backend/internal/biz/account/domain"
)

type Rejecting struct{}

func (Rejecting) Verify(context.Context, string) (accountdomain.VerifiedIdentity, error) {
	return accountdomain.VerifiedIdentity{}, accountdomain.ErrUnauthenticated
}
