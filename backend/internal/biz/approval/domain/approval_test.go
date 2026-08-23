package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRunApprovalRequiresExplicitValidDecision(t *testing.T) {
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	approval, err := Request("approval", "run", KindPlan, json.RawMessage(`{"summary":"change schema"}`), "requester", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := approval.Decide(false, "user", "", now.Add(time.Second)); err == nil {
		t.Fatal("rejection without a reason succeeded")
	}
	if err := approval.Decide(true, "user", "reviewed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if approval.State != StateApproved || approval.Version != 2 || approval.DecidedBy != "user" {
		t.Fatalf("Approval after decision = %+v", approval)
	}
	if err := approval.Decide(false, "other", "late", now.Add(2*time.Second)); err == nil {
		t.Fatal("second decision succeeded")
	}
}

func TestRunApprovalSystemRejectionHasNoUserIdentity(t *testing.T) {
	now := time.Now().UTC()
	approval, err := Request("approval", "run", KindPlan, json.RawMessage(`{"summary":"plan"}`), "requester", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := approval.RejectBySystem("wait expired", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if approval.State != StateRejected || approval.DecisionActorType != DecisionActorSystem || approval.DecidedBy != "" {
		t.Fatalf("system rejection = %+v", approval)
	}
	if _, err := Restore(approval); err != nil {
		t.Fatalf("restore system rejection: %v", err)
	}
}

func TestRunApprovalRejectsUnknownKindsAndNonObjectRequests(t *testing.T) {
	now := time.Now().UTC()
	for _, input := range []struct {
		kind    Kind
		request json.RawMessage
	}{
		{kind: "network", request: json.RawMessage(`{}`)},
		{kind: KindPlan, request: json.RawMessage(`[]`)},
		{kind: KindPlan, request: json.RawMessage(`not-json`)},
	} {
		if _, err := Request("approval", "run", input.kind, input.request, "requester", now); err == nil {
			t.Fatalf("Request(%q, %s) succeeded", input.kind, input.request)
		}
	}
}

func TestRestoreRunApprovalRequiresAuditableRequester(t *testing.T) {
	_, err := Restore(Approval{ID: "approval", RunID: "run", Kind: KindPlan, Request: json.RawMessage(`{}`), State: StatePending, RequestedAt: time.Now().UTC(), Version: 1})
	if err == nil {
		t.Fatal("Restore accepted a Run Approval without its requester")
	}
}
