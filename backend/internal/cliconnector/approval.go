package cliconnector

import (
	"context"
	"errors"
	"time"
)

var (
	ErrApprovalRejected = errors.New("CLI command approval rejected")
	ErrApprovalExpired  = errors.New("CLI command approval expired")
)

// ApprovalRequest contains the immutable, display-safe command identity that
// the Worker persists before allowing a high-risk process to start.
type ApprovalRequest struct {
	OwnerID, ExecutionKind, ExecutionID, StageID string
	ConnectorName, Operation, Target             string
	RedactedArguments, CommandDigest, Nonce      string
	Identity                                     Identity
	AllowedIdentities                            []Identity
	CommandDigests                               map[Identity]string
	ExpiresAt                                    time.Time
}

type ApprovalGrant struct {
	Nonce     string
	Identity  Identity
	ExpiresAt time.Time
}

type ApprovalCoordinator interface {
	Await(context.Context, ApprovalRequest) (ApprovalGrant, error)
	Consume(context.Context, string, string, string) error
	Close(context.Context, string, string) error
}
