package gormrepo

import (
	"context"
	"time"

	"agent-platform/backend/internal/biz/retention/domain"

	"gorm.io/gorm"
)

type Repository struct{ database *gorm.DB }

func New(database *gorm.DB) *Repository { return &Repository{database: database} }

func (repository *Repository) ListExpiredArtifacts(ctx context.Context, now, defaultCutoff, workspaceCutoff time.Time, limit int) ([]domain.Artifact, error) {
	var artifacts []domain.Artifact
	err := repository.database.WithContext(ctx).Raw(`
SELECT id::text AS id, object_key
FROM artifacts artifact
JOIN runs run ON run.id = artifact.run_id
JOIN sessions session ON session.id = run.session_id
JOIN coding_tasks task ON task.id = session.coding_task_id
WHERE artifact.deleted_at IS NULL
  AND (
    (artifact.kind = 'workspace_snapshot' AND task.completed_at IS NOT NULL AND task.completed_at < ?)
    OR (artifact.kind <> 'workspace_snapshot' AND ((artifact.expires_at IS NOT NULL AND artifact.expires_at <= ?) OR (artifact.expires_at IS NULL AND artifact.created_at < ?)))
  )
ORDER BY artifact.created_at, artifact.id
LIMIT ?`, workspaceCutoff, now, defaultCutoff, limit).Scan(&artifacts).Error
	return artifacts, err
}

func (repository *Repository) ListExpiredWorkspaces(ctx context.Context, cutoff time.Time, limit int) ([]domain.Workspace, error) {
	var workspaces []domain.Workspace
	err := repository.database.WithContext(ctx).Raw(`
SELECT session.id::text AS session_id, session.workspace_volume AS volume
FROM sessions session
JOIN coding_tasks task ON task.id = session.coding_task_id
WHERE session.workspace_purged_at IS NULL AND task.completed_at IS NOT NULL AND task.completed_at < ?
ORDER BY task.completed_at, session.id
LIMIT ?`, cutoff, limit).Scan(&workspaces).Error
	return workspaces, err
}

func (repository *Repository) MarkWorkspacePurged(ctx context.Context, sessionID string, now time.Time) (bool, error) {
	result := repository.database.WithContext(ctx).Table("sessions").Where("id = ? AND workspace_purged_at IS NULL", sessionID).Updates(map[string]any{
		"workspace_purged_at": now.UTC(), "updated_at": now.UTC(), "version": gorm.Expr("version + 1"),
	})
	return result.RowsAffected == 1, result.Error
}

func (repository *Repository) MarkArtifactDeleted(ctx context.Context, id string, now time.Time) (bool, error) {
	result := repository.database.WithContext(ctx).Exec(`UPDATE artifacts SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, now, id)
	return result.RowsAffected == 1, result.Error
}

func (repository *Repository) DeleteRunEvents(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return repository.deleteBatch(ctx, `run_events`, `created_at`, cutoff, limit)
}

func (repository *Repository) DeleteAuditEvents(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	return repository.deleteBatch(ctx, `audit_events`, `created_at`, cutoff, limit)
}

func (repository *Repository) DeleteIdempotencyKeys(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	result := repository.database.WithContext(ctx).Exec(`
DELETE FROM idempotency_keys
WHERE ctid IN (SELECT ctid FROM idempotency_keys WHERE expires_at < ? ORDER BY expires_at LIMIT ?)`, cutoff, limit)
	return result.RowsAffected, result.Error
}

func (repository *Repository) deleteBatch(ctx context.Context, table, timestampColumn string, cutoff time.Time, limit int) (int64, error) {
	result := repository.database.WithContext(ctx).Exec(
		`DELETE FROM `+table+` WHERE id IN (SELECT id FROM `+table+` WHERE `+timestampColumn+` < ? ORDER BY `+timestampColumn+`, id LIMIT ?)`,
		cutoff, limit,
	)
	return result.RowsAffected, result.Error
}
