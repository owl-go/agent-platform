package gormrepo_test

import (
	"context"
	"encoding/json"
	"os"
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
	if err := tx.Exec(`UPDATE runs SET state = 'running', ended_at = NULL WHERE id = ?`, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	approval, err := domain.Request(uuid.NewString(), run.ID, domain.KindPlan, json.RawMessage(`{"summary":"integration plan"}`), now)
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
}
