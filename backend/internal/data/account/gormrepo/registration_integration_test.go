package gormrepo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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
	loginCode := "0382"
	loginHash := strings.Repeat("b", 64)
	attempt := domain.RegistrationAttempt{ID: "attempt", Provider: domain.RegistrationWeChat, ConfigVersion: settings.Version, BrowserHash: "browser", LoginCode: loginCode, LoginCodeHash: loginHash, Status: "waiting", ExpiresAt: time.Now().Add(time.Minute), Version: 1}
	if err = repo.CreateAttempt(t.Context(), attempt); err != nil {
		t.Fatal(err)
	}
	byLogin, err := repo.AttemptByLoginCode(t.Context(), loginHash)
	if err != nil || byLogin.LoginCode != loginCode || byLogin.ID != attempt.ID {
		t.Fatal("login hash did not resolve encrypted challenge", err)
	}
	if _, err = repo.AttemptByCode(t.Context(), loginHash); err == nil {
		t.Fatal("login challenge accepted as OIDC exchange code")
	}
	duplicate := attempt
	duplicate.ID = "duplicate"
	if err = repo.CreateAttempt(t.Context(), duplicate); !errors.Is(err, domain.ErrLoginCodeConflict) {
		t.Fatal("duplicate login hash accepted")
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
	if strings.Contains(string(row.PayloadCiphertext), "private-person") || strings.Contains(string(row.PayloadCiphertext), loginCode) {
		t.Fatal("plaintext identity")
	}
	attempt.ID = "expired"
	attempt.Status = "waiting"
	attempt.ExpiresAt = time.Now().Add(-time.Second)
	attempt.Version = 1
	attempt.CodeHash = ""
	attempt.LoginCodeHash = ""
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

func TestRegistrationVerificationLimitsAreSharedAtomicAndExpire(t *testing.T) {
	db := registrationDB(t)
	box, _ := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	repos := []*RegistrationRepository{NewRegistration(db, box), NewRegistration(db, box)}
	sender := strings.Repeat("a", 64)
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- repos[i%2].ReserveWeChatVerification(context.Background(), sender) }()
	}
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !errors.Is(err, domain.ErrRegistrationRateLimited) {
			t.Fatal(err)
		}
	}
	if accepted != 5 {
		t.Fatalf("accepted %d concurrent sender guesses", accepted)
	}
	var global int
	if err := db.Raw("SELECT attempts FROM registration_verification_limits WHERE bucket_key='global'").Scan(&global).Error; err != nil || global != 5 {
		t.Fatal("blocked sender spent global budget", global, err)
	}
	for i := range 25 {
		if err := repos[0].ReserveWeChatVerification(t.Context(), fmt.Sprintf("%064x", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	if err := repos[1].ReserveWeChatVerification(t.Context(), strings.Repeat("b", 64)); !errors.Is(err, domain.ErrRegistrationRateLimited) {
		t.Fatal("global verification limit bypassed", err)
	}
	if err := db.Exec("UPDATE registration_verification_limits SET expires_at=?", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repos[1].ReserveWeChatVerification(t.Context(), sender); err != nil {
		t.Fatal("expired sender remained locked", err)
	}
}
func TestRegistrationShortCodeReservationSurvivesHandoffAndCapacityIsBounded(t *testing.T) {
	db := registrationDB(t)
	box, _ := secretcrypto.New(base64.RawStdEncoding.EncodeToString(make([]byte, 32)))
	repo := NewRegistration(db, box)
	a := domain.RegistrationAttempt{ID: "original", Provider: domain.RegistrationWeChat, LoginCode: "0007", LoginCodeHash: strings.Repeat("a", 64), Status: "waiting", Version: 1, ExpiresAt: time.Now().Add(5 * time.Minute)}
	if err := repo.CreateAttempt(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	// The OIDC code expires sooner, but the old message must not confirm a new browser.
	a.Status = "issued"
	a.ExpiresAt = time.Now().Add(-time.Second)
	if err := repo.TransitionAttempt(t.Context(), a, "waiting"); err != nil {
		t.Fatal(err)
	}
	duplicate := a
	duplicate.ID = "duplicate"
	duplicate.Status = "waiting"
	duplicate.ExpiresAt = time.Now().Add(time.Minute)
	if err := repo.CreateAttempt(t.Context(), duplicate); !errors.Is(err, domain.ErrLoginCodeConflict) {
		t.Fatal("handoff prematurely released the code", err)
	}
	for i := 1; i < 100; i++ {
		next := duplicate
		next.ID = fmt.Sprintf("attempt-%d", i)
		next.LoginCodeHash = fmt.Sprintf("%064x", i)
		if err := repo.CreateAttempt(t.Context(), next); err != nil {
			t.Fatal(err)
		}
	}
	next := duplicate
	next.ID = "overflow"
	next.LoginCodeHash = strings.Repeat("b", 64)
	if err := repo.CreateAttempt(t.Context(), next); err == nil {
		t.Fatal("short-code capacity was not bounded")
	}
	if err := db.Model(&registrationAttemptModel{}).Where("id=?", a.ID).Update("login_code_reserved_until", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateAttempt(t.Context(), duplicate); err != nil {
		t.Fatal("expired reservation not released", err)
	}
}
