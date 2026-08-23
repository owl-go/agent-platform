package gormrepo_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/approval/domain"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
)

func TestRunApprovalStateTransitionsWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("APPROVAL_DATABASE_DSN")
	if dsn == "" {
		t.Skip("APPROVAL_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 2, MaxIdleConnections: 1, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	var run struct {
		ID      string
		Version int64
		ActorID string
	}
	if err := tx.Raw(`SELECT id::text, version, created_by::text AS actor_id FROM runs ORDER BY created_at LIMIT 1`).Scan(&run).Error; err != nil {
		t.Fatal(err)
	}
	if run.ID == "" {
		t.Skip("a Run fixture is required for Approval integration")
	}
	if err := tx.Exec(`INSERT INTO role_grants (organization_id, team_id, user_id, role)
		SELECT task.organization_id, task.team_id, run.created_by, 'agent_user'
		FROM runs run JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id WHERE run.id = ?`, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`UPDATE runs SET state = 'running', ended_at = NULL WHERE id = ?`, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`UPDATE coding_tasks SET state = 'active', completed_at = NULL WHERE id = (
		SELECT session.coding_task_id FROM runs run JOIN sessions session ON session.id = run.session_id WHERE run.id = ?)`, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	approval, err := domain.Request(uuid.NewString(), run.ID, domain.KindPlan, json.RawMessage(`{"summary":"integration plan"}`), run.ActorID, now)
	if err != nil {
		t.Fatal(err)
	}
	workflow := bizworkflow.NewApproval(gormtx.New(tx))
	if err := workflow.Request(context.Background(), approval, run.Version, now); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := tx.Raw(`SELECT state FROM runs WHERE id = ?`, run.ID).Scan(&state).Error; err != nil || state != "waiting_confirmation" {
		t.Fatalf("Run state after request = %q, %v", state, err)
	}
	if err := approval.Decide(true, run.ActorID, "reviewed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Decide(context.Background(), approval, 1, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Raw(`SELECT state FROM runs WHERE id = ?`, run.ID).Scan(&state).Error; err != nil || state != "running" {
		t.Fatalf("Run state after approval = %q, %v", state, err)
	}
	var approvedVersion int64
	if err := tx.Raw(`SELECT version FROM runs WHERE id = ?`, run.ID).Scan(&approvedVersion).Error; err != nil {
		t.Fatal(err)
	}
	rejection, err := domain.Request(uuid.NewString(), run.ID, domain.KindHighRiskChange, json.RawMessage(`{"risk_reason":"unsafe network write"}`), run.ActorID, now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := workflow.Request(context.Background(), rejection, approvedVersion, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := rejection.Decide(false, run.ActorID, "risk denied", now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := workflow.Decide(context.Background(), rejection, 1, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	var rejected struct {
		State         string
		TerminalError []byte
	}
	if err := tx.Raw(`SELECT state, terminal_error FROM runs WHERE id = ?`, run.ID).Scan(&rejected).Error; err != nil {
		t.Fatal(err)
	}
	if rejected.State != "failed" || !json.Valid(rejected.TerminalError) || !jsonContains(rejected.TerminalError, "approval_rejected") {
		t.Fatalf("Run after rejection = %+v", rejected)
	}
	var terminalEvents int64
	if err := tx.Raw(`SELECT count(*) FROM run_events WHERE run_id = ? AND event_type = 'run.failed'`, run.ID).Scan(&terminalEvents).Error; err != nil {
		t.Fatal(err)
	}
	if terminalEvents != 1 {
		t.Fatalf("rejected Run terminal event count = %d, want 1", terminalEvents)
	}
	var projection struct{ TaskState, Status string }
	if err := tx.Raw(`
		SELECT task.state AS task_state, message.content->>'status' AS status
		FROM runs run JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id
		JOIN session_messages message ON message.session_id = session.id AND message.run_id = run.id
		WHERE run.id = ? AND message.content->>'type' = 'run_result'`, run.ID).Scan(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if projection.TaskState != "waiting_for_user" || projection.Status != "failed" {
		t.Fatalf("rejected Run task projection = %+v", projection)
	}
}

func jsonContains(value []byte, needle string) bool {
	return string(value) != "" && strings.Contains(string(value), needle)
}
