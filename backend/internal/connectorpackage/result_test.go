package connectorpackage_test

import (
	"testing"

	"agent-platform/backend/internal/connectorpackage"
)

func TestNormalizeInvocationFailureRedactsExactSecrets(t *testing.T) {
	result := connectorpackage.NormalizeInvocationFailure("req-1", connectorpackage.ErrorUnauthorized, "token=secret-value", true, "reauthorize", []string{"secret-value"})
	if result.OK || result.RequestID != "req-1" || result.Error == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Error.Message != "token=[REDACTED]" {
		t.Fatalf("message = %q", result.Error.Message)
	}
}

func TestNormalizeInvocationSuccessCopiesStructuredData(t *testing.T) {
	data := map[string]any{"value": "ok"}
	result := connectorpackage.NormalizeInvocationSuccess("req-2", data, []string{"slow"})
	if !result.OK || result.Error != nil || result.RequestID != "req-2" || result.Data["value"] != "ok" || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestNormalizeInvocationSuccessRedactsNestedValues(t *testing.T) {
	result := connectorpackage.NormalizeInvocationSuccessRedacted("req-3", map[string]any{"nested": map[string]any{"token": "secret-value"}}, []string{"secret-value"}, []string{"secret-value"})
	nested := result.Data["nested"].(map[string]any)
	if nested["token"] != "[REDACTED]" {
		t.Fatalf("nested value = %#v", nested["token"])
	}
}
