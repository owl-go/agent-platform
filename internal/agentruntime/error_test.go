package agentruntime_test

import (
	"errors"
	"testing"

	"agent-platform/internal/agentruntime"
)

func TestRuntimeErrorPreservesCodeAndCause(t *testing.T) {
	cause := errors.New("provider rejected credentials")
	err := &agentruntime.Error{
		Code:    agentruntime.ErrorAuthenticationFailed,
		Message: "runtime authentication failed",
		Cause:   cause,
	}

	if got := agentruntime.ErrorCodeOf(err); got != agentruntime.ErrorAuthenticationFailed {
		t.Fatalf("error code: got %q", got)
	}
	if !errors.Is(err, cause) {
		t.Fatal("runtime error did not preserve its cause")
	}
}

func TestErrorCodeOfUnknownError(t *testing.T) {
	if got := agentruntime.ErrorCodeOf(errors.New("boom")); got != agentruntime.ErrorInternalAdapter {
		t.Fatalf("unknown error code: got %q", got)
	}
}
