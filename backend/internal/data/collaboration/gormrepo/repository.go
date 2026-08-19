package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/collaboration/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

var _ domain.Repository = (*Repository)(nil)
var _ domain.CompletionProjector = (*Repository)(nil)

func (repository *Repository) ProjectCompletedRun(ctx context.Context, runID, sessionID string, now time.Time) error {
	var row struct {
		TaskID       string `gorm:"column:task_id"`
		ReviewBranch string `gorm:"column:review_branch"`
		State        string `gorm:"column:state"`
		Version      int64  `gorm:"column:version"`
	}
	query := `
		SELECT task.id AS task_id, session.review_branch, task.state, task.version
		FROM sessions session
		JOIN coding_tasks task ON task.id = session.coding_task_id
		WHERE session.id = ?
		FOR UPDATE OF task`
	result := repository.db.WithContext(ctx).Raw(query, sessionID).Scan(&row)
	if result.Error != nil {
		return fmt.Errorf("load completed Run Coding Task: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("completed Run Session has no Coding Task")
	}
	if row.State != string(domain.TaskStateActive) {
		return nil
	}
	update := repository.db.WithContext(ctx).Table("coding_tasks").Where("id = ? AND version = ?", row.TaskID, row.Version).Updates(map[string]any{
		"state": domain.TaskStateWaitingForUser, "updated_at": now.UTC(), "version": row.Version + 1,
	})
	if update.Error != nil {
		return fmt.Errorf("mark Coding Task waiting for user: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	content, err := json.Marshal(map[string]any{
		"type": "run_result", "status": "completed", "run_id": runID, "review_branch": row.ReviewBranch,
	})
	if err != nil {
		return err
	}
	if err := repository.db.WithContext(ctx).Table("session_messages").Create(map[string]any{
		"session_id": sessionID, "run_id": runID, "author_type": domain.MessageAuthorAgent,
		"content": jsonValue(content), "created_at": now.UTC(),
	}).Error; err != nil {
		return fmt.Errorf("append completed Run Session Message: %w", err)
	}
	return nil
}

func (repository *Repository) CreateLaunchOwned(ctx context.Context, registration domain.LaunchRegistration) (domain.Launch, domain.QueuedRunPlan, error) {
	db := repository.db.WithContext(ctx)
	selection, err := loadLaunchSelection(db, registration.Task.OrganizationID, registration.Task.TeamID, registration.Task.AgentReleaseID)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	task := registration.Task
	if err := task.Activate(registration.Now); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	session, err := domain.OpenSession(domain.SessionRegistration{
		ID: registration.SessionID, CodingTaskID: task.ID, RepositoryBindingID: selection.RepositoryBindingID,
		TargetBranch: selection.TargetBranch, ReviewBranch: registration.ReviewBranch,
		WorkspaceVolume: registration.WorkspaceVolume, Now: registration.Now,
	})
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if err := session.AddRun(registration.Now); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	plan, err := repository.createLaunchRecords(ctx, db, task, session, registration.Run, selection, registration.Now)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	return domain.Launch{Task: task, Session: session, RunID: registration.Run.ID}, plan, nil
}

func (repository *Repository) ContinueOwned(ctx context.Context, registration domain.ContinueRegistration) (domain.Launch, domain.QueuedRunPlan, error) {
	tx := repository.db.WithContext(ctx)
	var taskRow taskRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ? AND team_id = ? AND id = ?", registration.OrganizationID, registration.TeamID, registration.TaskID).
		Take(&taskRow).Error; err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, mapTaskError(err)
	}
	task, err := restoreTask(taskRow)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	var sessionRow sessionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("coding_task_id = ?", task.ID).Take(&sessionRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Launch{}, domain.QueuedRunPlan{}, domain.ErrSessionNotFound
		}
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	session, err := restoreSession(sessionRow)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if task.Version != registration.ExpectedTaskVersion || session.Version != registration.ExpectedSessionVersion {
		return domain.Launch{}, domain.QueuedRunPlan{}, domain.ErrConcurrentUpdate
	}
	if task.State != domain.TaskStateWaitingForUser {
		return domain.Launch{}, domain.QueuedRunPlan{}, domain.ErrTaskStateConflict
	}
	selection, err := loadLaunchSelection(tx, task.OrganizationID, task.TeamID, task.AgentReleaseID)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if selection.RepositoryBindingID != session.RepositoryBindingID {
		return domain.Launch{}, domain.QueuedRunPlan{}, fmt.Errorf("Agent Release no longer matches Session Repository Binding")
	}
	if err := task.Activate(registration.Now); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if err := session.AddRun(registration.Now); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if err := updateTaskRow(tx, task, registration.ExpectedTaskVersion); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	if err := updateSessionRow(tx, session, registration.ExpectedSessionVersion); err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	plan, err := repository.createRunPlanAndMessage(tx, task, session, registration.Run, selection, registration.Now)
	if err != nil {
		return domain.Launch{}, domain.QueuedRunPlan{}, err
	}
	return domain.Launch{Task: task, Session: session, RunID: registration.Run.ID}, plan, nil
}

func loadLaunchSelection(tx *gorm.DB, organizationID, teamID, releaseID string) (launchSelection, error) {
	var selection launchSelection
	query := `
		SELECT ar.agent_id, ar.repository_binding_id, rb.default_branch AS target_branch,
		       ar.runtime_image_id, cm.model_id, model_credential.secret_ref AS model_secret_ref,
		       git_credential.secret_ref AS git_secret_ref,
		       COALESCE((
		           SELECT jsonb_agg(jsonb_build_object('ref', build_credential.secret_ref, 'purpose', 'build') ORDER BY credential_id)
		           FROM jsonb_array_elements_text(rb.build_credential_profile_ids) AS credential_id
		           JOIN credential_profiles build_credential ON build_credential.id = credential_id::uuid
		           WHERE build_credential.disabled_at IS NULL
		       ), '[]'::jsonb) AS build_credential_bindings,
		       ar.model_budget, ar.execution_limits
		FROM agent_releases ar
		JOIN agents a ON a.id = ar.agent_id
		JOIN repository_bindings rb ON rb.id = ar.repository_binding_id
		JOIN runtime_images ri ON ri.id = ar.runtime_image_id
		JOIN configured_models cm ON cm.id = ar.configured_model_id
		JOIN credential_profiles model_credential ON model_credential.id = cm.credential_profile_id
		JOIN credential_profiles git_credential ON git_credential.id = rb.ssh_credential_profile_id
		WHERE ar.id = ? AND a.organization_id = ? AND a.team_id = ?
		  AND rb.organization_id = ? AND rb.team_id = ? AND ri.organization_id = a.organization_id
		  AND ar.status = 'released' AND ri.status = 'production' AND cm.enabled = true
		  AND model_credential.disabled_at IS NULL AND git_credential.disabled_at IS NULL
		  AND rb.validation_report IS NOT NULL AND (rb.validation_report->>'valid')::boolean = true
		FOR SHARE OF ar, a, rb, ri, cm, model_credential, git_credential`
	if err := tx.Raw(query, releaseID, organizationID, teamID, organizationID, teamID).Scan(&selection).Error; err != nil {
		return launchSelection{}, fmt.Errorf("resolve Coding Task launch dependencies: %w", err)
	}
	if selection.AgentID == "" {
		return launchSelection{}, domain.ErrReleaseUnavailable
	}
	return selection, nil
}

func (repository *Repository) createLaunchRecords(ctx context.Context, tx *gorm.DB, task domain.Task, session domain.Session, seed domain.RunSeed, selection launchSelection, now time.Time) (domain.QueuedRunPlan, error) {
	taskRow, err := taskToRecord(task)
	if err != nil {
		return domain.QueuedRunPlan{}, err
	}
	if err := tx.Create(&taskRow).Error; err != nil {
		return domain.QueuedRunPlan{}, fmt.Errorf("create Coding Task: %w", err)
	}
	sessionRow, err := sessionToRecord(session)
	if err != nil {
		return domain.QueuedRunPlan{}, err
	}
	if err := tx.Create(&sessionRow).Error; err != nil {
		return domain.QueuedRunPlan{}, fmt.Errorf("create Session: %w", err)
	}
	return repository.createRunPlanAndMessage(tx.WithContext(ctx), task, session, seed, selection, now)
}

func (repository *Repository) createRunPlanAndMessage(tx *gorm.DB, task domain.Task, session domain.Session, seed domain.RunSeed, selection launchSelection, now time.Time) (domain.QueuedRunPlan, error) {
	if seed.ID == "" || seed.CreatedBy == "" || seed.RequestText == "" {
		return domain.QueuedRunPlan{}, fmt.Errorf("Run identity, creator, and request are required")
	}
	modelBinding, err := json.Marshal(map[string]string{"model_id": selection.ModelID})
	if err != nil {
		return domain.QueuedRunPlan{}, err
	}
	var buildBindings []map[string]string
	if len(selection.BuildCredentialBindings) > 0 {
		if err := json.Unmarshal(selection.BuildCredentialBindings, &buildBindings); err != nil {
			return domain.QueuedRunPlan{}, fmt.Errorf("decode build Credential Bindings: %w", err)
		}
	}
	credentials := []map[string]string{
		{"ref": selection.ModelSecretRef, "purpose": "model"},
		{"ref": selection.GitSecretRef, "purpose": "git_ssh"},
	}
	credentials = append(credentials, buildBindings...)
	credentialBindings, err := json.Marshal(credentials)
	if err != nil {
		return domain.QueuedRunPlan{}, err
	}
	plan := domain.QueuedRunPlan{
		ID: seed.ID, SessionID: session.ID, CodingTaskID: task.ID, AgentReleaseID: task.AgentReleaseID,
		RuntimeImageID: selection.RuntimeImageID, RequestText: seed.RequestText,
		ModelBinding: modelBinding, CredentialBindings: credentialBindings,
		ModelBudget: append(json.RawMessage(nil), selection.ModelBudget...), ExecutionLimits: append(json.RawMessage(nil), selection.ExecutionLimits...),
		CreatedBy: seed.CreatedBy, CreatedAt: now,
	}
	content, err := json.Marshal(map[string]string{"text": seed.RequestText})
	if err != nil {
		return domain.QueuedRunPlan{}, err
	}
	runID, userID := seed.ID, seed.CreatedBy
	message := messageRecord{SessionID: session.ID, RunID: &runID, AuthorType: domain.MessageAuthorUser, AuthorUserID: &userID, Content: content, CreatedAt: now.UTC()}
	if err := tx.Create(&message).Error; err != nil {
		return domain.QueuedRunPlan{}, fmt.Errorf("create Session Message: %w", err)
	}
	return plan, nil
}

func (repository *Repository) GetTask(ctx context.Context, organizationID, teamID, id string) (domain.Task, error) {
	var record taskRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ? AND id = ?", organizationID, teamID, id).Take(&record).Error; err != nil {
		return domain.Task{}, mapTaskError(err)
	}
	return restoreTask(record)
}

func (repository *Repository) ListTasks(ctx context.Context, organizationID, teamID string) ([]domain.Task, error) {
	var records []taskRecord
	if err := repository.db.WithContext(ctx).Where("organization_id = ? AND team_id = ?", organizationID, teamID).Order("created_at DESC, id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Coding Tasks: %w", err)
	}
	result := make([]domain.Task, 0, len(records))
	for _, record := range records {
		task, err := restoreTask(record)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, nil
}

func (repository *Repository) UpdateTask(ctx context.Context, task domain.Task, expectedVersion int64) error {
	return updateTaskRow(repository.db.WithContext(ctx), task, expectedVersion)
}

func updateTaskRow(db *gorm.DB, task domain.Task, expectedVersion int64) error {
	result := db.Model(&taskRecord{}).Where("id = ? AND version = ?", task.ID, expectedVersion).Updates(map[string]any{
		"state": task.State, "updated_at": task.UpdatedAt, "completed_at": task.CompletedAt, "version": task.Version,
	})
	if result.Error != nil {
		return fmt.Errorf("update Coding Task: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func (repository *Repository) GetSessionByTask(ctx context.Context, taskID string) (domain.Session, error) {
	var record sessionRecord
	if err := repository.db.WithContext(ctx).Where("coding_task_id = ?", taskID).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Session{}, domain.ErrSessionNotFound
		}
		return domain.Session{}, fmt.Errorf("load Session: %w", err)
	}
	return restoreSession(record)
}

func (repository *Repository) UpdateSession(ctx context.Context, session domain.Session, expectedVersion int64) error {
	return updateSessionRow(repository.db.WithContext(ctx), session, expectedVersion)
}

func updateSessionRow(db *gorm.DB, session domain.Session, expectedVersion int64) error {
	memory, err := json.Marshal(session.Memory)
	if err != nil {
		return err
	}
	result := db.Model(&sessionRecord{}).Where("id = ? AND version = ?", session.ID, expectedVersion).Updates(map[string]any{
		"session_memory": jsonValue(memory), "run_count": session.RunCount,
		"updated_at": session.UpdatedAt, "version": session.Version,
	})
	if result.Error != nil {
		return fmt.Errorf("update Session: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentUpdate
	}
	return nil
}

func (repository *Repository) ListMessages(ctx context.Context, sessionID string, afterID int64, limit int) ([]domain.Message, error) {
	var records []messageRecord
	if err := repository.db.WithContext(ctx).Where("session_id = ? AND id > ?", sessionID, afterID).Order("id").Limit(limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list Session Messages: %w", err)
	}
	result := make([]domain.Message, 0, len(records))
	for _, record := range records {
		message := domain.Message{ID: record.ID, SessionID: record.SessionID, Author: record.AuthorType, Content: append(json.RawMessage(nil), record.Content...), CreatedAt: record.CreatedAt}
		if record.RunID != nil {
			message.RunID = *record.RunID
		}
		if record.AuthorUserID != nil {
			message.AuthorUserID = *record.AuthorUserID
		}
		result = append(result, message)
	}
	return result, nil
}

func mapTaskError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrTaskNotFound
	}
	return fmt.Errorf("load Coding Task: %w", err)
}
