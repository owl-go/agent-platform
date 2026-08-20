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
	ID           string    `json:"id"`
	DraftID      string    `json:"draft_id"`
	DraftVersion int64     `json:"draft_version"`
	RequestedBy  string    `json:"requested_by"`
	RiskReason   string    `json:"risk_reason"`
	ApprovedBy   string    `json:"approved_by"`
	ApprovedAt   time.Time `json:"approved_at"`
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
	RiskReason   string
	State        ApprovalState
	RequestedAt  time.Time
	DecidedBy    string
	DecidedAt    *time.Time
	Reason       string
	Version      int64
}

func RequestReleaseApproval(id, draftID string, draftVersion int64, requestedBy, riskReason string, now time.Time) (ReleaseApproval, error) {
	riskReason = strings.TrimSpace(riskReason)
	if id == "" || draftID == "" || draftVersion <= 0 || requestedBy == "" || riskReason == "" || len(riskReason) > 2000 || now.IsZero() {
		return ReleaseApproval{}, invalidAgentf("Release Approval identity, Draft, requester, risk reason, and time are required")
	}
	return ReleaseApproval{ID: id, DraftID: draftID, DraftVersion: draftVersion, RequestedBy: requestedBy, RiskReason: riskReason, State: ApprovalPending, RequestedAt: now.UTC(), Version: 1}, nil
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
	return &RiskApproval{ID: approval.ID, DraftID: approval.DraftID, DraftVersion: approval.DraftVersion, RequestedBy: approval.RequestedBy, RiskReason: approval.RiskReason, ApprovedBy: approval.DecidedBy, ApprovedAt: *approval.DecidedAt}
}

type ReleaseQualityCommand struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Executable     string   `json:"executable"`
	Arguments      []string `json:"arguments"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type RepositoryBindingSnapshot struct {
	ID                          string                  `json:"id"`
	Name                        string                  `json:"name"`
	RepositorySSHURL            string                  `json:"repository_ssh_url"`
	DefaultBranch               string                  `json:"default_branch"`
	Instructions                string                  `json:"instructions"`
	QualityCommands             []ReleaseQualityCommand `json:"quality_commands"`
	EgressPolicy                string                  `json:"egress_policy"`
	RequiredRuntimeCapabilities []string                `json:"required_runtime_capabilities"`
}

type RuntimeImageSnapshot struct {
	ID             string          `json:"id"`
	Runtime        string          `json:"runtime"`
	CLIVersion     string          `json:"cli_version"`
	AdapterVersion string          `json:"adapter_version"`
	ImageDigest    string          `json:"image_digest"`
	Capabilities   map[string]bool `json:"capabilities"`
}

type ConfiguredModelSnapshot struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ModelID  string `json:"model_id"`
	Endpoint string `json:"endpoint"`
}

type ReleaseDependencies struct {
	RepositoryBinding RepositoryBindingSnapshot `json:"repository_binding"`
	RuntimeImage      RuntimeImageSnapshot      `json:"runtime_image"`
	ConfiguredModel   ConfiguredModelSnapshot   `json:"configured_model"`
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
	ReleaseRisk         ReleaseRisk
	Dependencies        ReleaseDependencies
	ApprovalEvidence    *RiskApproval
	Status              ReleaseStatus
	BlockedReason       string
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
	Dependencies  ReleaseDependencies
	Now           time.Time
}

func Publish(input ReleaseRegistration) (Release, error) {
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.ReleasedBy) == "" || input.ReleaseNumber <= 0 || input.Now.IsZero() {
		return Release{}, invalidAgentf("Release identity, number, publisher, and time are required")
	}
	if err := input.Draft.CanRelease(); err != nil {
		return Release{}, err
	}
	if err := input.Dependencies.validate(input.Draft.Configuration); err != nil {
		return Release{}, err
	}
	now := input.Now.UTC()
	if input.Draft.ReleaseRisk == ReleaseRiskHigh {
		approval := input.Approval
		if approval == nil || strings.TrimSpace(approval.ID) == "" || approval.DraftID != input.Draft.ID || approval.DraftVersion != input.Draft.Version || strings.TrimSpace(approval.RequestedBy) == "" || strings.TrimSpace(approval.RiskReason) == "" || strings.TrimSpace(approval.ApprovedBy) == "" || approval.ApprovedBy == approval.RequestedBy || approval.ApprovedAt.IsZero() || approval.ApprovedAt.After(now) {
			return Release{}, ErrApprovalRequired
		}
	}
	configuration := input.Draft.Configuration
	dependencies := cloneReleaseDependencies(input.Dependencies)
	return Release{
		ID: input.ID, AgentID: input.Draft.AgentID, ReleaseNumber: input.ReleaseNumber,
		SourceDraftID: input.Draft.ID, RuntimeImageID: configuration.RuntimeImageID,
		ConfiguredModelID: configuration.ConfiguredModelID, RepositoryBindingID: configuration.RepositoryBindingID,
		Configuration: configuration, ReleaseRisk: input.Draft.ReleaseRisk, Dependencies: dependencies,
		ApprovalEvidence: cloneRiskApproval(input.Approval), Status: ReleaseStatusReleased, ReleasedBy: input.ReleasedBy, ReleasedAt: now, Version: 1,
	}, nil
}

func RestoreRelease(release Release) (Release, error) {
	if release.ID == "" || release.AgentID == "" || release.SourceDraftID == "" || release.ReleaseNumber <= 0 || release.ReleasedBy == "" || release.ReleasedAt.IsZero() || release.Version <= 0 {
		return Release{}, invalidAgentf("persisted Agent Release is invalid")
	}
	if err := release.Configuration.Validate(); err != nil {
		return Release{}, err
	}
	if release.ReleaseRisk != ReleaseRiskLow && release.ReleaseRisk != ReleaseRiskHigh {
		return Release{}, invalidAgentf("persisted Agent Release risk is invalid")
	}
	if err := release.Dependencies.validate(release.Configuration); err != nil {
		return Release{}, err
	}
	if release.ReleaseRisk == ReleaseRiskHigh && !validApprovalEvidence(release.ApprovalEvidence, release.SourceDraftID, release.ReleasedAt) {
		return Release{}, invalidAgentf("persisted high-risk Agent Release Approval evidence is invalid")
	}
	if release.ReleaseRisk == ReleaseRiskLow && release.ApprovalEvidence != nil {
		return Release{}, invalidAgentf("persisted low-risk Agent Release has Approval evidence")
	}
	if release.RuntimeImageID != release.Configuration.RuntimeImageID || release.ConfiguredModelID != release.Configuration.ConfiguredModelID || release.RepositoryBindingID != release.Configuration.RepositoryBindingID {
		return Release{}, invalidAgentf("persisted Agent Release snapshot references are inconsistent")
	}
	switch release.Status {
	case ReleaseStatusReleased:
		if release.BlockedReason != "" || release.DeprecatedAt != nil {
			return Release{}, invalidAgentf("released Agent Release has terminal metadata")
		}
	case ReleaseStatusBlocked:
		if strings.TrimSpace(release.BlockedReason) == "" || release.DeprecatedAt != nil {
			return Release{}, invalidAgentf("blocked Agent Release metadata is invalid")
		}
	case ReleaseStatusDeprecated:
		if release.BlockedReason != "" || release.DeprecatedAt == nil || release.DeprecatedAt.Before(release.ReleasedAt) {
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

func (release *Release) Block(reason string) error {
	reason = strings.TrimSpace(reason)
	if release.Status != ReleaseStatusReleased || reason == "" || len(reason) > 2000 {
		return invalidAgentf("only a released Agent Release can be blocked with a reason")
	}
	release.Status = ReleaseStatusBlocked
	release.BlockedReason = reason
	release.Version++
	return nil
}

func (dependencies ReleaseDependencies) validate(configuration Configuration) error {
	binding, runtime, model := dependencies.RepositoryBinding, dependencies.RuntimeImage, dependencies.ConfiguredModel
	if binding.ID != configuration.RepositoryBindingID || runtime.ID != configuration.RuntimeImageID || model.ID != configuration.ConfiguredModelID ||
		strings.TrimSpace(binding.Name) == "" || strings.TrimSpace(binding.RepositorySSHURL) == "" || strings.TrimSpace(binding.DefaultBranch) == "" || binding.EgressPolicy != "public" ||
		strings.TrimSpace(runtime.Runtime) == "" || strings.TrimSpace(runtime.CLIVersion) == "" || strings.TrimSpace(runtime.AdapterVersion) == "" || !validRepoDigest(runtime.ImageDigest) || runtime.Capabilities == nil ||
		strings.TrimSpace(model.Name) == "" || strings.TrimSpace(model.ModelID) == "" || strings.TrimSpace(model.Endpoint) == "" {
		return invalidAgentf("Agent Release dependency snapshots are invalid")
	}
	return nil
}

func validApprovalEvidence(approval *RiskApproval, draftID string, releasedAt time.Time) bool {
	return approval != nil && strings.TrimSpace(approval.ID) != "" && approval.DraftID == draftID && approval.DraftVersion > 0 &&
		strings.TrimSpace(approval.RequestedBy) != "" && strings.TrimSpace(approval.RiskReason) != "" && strings.TrimSpace(approval.ApprovedBy) != "" &&
		approval.ApprovedBy != approval.RequestedBy && !approval.ApprovedAt.IsZero() && !approval.ApprovedAt.After(releasedAt)
}

func validRepoDigest(value string) bool {
	_, digest, found := strings.Cut(value, "@sha256:")
	if !found || len(digest) != 64 {
		return false
	}
	for _, character := range digest {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func cloneReleaseDependencies(input ReleaseDependencies) ReleaseDependencies {
	result := input
	result.RepositoryBinding.RequiredRuntimeCapabilities = append([]string(nil), input.RepositoryBinding.RequiredRuntimeCapabilities...)
	result.RepositoryBinding.QualityCommands = make([]ReleaseQualityCommand, len(input.RepositoryBinding.QualityCommands))
	for index, command := range input.RepositoryBinding.QualityCommands {
		command.Arguments = append([]string(nil), command.Arguments...)
		result.RepositoryBinding.QualityCommands[index] = command
	}
	result.RuntimeImage.Capabilities = make(map[string]bool, len(input.RuntimeImage.Capabilities))
	for capability, enabled := range input.RuntimeImage.Capabilities {
		result.RuntimeImage.Capabilities[capability] = enabled
	}
	return result
}

func cloneRiskApproval(input *RiskApproval) *RiskApproval {
	if input == nil {
		return nil
	}
	result := *input
	return &result
}
