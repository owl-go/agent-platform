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

func TestParseRejectsMalformedPlatformEvent(t *testing.T) {
	if _, recognized, err := Parse([]byte(Prefix + `{"kind":"unknown","payload":{}}`)); !recognized || err == nil {
		t.Fatalf("malformed event recognized=%v error=%v", recognized, err)
	}
	if _, recognized, err := Parse([]byte(`{"ordinary":"runtime output"}`)); recognized || err != nil {
		t.Fatalf("ordinary output recognized=%v error=%v", recognized, err)
	}
}
