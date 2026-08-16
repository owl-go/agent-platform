package gormrepo_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/identity/domain"
	identitygorm "agent-platform/backend/internal/data/identity/gormrepo"
	"agent-platform/backend/internal/infrastructure/gormdb"
)

func TestRepositoryResolvesOrganizationScopedIdentity(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	suffix := fmt.Sprintf("identity-%d", time.Now().UnixNano())
	var organizationID, userID string
	query := `
		WITH org AS (
			INSERT INTO organizations (slug, name) VALUES (?, 'Identity Test') RETURNING id
		), platform_user AS (
			INSERT INTO users (organization_id, oidc_subject, email, display_name)
			SELECT id, ?, ? || '@example.test', 'Identity Test' FROM org RETURNING id, organization_id
		), grant_row AS (
			INSERT INTO role_grants (organization_id, user_id, role)
			SELECT organization_id, id, 'run_operator' FROM platform_user
		)
		SELECT organization_id::text, id::text AS user_id FROM platform_user`
	var row struct {
		OrganizationID string `gorm:"column:organization_id"`
		UserID         string `gorm:"column:user_id"`
	}
	if err := tx.Raw(query, suffix, suffix, suffix).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	organizationID, userID = row.OrganizationID, row.UserID

	repository := identitygorm.New(tx)
	principal, err := repository.FindPrincipal(context.Background(), domain.VerifiedIdentity{
		Subject: suffix, OrganizationSlug: suffix,
	})
	if err != nil {
		t.Fatal(err)
	}
	if principal.OrganizationID != organizationID || principal.UserID != userID || len(principal.Grants) != 1 || principal.Grants[0].Role != domain.RunOperator {
		t.Fatalf("Principal = %+v", principal)
	}
	if _, err := repository.FindPrincipal(context.Background(), domain.VerifiedIdentity{Subject: suffix, OrganizationSlug: "other"}); err != domain.ErrUserNotFound {
		t.Fatalf("other Organization error = %v", err)
	}
	if _, err := repository.FindRunScope(context.Background(), "6ba7b810-9dad-11d1-80b4-00c04fd430c8"); err != domain.ErrRunNotFound {
		t.Fatalf("missing Run error = %v", err)
	}
}

func openIntegrationDatabase(t *testing.T) *gormdb.Database {
	t.Helper()
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("EXECUTION_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{
		DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2,
		ConnectionMaxIdle: time.Minute, ConnectionMaxLife: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
