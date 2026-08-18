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
			ID               string `gorm:"column:id"`
			Email            string `gorm:"column:email"`
			DisplayName      string `gorm:"column:display_name"`
			OrganizationID   string `gorm:"column:organization_id"`
			OrganizationSlug string `gorm:"column:organization_slug"`
			OrganizationName string `gorm:"column:organization_name"`
			Disabled         bool   `gorm:"column:disabled"`
		}
		query := `
			SELECT u.id, u.email, u.display_name, u.organization_id,
			       o.slug AS organization_slug, o.name AS organization_name,
			       (u.disabled_at IS NOT NULL) AS disabled
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
			TeamID             *string `gorm:"column:team_id"`
			TeamOrganizationID *string `gorm:"column:team_organization_id"`
			Role               string  `gorm:"column:role"`
		}
		grantQuery := `
			SELECT grant_row.team_id, grant_row.role, team.organization_id AS team_organization_id
			FROM role_grants grant_row
			LEFT JOIN teams team ON team.id = grant_row.team_id
			WHERE grant_row.organization_id = ? AND grant_row.user_id = ?
			ORDER BY grant_row.created_at, grant_row.id`
		if err := tx.Raw(grantQuery, user.OrganizationID, user.ID).Scan(&records).Error; err != nil {
			return fmt.Errorf("load User Role Grants: %w", err)
		}
		grants := make([]domain.Grant, 0, len(records))
		for _, record := range records {
			if record.TeamID != nil && (record.TeamOrganizationID == nil || *record.TeamOrganizationID != user.OrganizationID) {
				return fmt.Errorf("Role Grant Team crosses Organization boundary")
			}
			role, err := domain.ParseRole(record.Role)
			if err != nil {
				return err
			}
			grants = append(grants, domain.Grant{TeamID: record.TeamID, Role: role})
		}
		var teamRecords []struct {
			ID   string `gorm:"column:id"`
			Slug string `gorm:"column:slug"`
			Name string `gorm:"column:name"`
		}
		teamQuery := `
			SELECT t.id, t.slug, t.name
			FROM teams t
			WHERE t.organization_id = ?
			  AND (
			    EXISTS (
			      SELECT 1 FROM role_grants organization_grant
			      WHERE organization_grant.organization_id = t.organization_id
			        AND organization_grant.user_id = ?
			        AND organization_grant.team_id IS NULL
			    )
			    OR EXISTS (
			      SELECT 1 FROM role_grants team_grant
			      WHERE team_grant.organization_id = t.organization_id
			        AND team_grant.user_id = ?
			        AND team_grant.team_id = t.id
			    )
			  )
			ORDER BY t.name, t.id`
		if err := tx.Raw(teamQuery, user.OrganizationID, user.ID, user.ID).Scan(&teamRecords).Error; err != nil {
			return fmt.Errorf("load accessible Teams: %w", err)
		}
		teams := make([]domain.Team, 0, len(teamRecords))
		for _, record := range teamRecords {
			teams = append(teams, domain.Team{ID: record.ID, Slug: record.Slug, Name: record.Name})
		}
		principal = domain.Principal{
			UserID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
			OrganizationID: user.OrganizationID, OrganizationSlug: user.OrganizationSlug, OrganizationName: user.OrganizationName,
			Disabled: user.Disabled, Grants: grants, Teams: teams,
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
