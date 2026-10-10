package gormrepo

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"github.com/google/uuid"
)

func TestProviderDuplicateNameReturnsBusinessError(t *testing.T) {
	db := conversationTestDatabase(t)
	// Use the real API/Worker database factory, including its error translation.
	var databaseName string
	if err := db.Raw("SELECT current_database()").Scan(&databaseName).Error; err != nil {
		t.Fatal(err)
	}
	dsn, err := url.Parse(os.Getenv("WORKSPACE_TEST_POSTGRES_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	dsn.Path = "/" + databaseName
	production, err := gormdb.OpenWithoutMigrations(context.Background(), gormdb.Config{
		DSN: dsn.String(), MaxOpenConnections: 2, MaxIdleConnections: 1,
		ConnectionMaxIdle: time.Minute, ConnectionMaxLife: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = production.Close() })
	db = production.ORM()
	owner := uuid.NewString()
	if err := db.Exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner).Error; err != nil {
		t.Fatal(err)
	}
	repo := New(db, nil)
	ctx := context.Background()
	input := domain.ModelProviderConnection{Name: "白白AI", ProviderType: "openai", Endpoint: "https://example.test/v1", Protocols: []string{"openai_chat"}, VerificationStatus: "unverified"}
	first, err := repo.CreateModelProviderConnection(ctx, owner, input, []byte("fixture"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateModelProviderConnection(ctx, owner, input, []byte("fixture"), nil)
	if err == nil || !errors.Is(err, domain.ErrProviderNameConflict) {
		t.Fatalf("duplicate create must return name conflict, got %v", err)
	}
	input.Name = "Second"
	second, err := repo.CreateModelProviderConnection(ctx, owner, input, []byte("fixture"), nil)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = first.Name
	_, err = repo.UpdateModelProviderConnection(ctx, owner, second.ID, input, nil, nil, second.Version)
	if err == nil || !errors.Is(err, domain.ErrProviderNameConflict) {
		t.Fatalf("duplicate rename must return name conflict, got %v", err)
	}
	unchanged, err := repo.getProviderConnection(ctx, owner, second.ID)
	if err != nil || unchanged.Name != "Second" || unchanged.Version != second.Version {
		t.Fatalf("failed rename changed record: %+v %v", unchanged, err)
	}
}
