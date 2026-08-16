package domain

import (
	"strings"
	"time"
)

type ReleaseStatus string

const (
	ReleaseStatusReleased   ReleaseStatus = "released"
	ReleaseStatusDeprecated ReleaseStatus = "deprecated"
	ReleaseStatusBlocked    ReleaseStatus = "blocked"
)

type RiskApproval struct {
	ID           string
	DraftID      string
	DraftVersion int64
	RequestedBy  string
	ApprovedBy   string
	ApprovedAt   time.Time
}

type ApprovalState string

const (
	ApprovalPending  ApprovalState = "pending"
	ApprovalApproved ApprovalState = "approved"
	ApprovalRejected ApprovalState = "rejected"
)

type ReleaseApproval struct {
	ID           string
	DraftID      string
	DraftVersion int64
	RequestedBy  string
	State        ApprovalState
	RequestedAt  time.Time
	DecidedBy    string
	DecidedAt    *time.Time
	Reason       string
	Version      int64
}

func RequestReleaseApproval(id, draftID string, draftVersion int64, requestedBy string, now time.Time) (ReleaseApproval, error) {
	if id == "" || draftID == "" || draftVersion <= 0 || requestedBy == "" || now.IsZero() {
		return ReleaseApproval{}, invalidAgentf("Release Approval identity, Draft, requester, and time are required")
	}
	return ReleaseApproval{ID: id, DraftID: draftID, DraftVersion: draftVersion, RequestedBy: requestedBy, State: ApprovalPending, RequestedAt: now.UTC(), Version: 1}, nil
}

func (approval *ReleaseApproval) Decide(approved bool, decidedBy, reason string, now time.Time) error {
	if approval.State != ApprovalPending || decidedBy == "" || decidedBy == approval.RequestedBy || now.IsZero() || now.Before(approval.RequestedAt) {
		return invalidAgentf("Release Approval decision requires a different approver and a pending Approval")
	}
	if !approved && strings.TrimSpace(reason) == "" {
		return invalidAgentf("Release Approval rejection reason is required")
	}
	approval.State = ApprovalRejected
	if approved {
		approval.State = ApprovalApproved
	}
	approval.DecidedBy = decidedBy
	decidedAt := now.UTC()
	approval.DecidedAt = &decidedAt
	approval.Reason = reason
	approval.Version++
	return nil
}

func (approval ReleaseApproval) ApprovedRiskApproval() *RiskApproval {
	if approval.State != ApprovalApproved || approval.DecidedAt == nil {
		return nil
	}
	return &RiskApproval{ID: approval.ID, DraftID: approval.DraftID, DraftVersion: approval.DraftVersion, RequestedBy: approval.RequestedBy, ApprovedBy: approval.DecidedBy, ApprovedAt: *approval.DecidedAt}
}

type Release struct {
	ID                  string
	AgentID             string
	ReleaseNumber       int64
	SourceDraftID       string
	RuntimeImageID      string
	ConfiguredModelID   string
	RepositoryBindingID string
	Configuration       Configuration
	Status              ReleaseStatus
	ReleasedBy          string
	ReleasedAt          time.Time
	DeprecatedAt        *time.Time
	Version             int64
}

type ReleaseRegistration struct {
	ID            string
	ReleaseNumber int64
	Draft         Draft
	ReleasedBy    string
	Approval      *RiskApproval
	Now           time.Time
}

func Publish(input ReleaseRegistration) (Release, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.ReleasedBy) == "" || input.ReleaseNumber <= 0 || input.Now.IsZero() {
		return Release{}, invalidAgentf("Release identity, number, publisher, and time are required")
	}
	if err := input.Draft.CanRelease(); err != nil {
		return Release{}, err
	}
	if input.Draft.ReleaseRisk == ReleaseRiskHigh {
		approval := input.Approval
		if approval == nil || approval.DraftID != input.Draft.ID || approval.DraftVersion != input.Draft.Version || approval.RequestedBy != input.ReleasedBy || approval.ApprovedBy == "" || approval.ApprovedBy == approval.RequestedBy || approval.ApprovedAt.IsZero() {
			return Release{}, ErrApprovalRequired
		}
	}
	now := input.Now.UTC()
	configuration := input.Draft.Configuration
	return Release{
		ID: input.ID, AgentID: input.Draft.AgentID, ReleaseNumber: input.ReleaseNumber,
		SourceDraftID: input.Draft.ID, RuntimeImageID: configuration.RuntimeImageID,
		ConfiguredModelID: configuration.ConfiguredModelID, RepositoryBindingID: configuration.RepositoryBindingID,
		Configuration: configuration, Status: ReleaseStatusReleased, ReleasedBy: input.ReleasedBy, ReleasedAt: now, Version: 1,
	}, nil
}

func RestoreRelease(release Release) (Release, error) {
	if release.ID == "" || release.AgentID == "" || release.SourceDraftID == "" || release.ReleaseNumber <= 0 || release.ReleasedBy == "" || release.ReleasedAt.IsZero() || release.Version <= 0 {
		return Release{}, invalidAgentf("persisted Agent Release is invalid")
	}
	if err := release.Configuration.Validate(); err != nil {
		return Release{}, err
	}
	if release.RuntimeImageID != release.Configuration.RuntimeImageID || release.ConfiguredModelID != release.Configuration.ConfiguredModelID || release.RepositoryBindingID != release.Configuration.RepositoryBindingID {
		return Release{}, invalidAgentf("persisted Agent Release snapshot references are inconsistent")
	}
	switch release.Status {
	case ReleaseStatusReleased, ReleaseStatusBlocked:
		if release.DeprecatedAt != nil {
			return Release{}, invalidAgentf("non-deprecated Agent Release has Deprecated At")
		}
	case ReleaseStatusDeprecated:
		if release.DeprecatedAt == nil || release.DeprecatedAt.Before(release.ReleasedAt) {
			return Release{}, invalidAgentf("deprecated Agent Release timestamp is invalid")
		}
	default:
		return Release{}, invalidAgentf("persisted Agent Release status is invalid")
	}
	return release, nil
}

func (release *Release) Deprecate(now time.Time) error {
	if release.Status != ReleaseStatusReleased || now.IsZero() || now.Before(release.ReleasedAt) {
		return invalidAgentf("only a released Agent Release can be deprecated")
	}
	release.Status = ReleaseStatusDeprecated
	deprecatedAt := now.UTC()
	release.DeprecatedAt = &deprecatedAt
	release.Version++
	return nil
}

func (release *Release) Block() {
	if release.Status == ReleaseStatusReleased {
		release.Status = ReleaseStatusBlocked
		release.Version++
	}
}
