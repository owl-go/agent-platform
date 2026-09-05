package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalRejected ApprovalState = "rejected"
	ApprovalConsumed ApprovalState = "consumed"
	ApprovalExpired  ApprovalState = "expired"
	ApprovalClosed   ApprovalState = "closed"
)

type ExecutionIdentity string

const (
	IdentityUser ExecutionIdentity = "user"
	IdentityBot  ExecutionIdentity = "bot"
)

type CommandApproval struct {
	ID, OwnerID, StageID, CommandDigest, NonceHash      string
	ExecutionKind, ExecutionID                          string
	ConnectorName, Operation, Target, RedactedArguments string
	State                                               ApprovalState
	Identity                                            ExecutionIdentity
	ExpiresAt                                           time.Time
	DecidedAt, ConsumedAt                               *time.Time
	Version                                             int64
}

func NewCommandApproval(id, ownerID, stageID, digest, nonce string, now time.Time, timeout time.Duration) (CommandApproval, error) {
	decodedDigest, digestErr := hex.DecodeString(digest)
	if id == "" || ownerID == "" || stageID == "" || digestErr != nil || len(decodedDigest) != sha256.Size || nonce == "" || timeout <= 0 || timeout > 15*time.Minute {
		return CommandApproval{}, fmt.Errorf("%w: invalid command approval", ErrInvalid)
	}
	return CommandApproval{ID: id, OwnerID: ownerID, StageID: stageID, CommandDigest: digest, NonceHash: ApprovalNonceHash(nonce), State: ApprovalPending, ExpiresAt: now.Add(timeout)}, nil
}

func (approval *CommandApproval) Decide(ownerID string, decision ApprovalState, identity ExecutionIdentity, now time.Time) error {
	if ownerID != approval.OwnerID {
		return ErrNotFound
	}
	if approval.State != ApprovalPending {
		return fmt.Errorf("%w: approval is no longer pending", ErrConflict)
	}
	if !now.Before(approval.ExpiresAt) {
		approval.State = ApprovalExpired
		return fmt.Errorf("%w: approval expired", ErrConflict)
	}
	if decision != ApprovalApproved && decision != ApprovalRejected {
		return fmt.Errorf("%w: invalid approval decision", ErrInvalid)
	}
	if decision == ApprovalApproved && identity != IdentityUser && identity != IdentityBot {
		return fmt.Errorf("%w: execution identity is required", ErrInvalid)
	}
	if decision == ApprovalApproved && approval.Identity != "" && identity != approval.Identity {
		return fmt.Errorf("%w: execution identity does not match the requested command", ErrConflict)
	}
	approval.State, approval.Identity = decision, identity
	decided := now
	approval.DecidedAt = &decided
	return nil
}

func (approval *CommandApproval) Consume(ownerID, digest, nonce string, now time.Time) error {
	if ownerID != approval.OwnerID {
		return ErrNotFound
	}
	if approval.State != ApprovalApproved || digest != approval.CommandDigest || ApprovalNonceHash(nonce) != approval.NonceHash || !now.Before(approval.ExpiresAt) {
		return fmt.Errorf("%w: approval cannot authorize this command", ErrConflict)
	}
	approval.State = ApprovalConsumed
	consumed := now
	approval.ConsumedAt = &consumed
	return nil
}

func ApprovalNonceHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
