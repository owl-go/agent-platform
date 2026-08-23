package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/authz"
)

var (
	ErrNotFound         = errors.New("Run Approval not found")
	ErrPendingExists    = errors.New("Run already has a pending Approval")
	ErrConcurrentUpdate = errors.New("Run Approval was modified concurrently")
	ErrRunState         = errors.New("Run state does not allow the Approval operation")
)

type Kind string

const (
	KindPlan           Kind = "plan"
	KindHighRiskChange Kind = "high_risk_change"
)

type State string
type DecisionActorType string

const (
	StatePending  State = "pending"
	StateApproved State = "approved"
	StateRejected State = "rejected"

	DecisionActorUser   DecisionActorType = "user"
	DecisionActorSystem DecisionActorType = "system"
)

type Approval struct {
	ID                string
	RunID             string
	Kind              Kind
	Request           json.RawMessage
	State             State
	RequestedAt       time.Time
	RequestedBy       string
	DecidedBy         string
	DecisionActorType DecisionActorType
	DecidedAt         *time.Time
	DecisionReason    string
	Version           int64
}

func Request(id, runID string, kind Kind, request json.RawMessage, requestedBy string, now time.Time) (Approval, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(requestedBy) == "" || now.IsZero() {
		return Approval{}, fmt.Errorf("Run Approval identity, Run, requester, and request time are required")
	}
	if kind != KindPlan && kind != KindHighRiskChange {
		return Approval{}, fmt.Errorf("unsupported Run Approval kind %q", kind)
	}
	if len(request) == 0 || len(request) > 64*1024 || !json.Valid(request) || request[0] != '{' {
		return Approval{}, fmt.Errorf("Run Approval request must be a JSON object no larger than 64 KiB")
	}
	return Approval{ID: id, RunID: runID, Kind: kind, Request: cloneJSON(request), State: StatePending, RequestedAt: now.UTC(), RequestedBy: requestedBy, Version: 1}, nil
}

func Restore(value Approval) (Approval, error) {
	if value.Version <= 0 || value.RequestedAt.IsZero() || strings.TrimSpace(value.RequestedBy) == "" || !json.Valid(value.Request) {
		return Approval{}, fmt.Errorf("invalid persisted Run Approval")
	}
	switch value.State {
	case StatePending:
		if value.DecidedAt != nil || value.DecidedBy != "" || value.DecisionActorType != "" {
			return Approval{}, fmt.Errorf("pending Run Approval has a decision")
		}
	case StateApproved, StateRejected:
		validUser := value.DecisionActorType == DecisionActorUser && value.DecidedBy != ""
		validSystem := value.DecisionActorType == DecisionActorSystem && value.DecidedBy == ""
		if value.DecidedAt == nil || (!validUser && !validSystem) {
			return Approval{}, fmt.Errorf("decided Run Approval is missing its decision")
		}
	default:
		return Approval{}, fmt.Errorf("unknown Run Approval state %q", value.State)
	}
	value.Request = cloneJSON(value.Request)
	return value, nil
}

func (approval *Approval) Decide(approved bool, actor, reason string, now time.Time) error {
	if approval.State != StatePending || strings.TrimSpace(actor) == "" || now.IsZero() || now.Before(approval.RequestedAt) {
		return fmt.Errorf("Run Approval decision requires a pending Approval, actor, and valid time")
	}
	reason = strings.TrimSpace(reason)
	if !approved && reason == "" {
		return fmt.Errorf("Run Approval rejection reason is required")
	}
	if len(reason) > 4000 {
		return fmt.Errorf("Run Approval decision reason exceeds 4000 characters")
	}
	approval.State = StateRejected
	if approved {
		approval.State = StateApproved
	}
	decidedAt := now.UTC()
	approval.DecidedBy = actor
	approval.DecisionActorType = DecisionActorUser
	approval.DecidedAt = &decidedAt
	approval.DecisionReason = reason
	approval.Version++
	return nil
}

func (approval *Approval) RejectBySystem(reason string, now time.Time) error {
	if approval.State != StatePending || now.IsZero() || now.Before(approval.RequestedAt) {
		return fmt.Errorf("system rejection requires a pending Approval and valid time")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 4000 {
		return fmt.Errorf("system rejection reason is required and cannot exceed 4000 characters")
	}
	decidedAt := now.UTC()
	approval.State = StateRejected
	approval.DecidedBy = ""
	approval.DecisionActorType = DecisionActorSystem
	approval.DecidedAt = &decidedAt
	approval.DecisionReason = reason
	approval.Version++
	return nil
}

type Repository interface {
	Get(context.Context, string) (Approval, error)
	GetInScope(context.Context, string, authz.ReadScope) (Approval, error)
	ListByRun(context.Context, string) ([]Approval, error)
	Create(context.Context, Approval) error
	Decide(context.Context, Approval, int64) error
	PendingExists(context.Context, string) (bool, error)
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}
