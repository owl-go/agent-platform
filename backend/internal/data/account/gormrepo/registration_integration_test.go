package gormrepo

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"agent-platform/backend/internal/secretcrypto"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func registrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKSPACE_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), cfg)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, _ := admin.DB()
	t.Cleanup(func() { _ = adminSQL.Close() })
	name := "registration_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = admin.Exec(`CREATE DATABASE "` + name + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(`DROP DATABASE "` + name + `" WITH (FORCE)`).Error; err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + name
	db, err := gorm.Open(postgres.Open(parsed.String()), cfg)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}
func TestRegistrationRepositoryEncryptedSettingsAuditCASAndAttemptConsumption(t *testing.T) {
	db := registrationDB(t)
	box, err := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewRegistration(db, box)
	accounts := New(db)
	admin, err := accounts.EnsureAdministrator(t.Context(), domain.User{OIDCSubject: "admin-sub", Username: "admin", Email: "admin@example.test", DisplayName: "Admin", Administrator: true})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := repo.SaveSettings(t.Context(), admin.ID, domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: true, AppID: "app", AppSecret: "sensitive-secret", TenantKey: "enterprise"}, 0, "open enterprise registration")
	if err != nil {
		t.Fatal(err)
	}
	var stored registrationSettingsModel
	if err = db.Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored.ConfigCiphertext), "sensitive-secret") {
		t.Fatal("plaintext credentials")
	}
	if err = repo.MarkReady(t.Context(), settings.Provider, settings.Version); err != nil {
		t.Fatal(err)
	}
	read, err := repo.Settings(t.Context(), settings.Provider)
	if err != nil || read.AppSecret != "sensitive-secret" || !read.Ready {
		t.Fatal("settings not restored", err)
	}
	if _, err = repo.SaveSettings(t.Context(), admin.ID, settings, 0, "stale"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale mutation accepted", err)
	}
	events, err := accounts.ListGovernanceAuditEvents(t.Context(), 10)
	if err != nil || len(events) != 1 || events[0].Action != "registration_method_updated" {
		t.Fatal("missing atomic audit", err)
	}
	// Optional email permits multiple external users while real emails remain unique.
	for _, sub := range []string{"scan-one", "scan-two"} {
		if _, err = accounts.CreateUser(t.Context(), domain.User{OIDCSubject: sub, Username: sub, DisplayName: "External User"}); err != nil {
			t.Fatal("no-email registration failed", err)
		}
	}
	attempt := domain.RegistrationAttempt{ID: "attempt", Provider: settings.Provider, ConfigVersion: settings.Version, BrowserHash: "browser", Status: "waiting", ExpiresAt: time.Now().Add(time.Minute), Version: 1}
	if err = repo.CreateAttempt(t.Context(), attempt); err != nil {
		t.Fatal(err)
	}
	attempt.Status = "verified"
	attempt.Identity = domain.RegistrationIdentity{Subject: "private-person"}
	if err = repo.TransitionAttempt(t.Context(), attempt, "waiting"); err != nil {
		t.Fatal(err)
	}
	attempt, err = repo.Attempt(t.Context(), attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	attempt.Status = "issued"
	attempt.CodeHash = "hash-only"
	if err = repo.TransitionAttempt(t.Context(), attempt, "verified"); err != nil {
		t.Fatal(err)
	}
	issued, err := repo.AttemptByCode(t.Context(), "hash-only")
	if err != nil {
		t.Fatal(err)
	}
	issued.Status = "consumed"
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repo.TransitionAttempt(context.Background(), issued, "issued") }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("code consumed %d times", success)
	}
	var row registrationAttemptModel
	_ = db.Take(&row).Error
	if strings.Contains(string(row.PayloadCiphertext), "private-person") {
		t.Fatal("plaintext identity")
	}
	attempt.ID = "expired"
	attempt.Status = "waiting"
	attempt.ExpiresAt = time.Now().Add(-time.Second)
	attempt.Version = 1
	attempt.CodeHash = ""
	if err = repo.CreateAttempt(t.Context(), attempt); err != nil {
		t.Fatal(err)
	}
	attempt.Status = "verified"
	if err = repo.TransitionAttempt(t.Context(), attempt, "waiting"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("expired state changed")
	}
	fresh := domain.RegistrationAttempt{ID: "fresh", Provider: settings.Provider, Status: "waiting", Version: 1, ExpiresAt: time.Now().Add(time.Minute)}
	if err = repo.CreateAttempt(t.Context(), fresh); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Attempt(t.Context(), "expired"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("expired attempt not cleaned")
	}
}
