package cliconnector

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ActionReason string

const (
	ReasonAuthorizationRequired ActionReason = "authorization_required"
	ReasonSetupRequired         ActionReason = "setup_required"
	ReasonPermissionsMissing    ActionReason = "permissions_missing"
	ReasonExpired               ActionReason = "expired"
	ReasonRefreshFailed         ActionReason = "refresh_failed"
	ReasonProviderDenied        ActionReason = "provider_denied"
	ReasonProviderUnavailable   ActionReason = "provider_unavailable"
	ReasonSchemeUnavailable     ActionReason = "scheme_unavailable"
	ReasonAccountSelection      ActionReason = "account_selection_required"
	ReasonCancelledByUser       ActionReason = "cancelled_by_user"
	ReasonUserActionExpired     ActionReason = "user_action_expired"
)

type UserAction string

const (
	UserActionOpenURL     UserAction = "open_url"
	UserActionShowCode    UserAction = "show_code"
	UserActionCopyValue   UserAction = "copy_value"
	UserActionCheckStatus UserAction = "check_status"
	UserActionEditSetup   UserAction = "edit_setup"
)

type ActionState string

const (
	ActionPending   ActionState = "pending"
	ActionResolved  ActionState = "resolved"
	ActionCancelled ActionState = "cancelled"
	ActionExpired   ActionState = "expired"
)

// ActionRequirement contains only reviewed UI semantics. Provider continuation
// data and credentials remain encrypted in server-owned records.
type ActionRequirement struct {
	ContractVersion     int               `json:"contract_version"`
	ID                  string            `json:"id"`
	OwnerID             string            `json:"-"`
	ExecutionKind       string            `json:"execution_kind"`
	ExecutionID         string            `json:"execution_id"`
	StageID             string            `json:"stage_id"`
	OperationID         string            `json:"operation_id"`
	ConnectorID         string            `json:"connector_id"`
	ConnectorName       string            `json:"connector_name"`
	AuthorizationScheme string            `json:"-"`
	Identity            Identity          `json:"identity"`
	EnablementID        string            `json:"enablement_id"`
	CapabilityID        string            `json:"capability_id"`
	OperationPhrase     map[string]string `json:"operation_phrase,omitempty"`
	Reason              ActionReason      `json:"reason"`
	Permissions         []string          `json:"permissions,omitempty"`
	Actions             []UserAction      `json:"actions"`
	State               ActionState       `json:"state"`
	ActionURL           string            `json:"action_url,omitempty"`
	ActionURLToken      string            `json:"-"`
	ActionURLOpenedAt   *time.Time        `json:"-"`
	ExpiresAt           time.Time         `json:"expires_at"`
	Version             int64             `json:"version"`
}

type ActionRequest struct {
	OwnerID, ExecutionKind, ExecutionID, StageID string
	OperationID, ConnectorID, ConnectorName      string
	AuthorizationScheme                          string
	Identity                                     Identity
	EnablementID, CapabilityID                   string
	OperationPhrase                              map[string]string
	Reason                                       ActionReason
	Permissions                                  []string
	Actions                                      []UserAction
	ExpiresAt                                    time.Time
}

type ActionCoordinator interface {
	AwaitAction(context.Context, ActionRequest) error
}

var (
	ErrActionRejected = errors.New("Connector action rejected")
	ErrActionExpired  = errors.New("Connector action expired")
)

type RequirementError struct {
	Reason       ActionReason
	EnablementID string
	Permissions  []string
	Actions      []UserAction
}

func (err *RequirementError) Error() string {
	return fmt.Sprintf("Connector action required: %s", err.Reason)
}

func NewRequirementError(reason ActionReason, enablementID string, permissions []string, actions ...UserAction) error {
	return &RequirementError{Reason: reason, EnablementID: enablementID, Permissions: append([]string(nil), permissions...), Actions: append([]UserAction(nil), actions...)}
}

func ValidateActionRequest(request ActionRequest) error {
	if request.Identity != IdentityUser && request.Identity != IdentityBot {
		return errors.New("unsupported Connector action identity")
	}
	if request.Reason != ReasonAuthorizationRequired && request.Reason != ReasonSetupRequired && request.Reason != ReasonPermissionsMissing &&
		request.Reason != ReasonExpired && request.Reason != ReasonRefreshFailed && request.Reason != ReasonProviderDenied &&
		request.Reason != ReasonProviderUnavailable && request.Reason != ReasonSchemeUnavailable && request.Reason != ReasonAccountSelection &&
		request.Reason != ReasonCancelledByUser && request.Reason != ReasonUserActionExpired {
		return errors.New("unsupported Connector action reason")
	}
	if len(request.Actions) == 0 {
		return errors.New("Connector action requirement has no permitted actions")
	}
	for _, action := range request.Actions {
		if action != UserActionOpenURL && action != UserActionCopyValue && action != UserActionCheckStatus {
			return errors.New("unsupported Connector user action")
		}
	}
	return nil
}
