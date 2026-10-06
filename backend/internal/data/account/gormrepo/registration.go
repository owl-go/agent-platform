package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
	"agent-platform/backend/internal/secretcrypto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RegistrationRepository struct {
	db  *gorm.DB
	box *secretcrypto.Box
}

func NewRegistration(db *gorm.DB, box *secretcrypto.Box) *RegistrationRepository {
	return &RegistrationRepository{db: db, box: box}
}

type registrationSettingsModel struct {
	Provider         string `gorm:"primaryKey"`
	Enabled          bool
	Ready            bool
	ConfigCiphertext []byte
	Version          int64
	UpdatedAt        time.Time
}

func (registrationSettingsModel) TableName() string { return "registration_settings" }

type registrationAttemptModel struct {
	ID                     string `gorm:"primaryKey"`
	Provider               string
	Status                 string
	CodeHash               *string
	LoginCodeHash          *string
	LoginCodeReservedUntil *time.Time
	PayloadCiphertext      []byte
	ExpiresAt              time.Time
	Version                int64
}

func (registrationAttemptModel) TableName() string { return "registration_attempts" }
func (r *RegistrationRepository) Settings(ctx context.Context, provider string) (domain.RegistrationSettings, error) {
	if !domain.RegistrationProvider(provider) {
		return domain.RegistrationSettings{}, domain.ErrNotFound
	}
	var row registrationSettingsModel
	if err := r.db.WithContext(ctx).Where("provider=?", provider).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.RegistrationSettings{Provider: provider}, nil
	} else if err != nil {
		return domain.RegistrationSettings{}, err
	}
	plain, err := r.box.Decrypt(row.ConfigCiphertext, "registration-settings:"+provider)
	if err != nil {
		return domain.RegistrationSettings{}, err
	}
	var s domain.RegistrationSettings
	if err = json.Unmarshal(plain, &s); err != nil {
		return s, fmt.Errorf("decode registration settings")
	}
	s.Provider = provider
	s.Version = row.Version
	s.Enabled = row.Enabled
	s.Ready = row.Ready
	return s, nil
}
func (r *RegistrationRepository) SaveSettings(ctx context.Context, actor string, s domain.RegistrationSettings, expected int64, reason string) (domain.RegistrationSettings, error) {
	s.Version = expected + 1
	s.Ready = false
	plain, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	cipher, err := r.box.Encrypt(plain, "registration-settings:"+s.Provider)
	if err != nil {
		return s, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if expected == 0 {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&registrationSettingsModel{Provider: s.Provider, Enabled: s.Enabled, ConfigCiphertext: cipher, Version: s.Version})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrConflict
			}
		} else {
			result := tx.Model(&registrationSettingsModel{}).Where("provider=? AND version=?", s.Provider, expected).Updates(map[string]any{"enabled": s.Enabled, "ready": false, "config_ciphertext": cipher, "version": s.Version, "updated_at": time.Now().UTC()})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrConflict
			}
		}
		detail, _ := json.Marshal(map[string]int64{"enabled": boolMetric(s.Enabled), "version": s.Version})
		return tx.Create(&governanceAuditModel{ActorUserID: actor, Action: "registration_method_updated", TargetType: "registration_method", TargetID: s.Provider, Reason: reason, Detail: detail, OccurredAt: time.Now().UTC()}).Error
	})
	return s, err
}
func boolMetric(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
func (r *RegistrationRepository) MarkReady(ctx context.Context, provider string, version int64) error {
	result := r.db.WithContext(ctx).Model(&registrationSettingsModel{}).Where("provider=? AND version=?", provider, version).Update("ready", true)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}
func (r *RegistrationRepository) attemptModel(a domain.RegistrationAttempt) (registrationAttemptModel, error) {
	plain, err := json.Marshal(a)
	if err != nil {
		return registrationAttemptModel{}, err
	}
	cipher, err := r.box.Encrypt(plain, "registration-attempt:"+a.ID)
	if err != nil {
		return registrationAttemptModel{}, err
	}
	var code *string
	if a.CodeHash != "" {
		code = &a.CodeHash
	}
	var loginCode *string
	if a.LoginCodeHash != "" {
		loginCode = &a.LoginCodeHash
	}
	return registrationAttemptModel{ID: a.ID, Provider: a.Provider, Status: a.Status, CodeHash: code, LoginCodeHash: loginCode, PayloadCiphertext: cipher, ExpiresAt: a.ExpiresAt, Version: a.Version}, nil
}
func (r *RegistrationRepository) CreateAttempt(ctx context.Context, a domain.RegistrationAttempt) error {
	row, err := r.attemptModel(a)
	if a.LoginCodeHash != "" {
		row.LoginCodeReservedUntil = &a.ExpiresAt
	}
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize the bounded ephemeral store, including cleanup, across API replicas.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(731924611)").Error; err != nil {
			return err
		}
		if err := tx.Where("expires_at < ? AND (login_code_reserved_until IS NULL OR login_code_reserved_until <= ?)", time.Now().UTC(), time.Now().UTC()).Delete(&registrationAttemptModel{}).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&registrationAttemptModel{}).Count(&count).Error; err != nil {
			return err
		}
		if count >= 10000 {
			return fmt.Errorf("registration capacity reached")
		}
		if a.Provider == domain.RegistrationWeChat {
			var reserved int64
			if err := tx.Model(&registrationAttemptModel{}).Where("provider=?", a.Provider).Count(&reserved).Error; err != nil {
				return err
			}
			if reserved >= 100 {
				return fmt.Errorf("registration capacity reached")
			}
		}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "login_code_hash"}}, DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrLoginCodeConflict
		}
		return nil
	})
}
func (r *RegistrationRepository) decodeAttempt(row registrationAttemptModel) (domain.RegistrationAttempt, error) {
	plain, err := r.box.Decrypt(row.PayloadCiphertext, "registration-attempt:"+row.ID)
	if err != nil {
		return domain.RegistrationAttempt{}, err
	}
	var a domain.RegistrationAttempt
	if json.Unmarshal(plain, &a) != nil {
		return a, fmt.Errorf("decode registration attempt")
	}
	a.Version = row.Version
	a.Status = row.Status
	a.ExpiresAt = row.ExpiresAt
	return a, nil
}
func (r *RegistrationRepository) Attempt(ctx context.Context, id string) (domain.RegistrationAttempt, error) {
	var row registrationAttemptModel
	if err := r.db.WithContext(ctx).Where("id=?", id).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RegistrationAttempt{}, domain.ErrNotFound
		}
		return domain.RegistrationAttempt{}, err
	}
	return r.decodeAttempt(row)
}
func (r *RegistrationRepository) AttemptByCode(ctx context.Context, hash string) (domain.RegistrationAttempt, error) {
	var row registrationAttemptModel
	if err := r.db.WithContext(ctx).Where("code_hash=?", hash).Take(&row).Error; err != nil {
		return domain.RegistrationAttempt{}, domain.ErrNotFound
	}
	return r.decodeAttempt(row)
}
func (r *RegistrationRepository) AttemptByLoginCode(ctx context.Context, hash string) (domain.RegistrationAttempt, error) {
	var row registrationAttemptModel
	if err := r.db.WithContext(ctx).Where("provider=? AND login_code_hash=?", domain.RegistrationWeChat, hash).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.RegistrationAttempt{}, domain.ErrNotFound
		}
		return domain.RegistrationAttempt{}, err
	}
	return r.decodeAttempt(row)
}
func (r *RegistrationRepository) TransitionAttempt(ctx context.Context, a domain.RegistrationAttempt, from string) error {
	expected := a.Version
	a.Version++
	row, err := r.attemptModel(a)
	if err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Model(&registrationAttemptModel{}).Where("id=? AND status=? AND version=? AND expires_at>?", a.ID, from, expected, time.Now().UTC()).Updates(map[string]any{"status": row.Status, "code_hash": row.CodeHash, "payload_ciphertext": row.PayloadCiphertext, "expires_at": row.ExpiresAt, "version": row.Version})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

// ReserveWeChatVerification bounds numeric guesses across API replicas. Only
// authenticated callback identities reach this method; identifiers stay hashed.
func (r *RegistrationRepository) ReserveWeChatVerification(ctx context.Context, senderHash string) error {
	if len(senderHash) != 64 {
		return domain.ErrUnauthenticated
	}
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(731924612)").Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM registration_verification_limits WHERE expires_at <= ?", now).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("registration_verification_limits").Count(&count).Error; err != nil {
			return err
		}
		if count >= 10000 {
			return domain.ErrRegistrationRateLimited
		}
		for _, bucket := range []struct {
			key    string
			limit  int
			window time.Duration
		}{
			{senderHash, 5, 5 * time.Minute}, {"global", 30, time.Minute},
		} {
			result := tx.Exec(`INSERT INTO registration_verification_limits (bucket_key, attempts, expires_at)
    VALUES (?, 1, ?) ON CONFLICT (bucket_key) DO UPDATE SET attempts=registration_verification_limits.attempts+1
    WHERE registration_verification_limits.attempts < ?`, bucket.key, now.Add(bucket.window), bucket.limit)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrRegistrationRateLimited
			}
		}
		return nil
	})
}
