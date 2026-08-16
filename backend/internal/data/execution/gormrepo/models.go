package gormrepo

import (
	"database/sql/driver"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type jsonValue []byte

func (value jsonValue) Value() (driver.Value, error) {
	if len(value) == 0 {
		return nil, nil
	}
	return string(value), nil
}

func (value *jsonValue) Scan(source any) error {
	switch typed := source.(type) {
	case nil:
		*value = nil
	case []byte:
		*value = append((*value)[:0], typed...)
	case string:
		*value = append((*value)[:0], typed...)
	default:
		return fmt.Errorf("scan JSON from %T", source)
	}
	return nil
}

func (jsonValue) GormDataType() string { return "json" }

func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type runRecord struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	SessionID          string     `gorm:"column:session_id"`
	AgentReleaseID     string     `gorm:"column:agent_release_id"`
	RuntimeImageID     string     `gorm:"column:runtime_image_id"`
	RequestText        string     `gorm:"column:request_text"`
	State              string     `gorm:"column:state"`
	ModelBinding       jsonValue  `gorm:"column:model_binding;type:jsonb"`
	CredentialBindings jsonValue  `gorm:"column:credential_bindings;type:jsonb"`
	ModelBudget        jsonValue  `gorm:"column:model_budget;type:jsonb"`
	ExecutionLimits    jsonValue  `gorm:"column:execution_limits;type:jsonb"`
	Usage              jsonValue  `gorm:"column:usage;type:jsonb"`
	CostAmount         string     `gorm:"column:cost_amount;type:numeric(20,8)"`
	TerminalError      jsonValue  `gorm:"column:terminal_error;type:jsonb"`
	AttemptCount       int        `gorm:"column:attempt_count"`
	CreatedBy          string     `gorm:"column:created_by"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	EndedAt            *time.Time `gorm:"column:ended_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	Version            int64      `gorm:"column:version"`
}

func (runRecord) TableName() string { return "runs" }

type attemptRecord struct {
	ID                    string     `gorm:"column:id;primaryKey"`
	RunID                 string     `gorm:"column:run_id"`
	AttemptNumber         int        `gorm:"column:attempt_number"`
	WorkerID              string     `gorm:"column:worker_id"`
	State                 string     `gorm:"column:state"`
	InfrastructureFailure bool       `gorm:"column:infrastructure_failure"`
	Error                 jsonValue  `gorm:"column:error;type:jsonb"`
	StartedAt             time.Time  `gorm:"column:started_at"`
	EndedAt               *time.Time `gorm:"column:ended_at"`
}

func (attemptRecord) TableName() string { return "run_attempts" }

type leaseRecord struct {
	RunID      string    `gorm:"column:run_id;primaryKey"`
	AttemptID  string    `gorm:"column:attempt_id"`
	WorkerID   string    `gorm:"column:worker_id"`
	LeaseToken string    `gorm:"column:lease_token"`
	ExpiresAt  time.Time `gorm:"column:expires_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (leaseRecord) TableName() string { return "run_leases" }

type workspaceLeaseRecord struct {
	SessionID  string    `gorm:"column:session_id;primaryKey"`
	RunID      string    `gorm:"column:run_id"`
	LeaseToken string    `gorm:"column:lease_token"`
	ExpiresAt  time.Time `gorm:"column:expires_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (workspaceLeaseRecord) TableName() string { return "workspace_write_leases" }

type eventRecord struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	RunID     string    `gorm:"column:run_id"`
	Sequence  int64     `gorm:"column:sequence"`
	EventType string    `gorm:"column:event_type"`
	Payload   jsonValue `gorm:"column:payload;type:jsonb"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (eventRecord) TableName() string { return "run_events" }

type runtimeImageRecord struct {
	ID             string    `gorm:"column:id;primaryKey"`
	Runtime        string    `gorm:"column:runtime"`
	CLIVersion     string    `gorm:"column:cli_version"`
	AdapterVersion string    `gorm:"column:adapter_version"`
	ImageDigest    string    `gorm:"column:image_digest"`
	Capabilities   jsonValue `gorm:"column:capabilities;type:jsonb"`
}

func (runtimeImageRecord) TableName() string { return "runtime_images" }

type sessionRecord struct {
	ID                  string `gorm:"column:id;primaryKey"`
	CodingTaskID        string `gorm:"column:coding_task_id"`
	RepositoryBindingID string `gorm:"column:repository_binding_id"`
	TargetBranch        string `gorm:"column:target_branch"`
	ReviewBranch        string `gorm:"column:review_branch"`
	WorkspaceVolume     string `gorm:"column:workspace_volume"`
}

func (sessionRecord) TableName() string { return "sessions" }

type repositoryBindingRecord struct {
	ID               string    `gorm:"column:id;primaryKey"`
	RepositorySSHURL string    `gorm:"column:repository_ssh_url"`
	GitAuthorName    string    `gorm:"column:git_author_name"`
	GitAuthorEmail   string    `gorm:"column:git_author_email"`
	QualityCommands  jsonValue `gorm:"column:quality_commands;type:jsonb"`
}

func (repositoryBindingRecord) TableName() string { return "repository_bindings" }
