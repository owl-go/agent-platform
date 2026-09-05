package gormrepo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"

	"gorm.io/gorm"
)

type Repository struct {
	db      *gorm.DB
	credits creditTransactionSettler
}

type creditTransactionSettler interface {
	SettleTx(*gorm.DB, creditsdomain.Settlement) (creditsdomain.Consumption, error)
}

func New(db *gorm.DB, credits creditTransactionSettler) *Repository {
	return &Repository{db: db, credits: credits}
}

var _ application.Repository = (*Repository)(nil)

type sessionRecord struct {
	ID               string     `gorm:"column:id"`
	OwnerID          string     `gorm:"column:owner_user_id"`
	Title            string     `gorm:"column:title"`
	ExpertID         *string    `gorm:"column:expert_id"`
	ExpertTeamID     *string    `gorm:"column:expert_team_id"`
	ExpertSnapshot   []byte     `gorm:"column:expert_snapshot;type:jsonb"`
	ArchivedAt       *time.Time `gorm:"column:archived_at"`
	RuntimeEngine    *string    `gorm:"column:runtime_engine"`
	NativeCheckpoint string     `gorm:"column:native_checkpoint"`
	RollingSummary   string     `gorm:"column:rolling_summary"`
	SummaryThrough   *int64     `gorm:"column:summary_through_message_id"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	Version          int64      `gorm:"column:version"`
}

func (sessionRecord) TableName() string { return "sessions" }

type messageRecord struct {
	ID                int64      `gorm:"column:id"`
	SessionID         string     `gorm:"column:session_id"`
	Role              string     `gorm:"column:role"`
	State             string     `gorm:"column:state"`
	Content           string     `gorm:"column:content"`
	Error             *string    `gorm:"column:error"`
	ProgressStage     string     `gorm:"column:progress_stage"`
	ElapsedMS         int64      `gorm:"column:elapsed_ms"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	Completed         *time.Time `gorm:"column:completed_at"`
	CancelRequested   *time.Time `gorm:"column:cancel_requested_at"`
	ResponseSnapshot  []byte     `gorm:"column:response_snapshot;type:jsonb"`
	Attachments       []byte     `gorm:"column:attachments;type:jsonb"`
	ExpertStages      []byte     `gorm:"column:expert_stages;type:jsonb"`
	CreditConsumption []byte     `gorm:"column:credit_consumption;type:jsonb"`
}

func (messageRecord) TableName() string { return "session_messages" }

type workflowRecord struct {
	ID                string     `gorm:"column:id"`
	OwnerID           string     `gorm:"column:owner_user_id"`
	Name              string     `gorm:"column:name"`
	Goal              string     `gorm:"column:goal"`
	ExpertID          *string    `gorm:"column:expert_id"`
	ExpertTeamID      *string    `gorm:"column:expert_team_id"`
	ProviderModelID   *string    `gorm:"column:provider_model_id"`
	RuntimeEngine     *string    `gorm:"column:runtime_engine"`
	Environment       []byte     `gorm:"column:environment;type:jsonb"`
	EnvironmentSecret []byte     `gorm:"column:environment_secret_ciphertext"`
	Schedule          []byte     `gorm:"column:schedule;type:jsonb"`
	NextScheduledAt   *time.Time `gorm:"column:next_scheduled_at"`
	GitSource         []byte     `gorm:"column:git_source;type:jsonb"`
	GitSecret         []byte     `gorm:"column:git_secret_ciphertext"`
	APIKey            *string    `gorm:"column:api_key"`
	APISecretHash     *string    `gorm:"column:api_secret_hash"`
	WorkspacePath     string     `gorm:"column:workspace_path"`
	DeletedAt         *time.Time `gorm:"column:deleted_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
	Version           int64      `gorm:"column:version"`
}

func (workflowRecord) TableName() string { return "workflows" }

type expertRecord struct {
	ID                     string    `gorm:"column:id"`
	OwnerID                string    `gorm:"column:owner_user_id"`
	Name                   string    `gorm:"column:name"`
	CapabilityIntroduction string    `gorm:"column:capability_introduction"`
	ExecutionInstruction   string    `gorm:"column:execution_instruction"`
	ProviderModelID        *string   `gorm:"column:provider_model_id"`
	RuntimeEngine          *string   `gorm:"column:runtime_engine"`
	ExpertiseTags          []byte    `gorm:"column:expertise_tags;type:jsonb"`
	MCPServerIDs           []byte    `gorm:"column:mcp_server_ids;type:jsonb"`
	SkillIDs               []byte    `gorm:"column:skill_ids;type:jsonb"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
	Version                int64     `gorm:"column:version"`
}

func (expertRecord) TableName() string { return "experts" }

type expertTeamRecord struct {
	ID                     string    `gorm:"column:id"`
	OwnerID                string    `gorm:"column:owner_user_id"`
	Name                   string    `gorm:"column:name"`
	CapabilityIntroduction string    `gorm:"column:capability_introduction"`
	ExpertiseTags          []byte    `gorm:"column:expertise_tags;type:jsonb"`
	ExpertIDs              []byte    `gorm:"column:expert_ids;type:jsonb"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
	Version                int64     `gorm:"column:version"`
}

func (expertTeamRecord) TableName() string { return "expert_teams" }

type settingsRecord struct {
	UserID                  string `gorm:"column:user_id"`
	Personality             string `gorm:"column:personality"`
	PersonalityInstructions string `gorm:"column:personality_instructions"`
	RuntimeModelDefaults    []byte `gorm:"column:runtime_model_defaults;type:jsonb"`
	DefaultRuntimeEngine    string `gorm:"column:default_runtime_engine"`
	Language                string `gorm:"column:language"`
	Timezone                string `gorm:"column:timezone"`
	Version                 int64  `gorm:"column:version"`
}

func (settingsRecord) TableName() string { return "personal_settings" }

type modelProviderConnectionRecord struct {
	ID                 string     `gorm:"column:id"`
	CredentialOwnerID  string     `gorm:"column:credential_owner_user_id"`
	Name               string     `gorm:"column:name"`
	ProviderType       string     `gorm:"column:provider_type"`
	Endpoint           string     `gorm:"column:endpoint"`
	Protocols          []byte     `gorm:"column:protocols;type:jsonb"`
	APIKeyCiphertext   []byte     `gorm:"column:api_key_ciphertext"`
	VerificationStatus string     `gorm:"column:verification_status"`
	VerificationError  *string    `gorm:"column:verification_error"`
	CustomEndpoint     bool       `gorm:"column:custom_endpoint"`
	LastSyncedAt       *time.Time `gorm:"column:last_synced_at"`
	LastSyncError      *string    `gorm:"column:last_sync_error"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	Version            int64      `gorm:"column:version"`
}

func (modelProviderConnectionRecord) TableName() string { return "model_provider_connections" }

type providerModelRecord struct {
	ID            string    `gorm:"column:id"`
	ConnectionID  string    `gorm:"column:connection_id"`
	ModelID       string    `gorm:"column:model_id"`
	DisplayName   string    `gorm:"column:display_name"`
	Available     bool      `gorm:"column:available"`
	ManuallyAdded bool      `gorm:"column:manually_added"`
	Compatibility []byte    `gorm:"column:compatibility;type:jsonb"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (providerModelRecord) TableName() string { return "provider_models" }

type modelProviderCredentialVersionRecord struct {
	ConnectionID      string    `gorm:"column:connection_id"`
	ConnectionVersion int64     `gorm:"column:connection_version"`
	APIKeyCiphertext  []byte    `gorm:"column:api_key_ciphertext"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (modelProviderCredentialVersionRecord) TableName() string {
	return "model_provider_credential_versions"
}

type mcpRecord struct {
	ID               string     `gorm:"column:id"`
	OwnerID          string     `gorm:"column:owner_user_id"`
	Name             string     `gorm:"column:name"`
	Transport        string     `gorm:"column:transport"`
	Configuration    []byte     `gorm:"column:configuration;type:jsonb"`
	SecretCiphertext []byte     `gorm:"column:secret_ciphertext"`
	TestRequestedAt  *time.Time `gorm:"column:test_requested_at"`
	TestedAt         *time.Time `gorm:"column:tested_at"`
	TestError        *string    `gorm:"column:test_error"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	Version          int64      `gorm:"column:version"`
}

func (mcpRecord) TableName() string { return "mcp_servers" }

type skillRecord struct {
	ID        string    `gorm:"column:id"`
	OwnerID   string    `gorm:"column:owner_user_id"`
	Name      string    `gorm:"column:name"`
	Source    string    `gorm:"column:source"`
	GitURL    *string   `gorm:"column:git_url"`
	GitRef    *string   `gorm:"column:git_ref"`
	ObjectKey string    `gorm:"column:object_key"`
	SHA256    string    `gorm:"column:sha256"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	Version   int64     `gorm:"column:version"`
}

func (skillRecord) TableName() string { return "skills" }

type runRecord struct {
	ID                string     `gorm:"column:id"`
	ConversationID    string     `gorm:"column:conversation_id"`
	TurnNumber        int        `gorm:"column:turn_number"`
	OwnerID           string     `gorm:"column:owner_user_id"`
	WorkflowID        *string    `gorm:"column:workflow_id"`
	WorkflowName      string     `gorm:"column:workflow_name"`
	Trigger           string     `gorm:"column:trigger"`
	State             string     `gorm:"column:state"`
	Input             []byte     `gorm:"column:input;type:jsonb"`
	WorkflowSnapshot  []byte     `gorm:"column:workflow_snapshot;type:jsonb"`
	FinalResult       []byte     `gorm:"column:final_result;type:jsonb"`
	TerminalError     *string    `gorm:"column:terminal_error"`
	QueuedAt          time.Time  `gorm:"column:queued_at"`
	StartedAt         *time.Time `gorm:"column:started_at"`
	EndedAt           *time.Time `gorm:"column:ended_at"`
	CancelRequested   *time.Time `gorm:"column:cancel_requested_at"`
	ExpertStages      []byte     `gorm:"column:expert_stages;type:jsonb"`
	CreditConsumption []byte     `gorm:"column:credit_consumption;type:jsonb"`
	NativeCheckpoint  string     `gorm:"column:native_checkpoint"`
	Version           int64      `gorm:"column:version"`
}

func (runRecord) TableName() string { return "runs" }

type artifactRecord struct {
	ID         string     `gorm:"column:id"`
	OwnerID    string     `gorm:"column:owner_user_id"`
	WorkflowID *string    `gorm:"column:workflow_id"`
	RunID      string     `gorm:"column:run_id"`
	Kind       string     `gorm:"column:kind"`
	Name       string     `gorm:"column:name"`
	Path       string     `gorm:"column:path"`
	ObjectKey  *string    `gorm:"column:object_key"`
	TextResult []byte     `gorm:"column:text_result;type:jsonb"`
	Size       int64      `gorm:"column:size_bytes"`
	SHA256     *string    `gorm:"column:sha256"`
	CreatedAt  time.Time  `gorm:"column:created_at"`
	ExpiresAt  *time.Time `gorm:"column:expires_at"`
}

func (artifactRecord) TableName() string { return "artifacts" }

func marshal(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode persistence value: %w", err)
	}
	return data, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}
