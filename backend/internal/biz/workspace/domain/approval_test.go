package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCommandApprovalIsOwnerBoundOneUseAndDigestBound(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	approval, err := NewCommandApproval("approval-1", "owner-1", "stage-1", "digest-1", "nonce-1", now, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := approval.Decide("other-owner", ApprovalApproved, IdentityUser, now); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner decision error = %v", err)
	}
	if err := approval.Decide("owner-1", ApprovalApproved, IdentityUser, now); err != nil {
		t.Fatal(err)
	}
	if err := approval.Consume("owner-1", "digest-changed", "nonce-1", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed digest error = %v", err)
	}
	if err := approval.Consume("owner-1", "digest-1", "nonce-1", now); err != nil {
		t.Fatal(err)
	}
	if err := approval.Consume("owner-1", "digest-1", "nonce-1", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestCommandApprovalRejectsDeadlinesOverFifteenMinutes(t *testing.T) {
	_, err := NewCommandApproval("approval-1", "owner-1", "stage-1", "digest-1", "nonce-1", time.Now(), 16*time.Minute)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}
