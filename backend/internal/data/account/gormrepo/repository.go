package gormrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/account/application"
	"agent-platform/backend/internal/biz/account/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct{ db *gorm.DB }

var _ application.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type userModel struct {
	ID            string     `gorm:"column:id"`
	OIDCSubject   string     `gorm:"column:oidc_subject"`
	Username      string     `gorm:"column:username"`
	Email         string     `gorm:"column:email"`
	DisplayName   string     `gorm:"column:display_name"`
	Administrator bool       `gorm:"column:administrator"`
	DisabledAt    *time.Time `gorm:"column:disabled_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
	Version       int64      `gorm:"column:version"`
}

func (userModel) TableName() string { return "users" }

func (repository *Repository) FindPrincipal(ctx context.Context, subject string) (domain.Principal, error) {
	var model userModel
	if err := repository.db.WithContext(ctx).Where("oidc_subject = ?", subject).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Principal{}, domain.ErrNotFound
		}
		return domain.Principal{}, fmt.Errorf("find User identity: %w", err)
	}
	return domain.Principal{
		UserID: model.ID, Username: model.Username, Email: model.Email, DisplayName: model.DisplayName,
		Administrator: model.Administrator, Disabled: model.DisabledAt != nil,
	}, nil
}

func (repository *Repository) EnsureAdministrator(ctx context.Context, user domain.User) (domain.User, error) {
	var row userModel
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("administrator = true").Take(&row).Error; err == nil {
			if row.OIDCSubject != user.OIDCSubject {
				return fmt.Errorf("a different bootstrap Administrator already exists")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = newUserModel(user)
		row.Administrator = true
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO personal_settings (user_id) VALUES (?)", row.ID).Error
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("ensure bootstrap Administrator: %w", err)
	}
	return toDomain(row), nil
}

func (repository *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	var models []userModel
	if err := repository.db.WithContext(ctx).Order("administrator DESC, created_at, id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list Users: %w", err)
	}
	users := make([]domain.User, 0, len(models))
	for _, model := range models {
		users = append(users, toDomain(model))
	}
	return users, nil
}

func (repository *Repository) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	model := newUserModel(user)
	model.Administrator = false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create User: %w", err)
		}
		if err := tx.Exec("INSERT INTO personal_settings (user_id) VALUES (?)", model.ID).Error; err != nil {
			return fmt.Errorf("create Personal Settings: %w", err)
		}
		return nil
	})
	return toDomain(model), err
}

func newUserModel(user domain.User) userModel {
	return userModel{
		ID: uuid.NewString(), OIDCSubject: user.OIDCSubject, Username: user.Username,
		Email: user.Email, DisplayName: user.DisplayName, Administrator: user.Administrator, Version: 1,
	}
}

func (repository *Repository) SetEnabled(ctx context.Context, userID string, enabled bool, expectedVersion int64) (domain.User, error) {
	var disabledAt any = gorm.Expr("now()")
	if enabled {
		disabledAt = nil
	}
	result := repository.db.WithContext(ctx).Model(&userModel{}).
		Where("id = ? AND version = ? AND administrator = false", userID, expectedVersion).
		Updates(map[string]any{"disabled_at": disabledAt, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return domain.User{}, fmt.Errorf("update User status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.User{}, domain.ErrConflict
	}
	var model userModel
	if err := repository.db.WithContext(ctx).Where("id = ?", userID).Take(&model).Error; err != nil {
		return domain.User{}, fmt.Errorf("reload User: %w", err)
	}
	return toDomain(model), nil
}

func toDomain(model userModel) domain.User {
	return domain.User{
		ID: model.ID, OIDCSubject: model.OIDCSubject, Username: model.Username, Email: model.Email,
		DisplayName: model.DisplayName, Administrator: model.Administrator, Enabled: model.DisabledAt == nil,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt, Version: model.Version,
	}
}
