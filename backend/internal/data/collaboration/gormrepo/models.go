package gormrepo

import (
	"database/sql/driver"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/collaboration/domain"

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
		return fmt.Errorf("scan Collaboration JSON from %T", source)
	}
	return nil
}

func (jsonValue) GormDataType() string                          { return "json" }
func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type taskRecord struct {
	ID, OrganizationID, TeamID, AgentReleaseID, CreatedBy string
	Title, RequestText                                    string
	IssueSnapshot                                         jsonValue `gorm:"type:jsonb"`
	State                                                 domain.TaskState
	CreatedAt, UpdatedAt                                  time.Time
	CompletedAt                                           *time.Time
	Version                                               int64
}

func (taskRecord) TableName() string { return "coding_tasks" }

type sessionRecord struct {
	ID, CodingTaskID, RepositoryBindingID       string
	TargetBranch, ReviewBranch, WorkspaceVolume string
	SessionMemory                               jsonValue `gorm:"type:jsonb"`
	RunCount                                    int
	CreatedAt, UpdatedAt                        time.Time
	Version                                     int64
}

func (sessionRecord) TableName() string { return "sessions" }

type messageRecord struct {
	ID           int64
	SessionID    string
	RunID        *string
	AuthorType   domain.MessageAuthor
	AuthorUserID *string
	Content      jsonValue `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (messageRecord) TableName() string { return "session_messages" }

type candidateRecord struct {
	ID, AgentID, CodingTaskID string
	ProposedContent           string
	State                     domain.MemoryCandidateState
	ProposedAt                time.Time
	DecidedBy                 *string
	DecidedAt                 *time.Time
	ResultingMemoryID         *string
}

func (candidateRecord) TableName() string { return "memory_candidates" }

type memoryRecord struct {
	ID, AgentID, Content, ApprovedBy string
	SourceTaskID                     *string
	Enabled                          bool
	CreatedAt, UpdatedAt             time.Time
	DeletedAt                        *time.Time
	Version                          int64
}

func (memoryRecord) TableName() string { return "agent_memories" }

type launchSelection struct {
	AgentID, RepositoryBindingID, RuntimeImageID, ConfiguredModelID string
	RepositoryBindingSnapshot, ConfiguredModelSnapshot              jsonValue
	ModelBudget, ExecutionLimits                                    jsonValue
	ReleaseStatus, RuntimeStatus                                    string
	ModelEnabled, BindingValid                                      bool
	TargetBranch, ModelID, ModelEndpoint                            string
	ModelCredentialProfileID                                        string
	ModelSecretRef, GitSecretRef                                    string
	BuildCredentialBindings                                         jsonValue
}
