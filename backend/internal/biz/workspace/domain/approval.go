package domain

import (
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
	ID, OwnerID, StageID, CommandDigest, Nonce          string
	ExecutionKind, ExecutionID                          string
	ConnectorName, Operation, Target, RedactedArguments string
	State                                               ApprovalState
	Identity                                            ExecutionIdentity
	ExpiresAt                                           time.Time
	DecidedAt, ConsumedAt                               *time.Time
	Version                                             int64
}

func NewCommandApproval(id, ownerID, stageID, digest, nonce string, now time.Time, timeout time.Duration) (CommandApproval, error) {
	if id == "" || ownerID == "" || stageID == "" || digest == "" || nonce == "" || timeout <= 0 || timeout > 15*time.Minute {
		return CommandApproval{}, fmt.Errorf("%w: invalid command approval", ErrInvalid)
	}
	return CommandApproval{ID: id, OwnerID: ownerID, StageID: stageID, CommandDigest: digest, Nonce: nonce, State: ApprovalPending, ExpiresAt: now.Add(timeout)}, nil
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
	approval.State, approval.Identity = decision, identity
	decided := now
	approval.DecidedAt = &decided
	return nil
}

func (approval *CommandApproval) Consume(ownerID, digest, nonce string, now time.Time) error {
	if ownerID != approval.OwnerID {
		return ErrNotFound
	}
	if approval.State != ApprovalApproved || digest != approval.CommandDigest || nonce != approval.Nonce || !now.Before(approval.ExpiresAt) {
		return fmt.Errorf("%w: approval cannot authorize this command", ErrConflict)
	}
	approval.State = ApprovalConsumed
	consumed := now
	approval.ConsumedAt = &consumed
	return nil
}
