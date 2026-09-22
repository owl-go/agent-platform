package connectorpackage

import "strings"

type InvocationErrorType string

const (
	ErrorNotInstalled      InvocationErrorType = "not_installed"
	ErrorUnauthorized      InvocationErrorType = "unauthorized"
	ErrorPermissionDenied  InvocationErrorType = "permission_denied"
	ErrorInvalidArgument   InvocationErrorType = "invalid_argument"
	ErrorNotFound          InvocationErrorType = "not_found"
	ErrorTimeout           InvocationErrorType = "timeout"
	ErrorRateLimited       InvocationErrorType = "rate_limited"
	ErrorUpstream          InvocationErrorType = "upstream_error"
	ErrorConnectorInternal InvocationErrorType = "connector_internal"
)

type InvocationError struct {
	Type       InvocationErrorType `json:"type"`
	Message    string              `json:"message"`
	Retryable  bool                `json:"retryable"`
	NextAction string              `json:"next_action,omitempty"`
}

type InvocationResult struct {
	OK        bool             `json:"ok"`
	Data      map[string]any   `json:"data,omitempty"`
	Error     *InvocationError `json:"error,omitempty"`
	RequestID string           `json:"request_id"`
	Warnings  []string         `json:"warnings,omitempty"`
}

func RedactValue(value any, secrets []string) any {
	switch item := value.(type) {
	case string:
		for _, secret := range secrets {
			if secret != "" {
				item = strings.ReplaceAll(item, secret, "[REDACTED]")
			}
		}
		return item
	case map[string]any:
		copyValue := make(map[string]any, len(item))
		for key, nested := range item {
			copyValue[key] = RedactValue(nested, secrets)
		}
		return copyValue
	case []any:
		copyValue := make([]any, len(item))
		for index, nested := range item {
			copyValue[index] = RedactValue(nested, secrets)
		}
		return copyValue
	default:
		return value
	}
}

func NormalizeInvocationSuccess(requestID string, data map[string]any, warnings []string) InvocationResult {
	copyData, _ := RedactValue(data, nil).(map[string]any)
	cleanWarnings := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		cleanWarnings = append(cleanWarnings, RedactValue(warning, nil).(string))
	}
	return InvocationResult{OK: true, Data: copyData, RequestID: requestID, Warnings: cleanWarnings}
}

func NormalizeInvocationSuccessRedacted(requestID string, data map[string]any, warnings, secrets []string) InvocationResult {
	redacted, _ := RedactValue(data, secrets).(map[string]any)
	cleanWarnings := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		cleanWarnings = append(cleanWarnings, RedactValue(warning, secrets).(string))
	}
	return InvocationResult{OK: true, Data: redacted, RequestID: requestID, Warnings: cleanWarnings}
}

func NormalizeInvocationFailure(requestID string, errorType InvocationErrorType, message string, retryable bool, nextAction string, secrets []string) InvocationResult {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
		}
	}
	return InvocationResult{OK: false, RequestID: requestID, Error: &InvocationError{Type: errorType, Message: message, Retryable: retryable, NextAction: nextAction}}
}
