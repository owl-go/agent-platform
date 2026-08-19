package gormuow

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	agentapplication "agent-platform/backend/internal/biz/agentlifecycle/application"
	approvalapplication "agent-platform/backend/internal/biz/approval/application"
	collaborationapplication "agent-platform/backend/internal/biz/collaboration/application"
	executionapplication "agent-platform/backend/internal/biz/execution/application"
	modelapplication "agent-platform/backend/internal/biz/modelcatalog/application"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	sourceapplication "agent-platform/backend/internal/biz/sourcecontrol/application"
	transaction "agent-platform/backend/internal/biz/transaction"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/agentlifecycle/draftvalidator"
	agentgorm "agent-platform/backend/internal/data/agentlifecycle/gormrepo"
	approvalgorm "agent-platform/backend/internal/data/approval/gormrepo"
	collaborationgorm "agent-platform/backend/internal/data/collaboration/gormrepo"
	executiongorm "agent-platform/backend/internal/data/execution/gormrepo"
	modelgorm "agent-platform/backend/internal/data/modelcatalog/gormrepo"
	runtimegorm "agent-platform/backend/internal/data/runtimecatalog/gormrepo"
	"agent-platform/backend/internal/data/sourcecontrol/bindingvalidator"
	sourcegorm "agent-platform/backend/internal/data/sourcecontrol/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type Clock interface{ Now() time.Time }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type UnitOfWork struct {
	db               *gorm.DB
	clock            Clock
	webhookTargetURL string
	evidenceVerifier runtimeapplication.EvidenceVerifier
}

var _ transaction.IdempotentTransactionManager = (*UnitOfWork)(nil)

func New(db *gorm.DB) *UnitOfWork { return NewWithClock(db, systemClock{}) }

func NewWithWebhook(db *gorm.DB, targetURL string) *UnitOfWork {
	return &UnitOfWork{db: db, clock: systemClock{}, webhookTargetURL: targetURL}
}

func NewWithEvidenceVerifier(db *gorm.DB, verifier runtimeapplication.EvidenceVerifier) *UnitOfWork {
	return &UnitOfWork{db: db, clock: systemClock{}, evidenceVerifier: verifier}
}

func NewWithWebhookAndEvidenceVerifier(db *gorm.DB, targetURL string, verifier runtimeapplication.EvidenceVerifier) *UnitOfWork {
	return &UnitOfWork{db: db, clock: systemClock{}, webhookTargetURL: targetURL, evidenceVerifier: verifier}
}

func NewWithClock(db *gorm.DB, clock Clock) *UnitOfWork {
	return &UnitOfWork{db: db, clock: clock}
}

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

func (jsonValue) GormDataType() string                          { return "json" }
func (jsonValue) GormDBDataType(*gorm.DB, *schema.Field) string { return "JSONB" }

type idempotencyRecord struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey"`
	Key            string    `gorm:"column:key;primaryKey"`
	Operation      string    `gorm:"column:operation;primaryKey"`
	RequestSHA256  string    `gorm:"column:request_sha256"`
	ResponseStatus *int      `gorm:"column:response_status"`
	ResponseBody   jsonValue `gorm:"column:response_body;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	ExpiresAt      time.Time `gorm:"column:expires_at"`
}

type auditRecord struct {
	OrganizationID string    `gorm:"column:organization_id"`
	TeamID         *string   `gorm:"column:team_id"`
	ActorUserID    *string   `gorm:"column:actor_user_id"`
	Action         string    `gorm:"column:action"`
	ResourceType   string    `gorm:"column:resource_type"`
	ResourceID     string    `gorm:"column:resource_id"`
	Details        jsonValue `gorm:"column:details;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (auditRecord) TableName() string { return "audit_events" }

func (idempotencyRecord) TableName() string { return "idempotency_keys" }

func (unit *UnitOfWork) Execute(ctx context.Context, request transaction.IdempotencyRequest, handler transaction.IdempotencyHandler) (transaction.IdempotencyResult, error) {
	if unit.db == nil || unit.clock == nil || handler == nil {
		return transaction.IdempotencyResult{}, fmt.Errorf("idempotent Unit of Work dependencies are required")
	}
	now := unit.clock.Now().UTC()
	if err := request.Validate(now); err != nil {
		return transaction.IdempotencyResult{}, err
	}
	var output transaction.IdempotencyResult
	err := unit.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh := idempotencyRecord{
			OrganizationID: request.OrganizationID, Key: request.Key, Operation: request.Operation,
			RequestSHA256: request.RequestSHA256, CreatedAt: now, ExpiresAt: request.ExpiresAt.UTC(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fresh).Error; err != nil {
			return fmt.Errorf("reserve Idempotency Key: %w", err)
		}
		var record idempotencyRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND key = ? AND operation = ?", request.OrganizationID, request.Key, request.Operation).
			Take(&record).Error; err != nil {
			return fmt.Errorf("lock Idempotency Key: %w", err)
		}
		if !record.ExpiresAt.After(now) {
			if err := tx.Delete(&record).Error; err != nil {
				return fmt.Errorf("delete expired Idempotency Key: %w", err)
			}
			if err := tx.Create(&fresh).Error; err != nil {
				return fmt.Errorf("replace expired Idempotency Key: %w", err)
			}
			record = fresh
		}
		if record.RequestSHA256 != request.RequestSHA256 {
			return transaction.ErrIdempotencyConflict
		}
		if record.ResponseStatus != nil && len(record.ResponseBody) > 0 {
			output = transaction.IdempotencyResult{
				Status: *record.ResponseStatus, Body: append(json.RawMessage(nil), record.ResponseBody...), Replayed: true,
			}
			return nil
		}

		workflows := gormtx.New(tx)
		result, err := handler(transaction.TransactionServices{
			RuntimeImages: runtimeapplication.NewWithEvidenceVerifier(runtimegorm.New(tx), unit.evidenceVerifier),
			Models:        modelapplication.New(modelgorm.New(tx)),
			SourceControl: sourceapplication.New(sourcegorm.New(tx)),
			Bindings:      sourceapplication.NewBindingService(sourcegorm.New(tx), bindingvalidator.New(tx)),
			Agents:        agentapplication.New(agentgorm.New(tx), draftvalidator.New(tx)),
			Approvals:     approvalapplication.New(approvalgorm.New(tx), bizworkflow.NewApproval(workflows)),
			Collaboration: collaborationapplication.NewWithLaunchCoordinator(collaborationgorm.New(tx), bizworkflow.NewLaunch(workflows)),
			Runs:          executionapplication.New(executiongorm.New(tx), bizworkflow.NewCompletion(workflows)),
		})
		if err != nil {
			return err
		}
		if err := result.Validate(); err != nil {
			return err
		}
		if request.ActorUserID != "" {
			if err := appendAudit(tx, request, result, now); err != nil {
				return err
			}
		}
		if unit.webhookTargetURL != "" {
			if err := appendWebhookDelivery(tx, request, result, now, unit.webhookTargetURL); err != nil {
				return err
			}
		}
		result.Replayed = false
		update := tx.Model(&idempotencyRecord{}).
			Where("organization_id = ? AND key = ? AND operation = ?", request.OrganizationID, request.Key, request.Operation).
			Updates(map[string]any{"response_status": result.Status, "response_body": jsonValue(result.Body)})
		if update.Error != nil {
			return fmt.Errorf("complete Idempotency Key: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("complete Idempotency Key: reservation disappeared")
		}
		var completed idempotencyRecord
		if err := tx.Where("organization_id = ? AND key = ? AND operation = ?", request.OrganizationID, request.Key, request.Operation).
			Take(&completed).Error; err != nil {
			return fmt.Errorf("read completed Idempotency Key: %w", err)
		}
		if completed.ResponseStatus == nil || len(completed.ResponseBody) == 0 {
			return fmt.Errorf("read completed Idempotency Key: response snapshot is missing")
		}
		output = transaction.IdempotencyResult{
			Status: *completed.ResponseStatus, Body: append(json.RawMessage(nil), completed.ResponseBody...), Replayed: false,
		}
		return nil
	})
	return output, err
}

func appendAudit(tx *gorm.DB, request transaction.IdempotencyRequest, result transaction.IdempotencyResult, now time.Time) error {
	resourceType, resourceID, action := auditIdentity(request.Operation, result.Body)
	details, err := json.Marshal(map[string]any{"response_status": result.Status, "idempotency_key": request.Key})
	if err != nil {
		return fmt.Errorf("encode Audit Event details: %w", err)
	}
	actorID := request.ActorUserID
	var teamID *string
	if request.TeamID != "" {
		value := request.TeamID
		teamID = &value
	}
	record := auditRecord{
		OrganizationID: request.OrganizationID, TeamID: teamID, ActorUserID: &actorID, Action: action,
		ResourceType: resourceType, ResourceID: resourceID, Details: details, CreatedAt: now,
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("append Audit Event: %w", err)
	}
	return nil
}

func appendWebhookDelivery(tx *gorm.DB, request transaction.IdempotencyRequest, result transaction.IdempotencyResult, now time.Time, targetURL string) error {
	resourceType, resourceID, action := auditIdentity(request.Operation, result.Body)
	payload, err := json.Marshal(map[string]any{
		"event_type": action, "organization_id": request.OrganizationID,
		"resource_type": resourceType, "resource_id": resourceID,
		"occurred_at": now, "data": map[string]any{"response_status": result.Status},
	})
	if err != nil {
		return fmt.Errorf("encode Webhook Delivery payload: %w", err)
	}
	record := map[string]any{
		"organization_id": request.OrganizationID, "event_type": action,
		"payload": jsonValue(payload), "target_url": targetURL,
		"state": "pending", "attempt_count": 0, "next_attempt_at": now, "created_at": now,
	}
	if err := tx.Table("webhook_deliveries").Create(record).Error; err != nil {
		return fmt.Errorf("append Webhook Delivery: %w", err)
	}
	return nil
}

func auditIdentity(operation string, body []byte) (string, string, string) {
	resourceID := "unknown"
	var response map[string]json.RawMessage
	if json.Unmarshal(body, &response) == nil {
		resourceID = responseID(response)
	} else if _, suffix, found := strings.Cut(operation, ":"); found && suffix != "" {
		resourceID = suffix
	}
	if resourceID == "unknown" {
		if _, suffix, found := strings.Cut(operation, ":"); found && suffix != "" {
			resourceID = suffix
		}
	}
	action, _, _ := strings.Cut(operation, ":")
	resourceType, _, _ := strings.Cut(action, ".")
	return resourceType, resourceID, action
}

func responseID(response map[string]json.RawMessage) string {
	var id string
	if json.Unmarshal(response["id"], &id) == nil && id != "" {
		return id
	}
	for _, key := range []string{"task", "candidate", "memory", "session"} {
		var nested struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(response[key], &nested) == nil && nested.ID != "" {
			return nested.ID
		}
	}
	return "unknown"
}
