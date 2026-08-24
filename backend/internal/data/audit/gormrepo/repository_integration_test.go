package gormrepo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/audit/application"
	"agent-platform/backend/internal/biz/audit/domain"
	"agent-platform/backend/internal/data/audit/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
)

func TestAuditSearchRemainsTeamScopedWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("AUDIT_DATABASE_DSN")
	if dsn == "" {
		t.Skip("AUDIT_DATABASE_DSN is required for PostgreSQL integration")
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
	var scope struct {
		OrganizationID string
		TeamID         string
		UserID         string
	}
	if err := tx.Raw(`SELECT organization_id::text, id::text AS team_id, (SELECT id::text FROM users WHERE organization_id = teams.organization_id LIMIT 1) AS user_id FROM teams ORDER BY created_at LIMIT 1`).Scan(&scope).Error; err != nil {
		t.Fatal(err)
	}
	if scope.TeamID == "" || scope.UserID == "" {
		t.Skip("organization, Team, and User fixtures are required")
	}
	if err := tx.Exec(`INSERT INTO audit_events (organization_id, team_id, actor_user_id, action, resource_type, resource_id, details, created_at) VALUES (?, ?, ?, 'run.cancel', 'run', 'integration-run', '{"response_status":200,"outcome":"succeeded"}'::jsonb, ?)`, scope.OrganizationID, scope.TeamID, scope.UserID, time.Now().UTC()).Error; err != nil {
		t.Fatal(err)
	}
	service := application.New(gormrepo.New(tx))
	values, err := service.Search(context.Background(), domain.Query{OrganizationID: scope.OrganizationID, TeamID: scope.TeamID, ResourceID: "integration-run", Limit: 10})
	if err != nil || len(values) != 1 || values[0].ActorUserID != scope.UserID {
		t.Fatalf("Search() = (%+v, %v)", values, err)
	}
	values, err = service.Search(context.Background(), domain.Query{OrganizationID: scope.OrganizationID, TeamID: scope.TeamID, ResourceID: "integration-run", Outcome: "succeeded", Limit: 10})
	if err != nil || len(values) != 1 || values[0].Outcome != "succeeded" {
		t.Fatalf("outcome Search() = (%+v, %v)", values, err)
	}
	values, err = service.Search(context.Background(), domain.Query{OrganizationID: scope.OrganizationID, TeamID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", ResourceID: "integration-run", Limit: 10})
	if err != nil || len(values) != 0 {
		t.Fatalf("cross-Team Search() = (%+v, %v)", values, err)
	}
}
