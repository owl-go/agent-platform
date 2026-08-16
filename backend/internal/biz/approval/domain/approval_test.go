package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRunApprovalRequiresExplicitValidDecision(t *testing.T) {
	now := time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC)
	approval, err := Request("approval", "run", KindPlan, json.RawMessage(`{"summary":"change schema"}`), now)
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
		if _, err := Request("approval", "run", input.kind, input.request, now); err == nil {
			t.Fatalf("Request(%q, %s) succeeded", input.kind, input.request)
		}
	}
}
