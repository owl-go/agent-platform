package agentruntime

import (
	"context"
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrorInvalidConfiguration ErrorCode = "invalid_configuration"
	ErrorRuntimeUnavailable   ErrorCode = "runtime_unavailable"
	ErrorAuthenticationFailed ErrorCode = "authentication_failed"
	ErrorModelFailed          ErrorCode = "model_failed"
	ErrorCommandFailed        ErrorCode = "command_failed"
	ErrorBudgetExhausted      ErrorCode = "budget_exhausted"
	ErrorInterrupted          ErrorCode = "interrupted"
	ErrorTimedOut             ErrorCode = "timed_out"
	ErrorEventDeliveryFailed  ErrorCode = "event_delivery_failed"
	ErrorInternalAdapter      ErrorCode = "internal_adapter_error"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func ErrorCodeOf(err error) ErrorCode {
	var runtimeErr *Error
	switch {
	case errors.As(err, &runtimeErr):
		return runtimeErr.Code
	case errors.Is(err, context.DeadlineExceeded):
		return ErrorTimedOut
	case errors.Is(err, context.Canceled):
		return ErrorInterrupted
	default:
		return ErrorInternalAdapter
	}
}
