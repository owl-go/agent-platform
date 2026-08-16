package gormrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"agent-platform/backend/internal/biz/identity/application"
	"agent-platform/backend/internal/biz/identity/domain"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var _ application.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) FindPrincipal(ctx context.Context, identity domain.VerifiedIdentity) (domain.Principal, error) {
	var principal domain.Principal
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user struct {
			ID             string `gorm:"column:id"`
			OrganizationID string `gorm:"column:organization_id"`
			Disabled       bool   `gorm:"column:disabled"`
		}
		query := `
			SELECT u.id, u.organization_id, (u.disabled_at IS NOT NULL) AS disabled
			FROM users u
			JOIN organizations o ON o.id = u.organization_id
			WHERE o.slug = ? AND u.oidc_subject = ?`
		result := tx.Raw(query, identity.OrganizationSlug, identity.Subject).Scan(&user)
		if result.Error != nil {
			return fmt.Errorf("load User identity: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrUserNotFound
		}

		var records []struct {
			TeamID *string `gorm:"column:team_id"`
			Role   string  `gorm:"column:role"`
		}
		if err := tx.Raw(`SELECT team_id, role FROM role_grants WHERE organization_id = ? AND user_id = ? ORDER BY created_at, id`, user.OrganizationID, user.ID).Scan(&records).Error; err != nil {
			return fmt.Errorf("load User Role Grants: %w", err)
		}
		grants := make([]domain.Grant, 0, len(records))
		for _, record := range records {
			role, err := domain.ParseRole(record.Role)
			if err != nil {
				return err
			}
			grants = append(grants, domain.Grant{TeamID: record.TeamID, Role: role})
		}
		principal = domain.Principal{
			UserID: user.ID, OrganizationID: user.OrganizationID, Disabled: user.Disabled, Grants: grants,
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return principal, err
}

func (repository *Repository) FindRunScope(ctx context.Context, runID string) (domain.RunScope, error) {
	var scope domain.RunScope
	query := `
		SELECT task.organization_id, task.team_id
		FROM runs run
		JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id
		WHERE run.id = ?`
	result := repository.db.WithContext(ctx).Raw(query, runID).Scan(&scope)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.RunScope{}, domain.ErrRunNotFound
		}
		return domain.RunScope{}, fmt.Errorf("load Run authorization scope: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.RunScope{}, domain.ErrRunNotFound
	}
	return scope, nil
}
