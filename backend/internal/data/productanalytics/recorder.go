package productanalytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	analytics "agent-platform/backend/internal/productanalytics"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Recorder struct {
	db     *gorm.DB
	logger *slog.Logger
}

type eventRecord struct {
	ID                  string    `gorm:"column:id"`
	Name                string    `gorm:"column:name"`
	AnonymousUserKey    string    `gorm:"column:anonymous_user_key"`
	AnonymousSubjectKey *string   `gorm:"column:anonymous_subject_key"`
	ObjectType          string    `gorm:"column:object_type"`
	Attributes          []byte    `gorm:"column:attributes;type:jsonb"`
	DedupKey            string    `gorm:"column:dedup_key"`
	OccurredAt          time.Time `gorm:"column:occurred_at"`
}

func (eventRecord) TableName() string { return "product_events" }

var safeValue = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
var safeTemplateID = regexp.MustCompile(`^template_[a-z0-9_-]{1,40}$`)

var allowedTokenValues = map[string]map[string]struct{}{
	"login_completed.identity_source":            tokenSet("oidc"),
	"default_execution_ready.runtime_type":       tokenSet("claude", "codex", "hermes", "openclaw", "pi"),
	"default_execution_ready.compatibility":      tokenSet("verified", "unverified"),
	"first_task_started.entry":                   tokenSet("session", "home", "template"),
	"first_response_succeeded.duration_bucket":   tokenSet("under_10s", "10s_30s", "30s_2m", "2m_10m", "over_10m"),
	"first_response_succeeded.specialist_type":   tokenSet("default", "expert", "team"),
	"first_response_failed.failure_stage":        tokenSet("configuration", "runtime_start", "authorization", "model", "tool", "budget", "event_delivery", "execution"),
	"first_response_failed.safe_error_code":      tokenSet("invalid_configuration", "runtime_unavailable", "authentication_failed", "model_failed", "command_failed", "budget_exhausted", "interrupted", "timed_out", "event_delivery_failed", "internal_adapter_error"),
	"workflow_created.source":                    tokenSet("manual", "session", "template", "import"),
	"workflow_second_run_succeeded.trigger_type": tokenSet("manual", "scheduled", "api"),
}

var allowedAttributes = map[string]map[string]string{
	"login_completed":               {"identity_source": "token", "first_login": "bool"},
	"default_execution_ready":       {"inherited": "bool", "runtime_type": "token", "compatibility": "token"},
	"first_task_started":            {"entry": "token", "template_id": "template", "has_file": "bool"},
	"first_response_succeeded":      {"duration_bucket": "token", "specialist_type": "token", "artifact_count": "int"},
	"first_response_failed":         {"failure_stage": "token", "safe_error_code": "token", "recoverable": "bool"},
	"workflow_save_started":         {"source_session": "bool", "prefilled_fields": "int"},
	"workflow_created":              {"source": "token", "has_schedule": "bool", "has_connector": "bool"},
	"workflow_second_run_succeeded": {"days_since_create": "int", "trigger_type": "token"},
}

var requiredAttributes = map[string][]string{
	"login_completed":               {"identity_source", "first_login"},
	"default_execution_ready":       {"inherited", "runtime_type", "compatibility"},
	"first_task_started":            {"entry", "has_file"},
	"first_response_succeeded":      {"duration_bucket", "specialist_type", "artifact_count"},
	"first_response_failed":         {"failure_stage", "safe_error_code", "recoverable"},
	"workflow_save_started":         {"source_session", "prefilled_fields"},
	"workflow_created":              {"source", "has_schedule", "has_connector"},
	"workflow_second_run_succeeded": {"days_since_create", "trigger_type"},
}

func New(db *gorm.DB, logger *slog.Logger) (*Recorder, error) {
	if db == nil || logger == nil {
		return nil, fmt.Errorf("product analytics database and logger are required")
	}
	return &Recorder{db: db, logger: logger}, nil
}

func (recorder *Recorder) LoginCompleted(ctx context.Context, ownerID, identitySource string) {
	recorder.record(ctx, "login_completed", ownerID, "", "account", map[string]any{"identity_source": identitySource, "first_login": true}, "user")
}

func (recorder *Recorder) DefaultExecutionReady(ctx context.Context, ownerID string, inherited bool, runtimeType, compatibility string) {
	recorder.record(ctx, "default_execution_ready", ownerID, "", "account", map[string]any{"inherited": inherited, "runtime_type": runtimeType, "compatibility": compatibility}, "user")
}

func (recorder *Recorder) FirstTaskStarted(ctx context.Context, ownerID, sessionID, entry string, hasFile bool) {
	recorder.record(ctx, "first_task_started", ownerID, sessionID, "session", map[string]any{"entry": entry, "has_file": hasFile}, "user")
}

func (recorder *Recorder) SessionTerminal(ctx context.Context, observation analytics.SessionTerminalObservation) {
	attributes := map[string]any{}
	name := "first_response_succeeded"
	if observation.State == "succeeded" {
		duration := observation.Duration
		var persisted struct {
			ElapsedMS int64 `gorm:"column:elapsed_ms"`
		}
		if err := recorder.db.WithContext(ctx).Raw(`
			SELECT message.elapsed_ms
			FROM session_messages message
			JOIN sessions session ON session.id = message.session_id
			WHERE message.id = ? AND message.session_id = ? AND session.owner_user_id = ?`, observation.MessageID, observation.SessionID, observation.OwnerID).Scan(&persisted).Error; err != nil {
			recorder.logger.WarnContext(ctx, "read Session response duration", "error", err)
		} else if persisted.ElapsedMS > 0 {
			duration = time.Duration(persisted.ElapsedMS) * time.Millisecond
		}
		attributes["duration_bucket"] = durationBucket(duration)
		attributes["specialist_type"] = observation.Specialist
		attributes["artifact_count"] = observation.ArtifactCount
	} else if observation.State == "failed" {
		name = "first_response_failed"
		attributes["failure_stage"] = observation.FailureStage
		attributes["safe_error_code"] = observation.SafeErrorCode
		attributes["recoverable"] = observation.Recoverable
	} else {
		return
	}
	recorder.record(ctx, name, observation.OwnerID, observation.SessionID, "session", attributes, "user")
}

func (recorder *Recorder) WorkflowSaveStarted(ctx context.Context, ownerID, sessionID string, prefilledFields int) {
	recorder.record(ctx, "workflow_save_started", ownerID, sessionID, "session", map[string]any{"source_session": sessionID != "", "prefilled_fields": prefilledFields}, "subject")
}

func (recorder *Recorder) WorkflowCreated(ctx context.Context, observation analytics.WorkflowCreatedObservation) {
	recorder.record(ctx, "workflow_created", observation.OwnerID, observation.WorkflowID, "workflow", map[string]any{"source": observation.Source, "has_schedule": observation.HasSchedule, "has_connector": observation.HasConnector}, "subject")
}

func (recorder *Recorder) WorkflowTerminal(ctx context.Context, observation analytics.WorkflowTerminalObservation) {
	if observation.State != "succeeded" {
		return
	}
	var facts struct {
		DaysSinceCreate int    `gorm:"column:days_since_create"`
		Trigger         string `gorm:"column:trigger"`
		Successes       int64  `gorm:"column:successes"`
	}
	err := recorder.db.WithContext(ctx).Raw(`
		SELECT GREATEST(0, floor(extract(epoch FROM (current_run.ended_at - workflow.created_at)) / 86400))::int AS days_since_create,
		       current_run.trigger,
		       (SELECT count(*) FROM runs succeeded WHERE succeeded.workflow_id = workflow.id AND succeeded.owner_user_id = workflow.owner_user_id AND succeeded.state = 'succeeded') AS successes
		FROM workflows workflow
		JOIN runs current_run ON current_run.id = ? AND current_run.workflow_id = workflow.id AND current_run.owner_user_id = workflow.owner_user_id
		WHERE workflow.id = ? AND workflow.owner_user_id = ?`, observation.RunID, observation.WorkflowID, observation.OwnerID).Scan(&facts).Error
	if err != nil {
		recorder.logger.WarnContext(ctx, "read Workflow retention facts", "error", err)
		return
	}
	if facts.Successes < 2 {
		return
	}
	recorder.record(ctx, "workflow_second_run_succeeded", observation.OwnerID, observation.WorkflowID, "workflow", map[string]any{"days_since_create": facts.DaysSinceCreate, "trigger_type": facts.Trigger}, "subject")
}

func (recorder *Recorder) record(ctx context.Context, name, ownerID, subjectID, objectType string, attributes map[string]any, dedupScope string) {
	row, err := newEvent(name, ownerID, subjectID, objectType, attributes, dedupScope, time.Now().UTC())
	if err != nil {
		recorder.logger.ErrorContext(ctx, "reject unsafe product event", "event", name, "error", err)
		return
	}
	if err := recorder.db.WithContext(context.WithoutCancel(ctx)).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedup_key"}}, DoNothing: true}).Create(&row).Error; err != nil {
		recorder.logger.WarnContext(ctx, "record product event", "event", name, "error", err)
	}
}

func newEvent(name, ownerID, subjectID, objectType string, attributes map[string]any, dedupScope string, occurredAt time.Time) (eventRecord, error) {
	if strings.TrimSpace(ownerID) == "" {
		return eventRecord{}, fmt.Errorf("owner is required")
	}
	allowed, exists := allowedAttributes[name]
	if !exists {
		return eventRecord{}, fmt.Errorf("unsupported event name")
	}
	if objectType != "account" && objectType != "session" && objectType != "workflow" && objectType != "run" {
		return eventRecord{}, fmt.Errorf("unsupported object type")
	}
	for _, key := range requiredAttributes[name] {
		if _, present := attributes[key]; !present {
			return eventRecord{}, fmt.Errorf("required attribute %q is missing", key)
		}
	}
	for key, value := range attributes {
		kind, ok := allowed[key]
		if !ok {
			return eventRecord{}, fmt.Errorf("attribute %q is not allowed for %s", key, name)
		}
		switch kind {
		case "token":
			text, ok := value.(string)
			if !ok || !safeValue.MatchString(text) {
				return eventRecord{}, fmt.Errorf("attribute %q is not a safe token", key)
			}
			if choices, constrained := allowedTokenValues[name+"."+key]; constrained {
				if _, accepted := choices[text]; !accepted {
					return eventRecord{}, fmt.Errorf("attribute %q is not an accepted value", key)
				}
			}
		case "bool":
			if _, ok := value.(bool); !ok {
				return eventRecord{}, fmt.Errorf("attribute %q is not a boolean", key)
			}
		case "int":
			number, ok := value.(int)
			if !ok || number < 0 || number > 1_000_000 {
				return eventRecord{}, fmt.Errorf("attribute %q is outside its safe range", key)
			}
		case "template":
			text, ok := value.(string)
			if !ok || !safeTemplateID.MatchString(text) {
				return eventRecord{}, fmt.Errorf("attribute %q is not a safe template identifier", key)
			}
		}
	}
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return eventRecord{}, fmt.Errorf("encode attributes: %w", err)
	}
	userKey := anonymousKey("user", ownerID)
	var subjectKey *string
	if subjectID != "" {
		value := anonymousKey(objectType, subjectID)
		subjectKey = &value
	}
	dedupValue := ownerID
	if dedupScope == "subject" && subjectID != "" {
		dedupValue = subjectID
	}
	return eventRecord{
		ID: uuid.NewString(), Name: name, AnonymousUserKey: userKey, AnonymousSubjectKey: subjectKey,
		ObjectType: objectType, Attributes: encoded, DedupKey: anonymousKey("dedup:"+name, dedupValue), OccurredAt: occurredAt,
	}, nil
}

func tokenSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func anonymousKey(namespace, value string) string {
	sum := sha256.Sum256([]byte("agent-workspace-product-analytics:" + namespace + ":" + value))
	return hex.EncodeToString(sum[:])
}

func durationBucket(duration time.Duration) string {
	switch {
	case duration < 10*time.Second:
		return "under_10s"
	case duration < 30*time.Second:
		return "10s_30s"
	case duration < 2*time.Minute:
		return "30s_2m"
	case duration < 10*time.Minute:
		return "2m_10m"
	default:
		return "over_10m"
	}
}
