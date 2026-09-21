package gormrepo

import (
	"context"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/infrastructure/gormdb"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func rateLimitTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKSPACE_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })

	database := "aiapplication_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec(`CREATE DATABASE "` + database + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(`DROP DATABASE "` + database + `" WITH (FORCE)`).Error; err != nil {
			t.Error(err)
		}
	})

	parsed.Path = "/" + database
	db, err := gorm.Open(postgres.Open(parsed.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestConsumeExternalRateIsAtomicPerMinute(t *testing.T) {
	db := rateLimitTestDatabase(t)
	var migrationCount int64
	if err := db.Table("schema_migrations").Where("name = ?", "000046_external_rate_limits.sql").Count(&migrationCount).Error; err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("rate-limit migration count = %d, want 1", migrationCount)
	}
	var embeddingType string
	if err := db.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = 'knowledge_chunks' AND column_name = 'embedding'`).Scan(&embeddingType).Error; err != nil {
		t.Fatal(err)
	}
	if embeddingType != "jsonb" {
		t.Fatalf("stock PostgreSQL embedding type = %q, want jsonb fallback", embeddingType)
	}
	repository := New(db, nil)
	now := time.Date(2026, time.September, 21, 4, 5, 30, 0, time.UTC)
	const attempts = 12
	const limit = 3
	start := make(chan struct{})
	results := make(chan bool, attempts)
	errors := make(chan error, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			allowed, err := repository.ConsumeExternalRate(context.Background(), "visitor", "visitor-hash", now, limit)
			if err != nil {
				errors <- err
				return
			}
			results <- allowed
		}()
	}
	close(start)
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	allowedCount := 0
	for allowed := range results {
		if allowed {
			allowedCount++
		}
	}
	if allowedCount != limit {
		t.Fatalf("allowed requests = %d, want %d", allowedCount, limit)
	}

	allowed, err := repository.ConsumeExternalRate(context.Background(), "visitor", "visitor-hash", now.Add(time.Minute), limit)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first request in the next minute was rejected")
	}
}
