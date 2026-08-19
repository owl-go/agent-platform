package api

import (
	"encoding/json"
	"fmt"
	"time"

	agentdomain "agent-platform/backend/internal/biz/agentlifecycle/domain"
	approvaldomain "agent-platform/backend/internal/biz/approval/domain"
	artifactdomain "agent-platform/backend/internal/biz/artifact/domain"
	collaborationdomain "agent-platform/backend/internal/biz/collaboration/domain"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"
	sourcedomain "agent-platform/backend/internal/biz/sourcecontrol/domain"
	transaction "agent-platform/backend/internal/biz/transaction"
)

type runResponse struct {
	ID              string                `json:"id"`
	SessionID       string                `json:"session_id"`
	AgentReleaseID  string                `json:"agent_release_id"`
	RuntimeImageID  string                `json:"runtime_image_id"`
	RequestText     string                `json:"request_text"`
	State           executiondomain.State `json:"state"`
	ModelBinding    json.RawMessage       `json:"model_binding"`
	ModelBudget     json.RawMessage       `json:"model_budget"`
	ExecutionLimits json.RawMessage       `json:"execution_limits"`
	Usage           json.RawMessage       `json:"usage"`
	CostAmount      string                `json:"cost_amount"`
	TerminalError   json.RawMessage       `json:"terminal_error,omitempty"`
	AttemptCount    int                   `json:"attempt_count"`
	CreatedBy       string                `json:"created_by"`
	CreatedAt       time.Time             `json:"created_at"`
	StartedAt       *time.Time            `json:"started_at,omitempty"`
	EndedAt         *time.Time            `json:"ended_at,omitempty"`
	UpdatedAt       time.Time             `json:"updated_at"`
	Version         int64                 `json:"version"`
	Attempts        []attemptResponse     `json:"attempts"`
}
type attemptResponse struct {
	ID                    string                       `json:"id"`
	Number                int                          `json:"number"`
	WorkerID              string                       `json:"worker_id"`
	State                 executiondomain.AttemptState `json:"state"`
	InfrastructureFailure bool                         `json:"infrastructure_failure"`
	Error                 json.RawMessage              `json:"error,omitempty"`
	StartedAt             time.Time                    `json:"started_at"`
	EndedAt               *time.Time                   `json:"ended_at,omitempty"`
}

func newRunResponse(value executiondomain.Details) runResponse {
	attempts := make([]attemptResponse, 0, len(value.Attempts))
	for _, attempt := range value.Attempts {
		attempts = append(attempts, attemptResponse{ID: attempt.ID, Number: attempt.Number, WorkerID: attempt.WorkerID, State: attempt.State, InfrastructureFailure: attempt.InfrastructureFailure, Error: attempt.Error, StartedAt: attempt.StartedAt, EndedAt: attempt.EndedAt})
	}
	return runResponse{ID: value.ID, SessionID: value.SessionID, AgentReleaseID: value.AgentReleaseID, RuntimeImageID: value.RuntimeImageID, RequestText: value.RequestText, State: value.State, ModelBinding: value.ModelBinding, ModelBudget: value.ModelBudget, ExecutionLimits: value.ExecutionLimits, Usage: value.Usage, CostAmount: value.Cost, TerminalError: value.TerminalError, AttemptCount: value.AttemptCount, CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, StartedAt: value.StartedAt, EndedAt: value.EndedAt, UpdatedAt: value.UpdatedAt, Version: value.Version, Attempts: attempts}
}

type runApprovalResponse struct {
	ID             string               `json:"id"`
	RunID          string               `json:"run_id"`
	Kind           approvaldomain.Kind  `json:"kind"`
	Request        json.RawMessage      `json:"request"`
	State          approvaldomain.State `json:"state"`
	RequestedAt    time.Time            `json:"requested_at"`
	DecidedBy      string               `json:"decided_by,omitempty"`
	DecidedAt      *time.Time           `json:"decided_at,omitempty"`
	DecisionReason string               `json:"decision_reason"`
	Version        int64                `json:"version"`
}

func newRunApprovalResponse(value approvaldomain.Approval) runApprovalResponse {
	return runApprovalResponse{ID: value.ID, RunID: value.RunID, Kind: value.Kind, Request: value.Request, State: value.State, RequestedAt: value.RequestedAt, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, DecisionReason: value.DecisionReason, Version: value.Version}
}

type artifactResponse struct {
	ID          string            `json:"id"`
	RunID       string            `json:"run_id"`
	Kind        string            `json:"kind"`
	SizeBytes   int64             `json:"size_bytes"`
	SHA256      string            `json:"sha256"`
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

func newArtifactResponse(value artifactdomain.Artifact) artifactResponse {
	return artifactResponse{ID: value.ID, RunID: value.RunID, Kind: value.Kind, SizeBytes: value.SizeBytes, SHA256: value.SHA256, ContentType: value.ContentType, Metadata: value.Metadata, ExpiresAt: value.ExpiresAt, CreatedAt: value.CreatedAt}
}

type auditEventResponse struct {
	ID           int64           `json:"id"`
	TeamID       string          `json:"team_id"`
	ActorUserID  string          `json:"actor_user_id,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Details      json.RawMessage `json:"details"`
	CreatedAt    time.Time       `json:"created_at"`
}

type runtimeImageResponse struct {
	ID                        string                `json:"id"`
	Runtime                   runtimedomain.Runtime `json:"runtime"`
	CLIVersion                string                `json:"cli_version"`
	AdapterVersion            string                `json:"adapter_version"`
	ImageDigest               string                `json:"image_digest"`
	Capabilities              map[string]bool       `json:"capabilities"`
	Status                    runtimedomain.Status  `json:"status"`
	BlockedReason             string                `json:"blocked_reason,omitempty"`
	ConformanceEvidenceKey    string                `json:"conformance_evidence_key,omitempty"`
	ConformanceEvidenceSHA256 string                `json:"conformance_evidence_sha256,omitempty"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
	Version                   int64                 `json:"version"`
}

func newRuntimeImageResponse(value runtimedomain.RuntimeImage) runtimeImageResponse {
	return runtimeImageResponse{ID: value.ID, Runtime: value.Runtime, CLIVersion: value.CLIVersion, AdapterVersion: value.AdapterVersion, ImageDigest: value.ImageDigest, Capabilities: value.Capabilities, Status: value.Status, BlockedReason: value.BlockedReason, ConformanceEvidenceKey: value.ConformanceEvidenceKey, ConformanceEvidenceSHA256: value.ConformanceEvidenceSHA256, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

type credentialProfileResponse struct {
	ID               string                     `json:"id"`
	TeamID           *string                    `json:"team_id,omitempty"`
	Name             string                     `json:"name"`
	Kind             modeldomain.CredentialKind `json:"kind"`
	SecretConfigured bool                       `json:"secret_configured"`
	Enabled          bool                       `json:"enabled"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
	Version          int64                      `json:"version"`
}

func newCredentialProfileResponse(value modeldomain.CredentialProfile) credentialProfileResponse {
	return credentialProfileResponse{ID: value.ID, TeamID: value.TeamID, Name: value.Name, Kind: value.Kind, SecretConfigured: value.SecretRef != "", Enabled: value.Enabled(), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

type configuredModelResponse struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	ModelID             string    `json:"model_id"`
	Endpoint            string    `json:"endpoint"`
	CredentialProfileID string    `json:"credential_profile_id"`
	Enabled             bool      `json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	Version             int64     `json:"version"`
}

func newConfiguredModelResponse(value modeldomain.ConfiguredModel) configuredModelResponse {
	return configuredModelResponse{ID: value.ID, Name: value.Name, ModelID: value.ModelID, Endpoint: value.Endpoint, CredentialProfileID: value.CredentialProfileID, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

type sourceControlProviderResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      sourcedomain.Kind `json:"kind"`
	BaseURL   string            `json:"base_url"`
	Enabled   bool              `json:"enabled"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Version   int64             `json:"version"`
}

func newSourceControlProviderResponse(value sourcedomain.Provider) sourceControlProviderResponse {
	return sourceControlProviderResponse{ID: value.ID, Name: value.Name, Kind: value.Kind, BaseURL: value.BaseURL, Enabled: value.Enabled, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

type repositoryBindingResponse struct {
	ID                          string                         `json:"id"`
	TeamID                      string                         `json:"team_id"`
	SourceControlProviderID     string                         `json:"source_control_provider_id"`
	Name                        string                         `json:"name"`
	RepositorySSHURL            string                         `json:"repository_ssh_url"`
	DefaultBranch               string                         `json:"default_branch"`
	SSHCredentialProfileID      string                         `json:"ssh_credential_profile_id"`
	BuildCredentialProfileIDs   []string                       `json:"build_credential_profile_ids"`
	GitAuthorName               string                         `json:"git_author_name"`
	GitAuthorEmail              string                         `json:"git_author_email"`
	AllowedRuntimeImageIDs      []string                       `json:"allowed_runtime_image_ids"`
	DefaultRuntimeImageID       string                         `json:"default_runtime_image_id"`
	RequiredRuntimeCapabilities []string                       `json:"required_runtime_capabilities"`
	DefaultModelID              string                         `json:"default_model_id"`
	ModelBudget                 sourcedomain.ModelBudget       `json:"model_budget"`
	Instructions                string                         `json:"instructions"`
	QualityCommands             []sourcedomain.QualityCommand  `json:"quality_commands"`
	EgressPolicy                sourcedomain.EgressPolicy      `json:"egress_policy"`
	ValidationReport            *sourcedomain.ValidationReport `json:"validation_report"`
	ValidatedAt                 *time.Time                     `json:"validated_at"`
	CreatedAt                   time.Time                      `json:"created_at"`
	UpdatedAt                   time.Time                      `json:"updated_at"`
	Version                     int64                          `json:"version"`
}

func newRepositoryBindingResponse(value sourcedomain.RepositoryBinding) repositoryBindingResponse {
	return repositoryBindingResponse{ID: value.ID, TeamID: value.TeamID, SourceControlProviderID: value.SourceControlProviderID, Name: value.Name, RepositorySSHURL: value.RepositorySSHURL, DefaultBranch: value.DefaultBranch, SSHCredentialProfileID: value.SSHCredentialProfileID, BuildCredentialProfileIDs: append([]string(nil), value.BuildCredentialProfileIDs...), GitAuthorName: value.GitAuthorName, GitAuthorEmail: value.GitAuthorEmail, AllowedRuntimeImageIDs: append([]string(nil), value.AllowedRuntimeImageIDs...), DefaultRuntimeImageID: value.DefaultRuntimeImageID, RequiredRuntimeCapabilities: append([]string(nil), value.RequiredRuntimeCapabilities...), DefaultModelID: value.DefaultModelID, ModelBudget: value.ModelBudget, Instructions: value.Instructions, QualityCommands: append([]sourcedomain.QualityCommand(nil), value.QualityCommands...), EgressPolicy: value.EgressPolicy, ValidationReport: value.ValidationReport, ValidatedAt: value.ValidatedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}

type agentResponse struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"team_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int64     `json:"version"`
}
type draftResponse struct {
	ID               string                        `json:"id"`
	AgentID          string                        `json:"agent_id"`
	Revision         int64                         `json:"revision"`
	State            agentdomain.DraftState        `json:"state"`
	Configuration    agentdomain.Configuration     `json:"configuration"`
	ReleaseRisk      agentdomain.ReleaseRisk       `json:"release_risk"`
	ValidationReport *agentdomain.ValidationReport `json:"validation_report"`
	CreatedBy        string                        `json:"created_by"`
	CreatedAt        time.Time                     `json:"created_at"`
	UpdatedAt        time.Time                     `json:"updated_at"`
	Version          int64                         `json:"version"`
}
type releaseResponse struct {
	ID                  string                    `json:"id"`
	AgentID             string                    `json:"agent_id"`
	ReleaseNumber       int64                     `json:"release_number"`
	SourceDraftID       string                    `json:"source_draft_id"`
	RuntimeImageID      string                    `json:"runtime_image_id"`
	ConfiguredModelID   string                    `json:"configured_model_id"`
	RepositoryBindingID string                    `json:"repository_binding_id"`
	Configuration       agentdomain.Configuration `json:"configuration"`
	Status              agentdomain.ReleaseStatus `json:"status"`
	ReleasedBy          string                    `json:"released_by"`
	ReleasedAt          time.Time                 `json:"released_at"`
	DeprecatedAt        *time.Time                `json:"deprecated_at"`
	Version             int64                     `json:"version"`
}
type approvalResponse struct {
	ID           string                    `json:"id"`
	DraftID      string                    `json:"draft_id"`
	DraftVersion int64                     `json:"draft_version"`
	RequestedBy  string                    `json:"requested_by"`
	State        agentdomain.ApprovalState `json:"state"`
	RequestedAt  time.Time                 `json:"requested_at"`
	DecidedBy    string                    `json:"decided_by,omitempty"`
	DecidedAt    *time.Time                `json:"decided_at"`
	Reason       string                    `json:"reason"`
	Version      int64                     `json:"version"`
}

func newAgentResponse(value agentdomain.Agent) agentResponse {
	return agentResponse{ID: value.ID, TeamID: value.TeamID, Name: value.Name, Description: value.Description, CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}
func newDraftResponse(value agentdomain.Draft) draftResponse {
	return draftResponse{ID: value.ID, AgentID: value.AgentID, Revision: value.Revision, State: value.State, Configuration: value.Configuration, ReleaseRisk: value.ReleaseRisk, ValidationReport: value.ValidationReport, CreatedBy: value.CreatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}
func newReleaseResponse(value agentdomain.Release) releaseResponse {
	return releaseResponse{ID: value.ID, AgentID: value.AgentID, ReleaseNumber: value.ReleaseNumber, SourceDraftID: value.SourceDraftID, RuntimeImageID: value.RuntimeImageID, ConfiguredModelID: value.ConfiguredModelID, RepositoryBindingID: value.RepositoryBindingID, Configuration: value.Configuration, Status: value.Status, ReleasedBy: value.ReleasedBy, ReleasedAt: value.ReleasedAt, DeprecatedAt: value.DeprecatedAt, Version: value.Version}
}
func newApprovalResponse(value agentdomain.ReleaseApproval) approvalResponse {
	return approvalResponse{ID: value.ID, DraftID: value.DraftID, DraftVersion: value.DraftVersion, RequestedBy: value.RequestedBy, State: value.State, RequestedAt: value.RequestedAt, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, Reason: value.Reason, Version: value.Version}
}

type taskResponse struct {
	ID             string                             `json:"id"`
	TeamID         string                             `json:"team_id"`
	AgentReleaseID string                             `json:"agent_release_id"`
	CreatedBy      string                             `json:"created_by"`
	Title          string                             `json:"title"`
	RequestText    string                             `json:"request_text"`
	IssueSnapshot  *collaborationdomain.IssueSnapshot `json:"issue_snapshot"`
	State          collaborationdomain.TaskState      `json:"state"`
	CreatedAt      time.Time                          `json:"created_at"`
	UpdatedAt      time.Time                          `json:"updated_at"`
	CompletedAt    *time.Time                         `json:"completed_at"`
	Version        int64                              `json:"version"`
}
type sessionResponse struct {
	ID                  string                            `json:"id"`
	CodingTaskID        string                            `json:"coding_task_id"`
	RepositoryBindingID string                            `json:"repository_binding_id"`
	TargetBranch        string                            `json:"target_branch"`
	ReviewBranch        string                            `json:"review_branch"`
	Memory              collaborationdomain.SessionMemory `json:"memory"`
	RunCount            int                               `json:"run_count"`
	CreatedAt           time.Time                         `json:"created_at"`
	UpdatedAt           time.Time                         `json:"updated_at"`
	Version             int64                             `json:"version"`
}
type messageResponse struct {
	ID           int64                             `json:"id"`
	RunID        string                            `json:"run_id,omitempty"`
	Author       collaborationdomain.MessageAuthor `json:"author"`
	AuthorUserID string                            `json:"author_user_id,omitempty"`
	Content      json.RawMessage                   `json:"content"`
	CreatedAt    time.Time                         `json:"created_at"`
}
type memoryCandidateResponse struct {
	ID                string                                   `json:"id"`
	AgentID           string                                   `json:"agent_id"`
	CodingTaskID      string                                   `json:"coding_task_id"`
	ProposedContent   string                                   `json:"proposed_content"`
	State             collaborationdomain.MemoryCandidateState `json:"state"`
	ProposedAt        time.Time                                `json:"proposed_at"`
	DecidedBy         string                                   `json:"decided_by,omitempty"`
	DecidedAt         *time.Time                               `json:"decided_at"`
	ResultingMemoryID string                                   `json:"resulting_memory_id,omitempty"`
}
type agentMemoryResponse struct {
	ID           string     `json:"id"`
	AgentID      string     `json:"agent_id"`
	Content      string     `json:"content"`
	ApprovedBy   string     `json:"approved_by"`
	SourceTaskID string     `json:"source_task_id,omitempty"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at"`
	Version      int64      `json:"version"`
}

func newTaskResponse(value collaborationdomain.Task) taskResponse {
	return taskResponse{ID: value.ID, TeamID: value.TeamID, AgentReleaseID: value.AgentReleaseID, CreatedBy: value.CreatedBy, Title: value.Title, RequestText: value.RequestText, IssueSnapshot: value.IssueSnapshot, State: value.State, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, CompletedAt: value.CompletedAt, Version: value.Version}
}
func newSessionResponse(value collaborationdomain.Session) sessionResponse {
	return sessionResponse{ID: value.ID, CodingTaskID: value.CodingTaskID, RepositoryBindingID: value.RepositoryBindingID, TargetBranch: value.TargetBranch, ReviewBranch: value.ReviewBranch, Memory: value.Memory, RunCount: value.RunCount, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Version: value.Version}
}
func newMemoryCandidateResponse(value collaborationdomain.MemoryCandidate) memoryCandidateResponse {
	return memoryCandidateResponse{ID: value.ID, AgentID: value.AgentID, CodingTaskID: value.CodingTaskID, ProposedContent: value.ProposedContent, State: value.State, ProposedAt: value.ProposedAt, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, ResultingMemoryID: value.ResultingMemoryID}
}
func newAgentMemoryResponse(value collaborationdomain.AgentMemory) agentMemoryResponse {
	return agentMemoryResponse{ID: value.ID, AgentID: value.AgentID, Content: value.Content, Enabled: value.Enabled, ApprovedBy: value.ApprovedBy, SourceTaskID: value.SourceTaskID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, DeletedAt: value.DeletedAt, Version: value.Version}
}

func encodeWriteResult(status int, value any, err error) (transaction.IdempotencyResult, error) {
	if err != nil {
		return transaction.IdempotencyResult{}, err
	}
	body, err := json.Marshal(value)
	if err != nil {
		return transaction.IdempotencyResult{}, fmt.Errorf("encode response: %w", err)
	}
	return transaction.IdempotencyResult{Status: status, Body: body}, nil
}
