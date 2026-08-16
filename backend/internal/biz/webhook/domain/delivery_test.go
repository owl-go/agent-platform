package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeliveryValidationRequiresSafeTargetAndJSON(t *testing.T) {
	delivery := Delivery{
		ID: "delivery", OrganizationID: "organization", EventType: "run.completed",
		Payload: json.RawMessage(`{"run_id":"run"}`), TargetURL: "https://hooks.example.test/agent-platform",
	}
	if err := delivery.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"http://hooks.example.test", "https://user:secret@hooks.example.test", "file:///tmp/hook"} {
		delivery.TargetURL = target
		if err := delivery.Validate(); err == nil {
			t.Fatalf("Validate accepted unsafe target %q", target)
		}
	}
}

func TestRetryDelayIsExponentiallyBounded(t *testing.T) {
	base, maximum := time.Second, 5*time.Second
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for index, expected := range want {
		if got := RetryDelay(index+1, base, maximum); got != expected {
			t.Fatalf("attempt %d delay = %s, want %s", index+1, got, expected)
		}
	}
}
