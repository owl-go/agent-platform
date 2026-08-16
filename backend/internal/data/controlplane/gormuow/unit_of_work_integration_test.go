package gormuow_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	"agent-platform/backend/internal/biz/runtimecatalog/domain"
	transaction "agent-platform/backend/internal/biz/transaction"
	"agent-platform/backend/internal/data/controlplane/gormuow"
	"agent-platform/backend/internal/infrastructure/gormdb"
)

func TestUnitOfWorkExecutesConcurrentIdempotentWriteOnce(t *testing.T) {
	database := openIntegrationDatabase(t)
	suffix := fmt.Sprintf("idempotency-%d", time.Now().UnixNano())
	var organizationID string
	if err := database.ORM().Raw(`INSERT INTO organizations (slug, name) VALUES (?, 'Idempotency Test') RETURNING id::text`, suffix).Scan(&organizationID).Error; err != nil {
		t.Fatal(err)
	}
	var actorUserID string
	if err := database.ORM().Raw(`INSERT INTO users (organization_id, oidc_subject, email, display_name) VALUES (?, ?, ?, 'Idempotency Actor') RETURNING id::text`, organizationID, suffix, suffix+"@example.test").Scan(&actorUserID).Error; err != nil {
		t.Fatal(err)
	}
	digestBytes := sha256.Sum256([]byte(suffix))
	digest := "registry.example/idempotency@sha256:" + hex.EncodeToString(digestBytes[:])
	t.Cleanup(func() {
		database.ORM().Exec("DELETE FROM webhook_deliveries WHERE organization_id = ?", organizationID)
		database.ORM().Exec("DELETE FROM audit_events WHERE organization_id = ?", organizationID)
		database.ORM().Exec("DELETE FROM idempotency_keys WHERE organization_id = ?", organizationID)
		database.ORM().Exec("DELETE FROM runtime_images WHERE image_digest = ?", digest)
		database.ORM().Exec("DELETE FROM users WHERE id = ?", actorUserID)
		database.ORM().Exec("DELETE FROM organizations WHERE id = ?", organizationID)
	})

	request := transaction.IdempotencyRequest{
		OrganizationID: organizationID, ActorUserID: actorUserID, Key: "register-runtime", Operation: "runtime-image.register",
		RequestSHA256: strings.Repeat("a", 64), ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	var executions atomic.Int32
	handler := func(services transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		executions.Add(1)
		image, err := services.RuntimeImages.Register(context.Background(), runtimeapplication.RegisterCommand{
			Runtime: domain.Claude, CLIVersion: "test", AdapterVersion: "test", ImageDigest: digest,
		})
		if err != nil {
			return transaction.IdempotencyResult{}, err
		}
		time.Sleep(50 * time.Millisecond)
		body, _ := json.Marshal(map[string]string{"id": image.ID})
		return transaction.IdempotencyResult{Status: 201, Body: body}, nil
	}

	unit := gormuow.NewWithWebhook(database.ORM(), "https://hooks.example.test/agent-platform")
	results := make([]transaction.IdempotencyResult, 2)
	errorsFound := make([]error, 2)
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errorsFound[index] = unit.Execute(context.Background(), request, handler)
		}(index)
	}
	wait.Wait()
	for index, err := range errorsFound {
		if err != nil {
			t.Fatalf("Execute[%d] error = %v", index, err)
		}
	}
	if executions.Load() != 1 || results[0].Replayed == results[1].Replayed || string(results[0].Body) != string(results[1].Body) {
		t.Fatalf("executions=%d results=%+v", executions.Load(), results)
	}
	var auditCount int64
	if err := database.ORM().Raw(`SELECT count(*) FROM audit_events WHERE organization_id = ? AND actor_user_id = ? AND action = 'runtime-image.register'`, organizationID, actorUserID).Scan(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("idempotent write created %d Audit Events, want 1", auditCount)
	}
	var deliveryCount int64
	if err := database.ORM().Raw(`SELECT count(*) FROM webhook_deliveries WHERE organization_id = ? AND event_type = 'runtime-image.register' AND target_url = 'https://hooks.example.test/agent-platform'`, organizationID).Scan(&deliveryCount).Error; err != nil {
		t.Fatal(err)
	}
	if deliveryCount != 1 {
		t.Fatalf("idempotent write created %d Webhook Deliveries, want 1", deliveryCount)
	}

	conflict := request
	conflict.RequestSHA256 = strings.Repeat("b", 64)
	if _, err := unit.Execute(context.Background(), conflict, handler); !errors.Is(err, transaction.ErrIdempotencyConflict) {
		t.Fatalf("conflicting request error = %v", err)
	}

	failed := request
	failed.Key = "failed-write"
	writeError := errors.New("business write failed")
	if _, err := unit.Execute(context.Background(), failed, func(transaction.TransactionServices) (transaction.IdempotencyResult, error) {
		return transaction.IdempotencyResult{}, writeError
	}); !errors.Is(err, writeError) {
		t.Fatalf("failed write error = %v", err)
	}
	var count int64
	if err := database.ORM().Raw(`SELECT count(*) FROM idempotency_keys WHERE organization_id = ? AND key = 'failed-write'`, organizationID).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed write left %d Idempotency Key rows", count)
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
