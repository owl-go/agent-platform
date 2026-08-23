package platformprotocol

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestApprovalRequestRoundTrip(t *testing.T) {
	line, err := EncodeApprovalRequest("Review Branch delivery", "agent-platform/task")
	if err != nil {
		t.Fatal(err)
	}
	event, recognized, err := Parse(line)
	if err != nil || !recognized || event.Kind != "approval.requested" {
		t.Fatalf("Parse() = (%+v, %v, %v)", event, recognized, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload["kind"] != "high_risk_change" {
		t.Fatalf("payload = %s, error = %v", event.Payload, err)
	}
	if !bytes.Contains(event.Payload, []byte("agent-platform/task")) {
		t.Fatalf("payload = %s", event.Payload)
	}
}

func TestWorkflowDeliveryRoundTrip(t *testing.T) {
	line, err := EncodeWorkflowDelivered("agent-platform/task", "0123456789abcdef0123456789abcdef01234567", []string{"backend/main.go"})
	if err != nil {
		t.Fatal(err)
	}
	event, recognized, err := Parse(line)
	if err != nil || !recognized || event.Kind != "workflow.delivered" {
		t.Fatalf("Parse() = (%+v, %v, %v)", event, recognized, err)
	}
	if !bytes.Contains(event.Payload, []byte("0123456789abcdef0123456789abcdef01234567")) {
		t.Fatalf("payload = %s", event.Payload)
	}
}

func TestParseRejectsMalformedPlatformEvent(t *testing.T) {
	if _, recognized, err := Parse([]byte(Prefix + `{"kind":"unknown","payload":{}}`)); !recognized || err == nil {
		t.Fatalf("malformed event recognized=%v error=%v", recognized, err)
	}
	if _, recognized, err := Parse([]byte(`{"ordinary":"runtime output"}`)); recognized || err != nil {
		t.Fatalf("ordinary output recognized=%v error=%v", recognized, err)
	}
	for _, line := range []string{
		Prefix + `{"kind":"workflow.delivered","payload":{"review_branch":"review","commit":"short","changed_files":[]}}`,
		Prefix + `{"kind":"workflow.delivered","payload":{"review_branch":"../main","commit":"0123456789abcdef0123456789abcdef01234567","changed_files":[]}}`,
		Prefix + `{"kind":"workflow.delivered","payload":{"review_branch":"review","commit":"0123456789abcdef0123456789abcdef01234567","changed_files":["../secret"]}}`,
	} {
		if _, recognized, err := Parse([]byte(line)); !recognized || err == nil {
			t.Fatalf("unsafe delivery recognized=%v error=%v line=%s", recognized, err, line)
		}
	}
}
