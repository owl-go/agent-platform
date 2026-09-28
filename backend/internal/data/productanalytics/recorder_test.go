package productanalytics

import (
	"bytes"
	"testing"
	"time"
)

func TestNewEventHashesIdentifiersAndKeepsOnlyAllowlistedAttributes(t *testing.T) {
	row, err := newEvent("first_task_started", "user-raw", "session-raw", "session", map[string]any{"entry": "session", "has_file": true}, "user", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if row.AnonymousUserKey == "user-raw" || row.AnonymousSubjectKey == nil || *row.AnonymousSubjectKey == "session-raw" {
		t.Fatalf("identifiers were not anonymized: %#v", row)
	}
	if bytes.Contains(row.Attributes, []byte("user-raw")) || bytes.Contains(row.Attributes, []byte("session-raw")) {
		t.Fatalf("attributes leaked identifiers: %s", row.Attributes)
	}
	if string(row.Attributes) != `{"entry":"session","has_file":true}` {
		t.Fatalf("attributes = %s", row.Attributes)
	}
}

func TestNewEventRejectsContentAndUnknownFields(t *testing.T) {
	for name, attributes := range map[string]map[string]any{
		"prompt":          {"prompt": "summarize payroll.xlsx"},
		"filename":        {"entry": "payroll.xlsx", "has_file": true},
		"externalAccount": {"identity_source": "alice@example.com", "first_login": true},
	} {
		eventName := "first_task_started"
		if name == "externalAccount" {
			eventName = "login_completed"
		}
		if _, err := newEvent(eventName, "user", "session", "session", attributes, "user", time.Now()); err == nil {
			t.Fatalf("%s attributes were accepted", name)
		}
	}
}

func TestNewEventRejectsMissingRequiredAttributes(t *testing.T) {
	if _, err := newEvent("workflow_created", "user", "workflow", "workflow", map[string]any{"source": "manual"}, "subject", time.Now()); err == nil {
		t.Fatal("event without required boolean attributes was accepted")
	}
}

func TestDurationBucketUsesBoundedCategories(t *testing.T) {
	tests := map[time.Duration]string{
		5 * time.Second:  "under_10s",
		20 * time.Second: "10s_30s",
		time.Minute:      "30s_2m",
		5 * time.Minute:  "2m_10m",
		20 * time.Minute: "over_10m",
	}
	for duration, want := range tests {
		if got := durationBucket(duration); got != want {
			t.Errorf("durationBucket(%s) = %q, want %q", duration, got, want)
		}
	}
}
