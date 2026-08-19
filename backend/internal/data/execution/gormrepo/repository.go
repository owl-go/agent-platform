package gormrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const reconcileBatchSize = 50

type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)
var _ domain.ApprovalCommands = (*Repository)(nil)
var _ domain.LaunchCommands = (*Repository)(nil)

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (repository *Repository) CreateQueuedRun(ctx context.Context, queued domain.QueuedRun) error {
	if queued.ID == "" || queued.SessionID == "" || queued.AgentReleaseID == "" || queued.RuntimeImageID == "" || queued.RequestText == "" || queued.CreatedBy == "" {
		return fmt.Errorf("queued Run identity, frozen dependencies, request, and creator are required")
	}
	record := runRecord{
		ID: queued.ID, SessionID: queued.SessionID, AgentReleaseID: queued.AgentReleaseID,
		RuntimeImageID: queued.RuntimeImageID, RequestText: queued.RequestText, State: string(domain.Queued),
		ModelBinding: jsonValue(queued.ModelBinding), CredentialBindings: jsonValue(queued.CredentialBindings),
		ModelBudget: jsonValue(queued.ModelBudget), ExecutionLimits: jsonValue(queued.ExecutionLimits),
		CreatedBy: queued.CreatedBy, CreatedAt: queued.CreatedAt.UTC(), UpdatedAt: queued.CreatedAt.UTC(),
	}
	if err := repository.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create queued Run: %w", err)
	}
	return appendEvent(repository.db.WithContext(ctx), queued.ID, "run.queued", map[string]any{
		"state": "queued", "coding_task_id": queued.CodingTaskID,
	}, queued.CreatedAt)
}

func (repository *Repository) PauseForApproval(ctx context.Context, runID string, expectedVersion int64, approvalID, kind string, now time.Time) error {
	var run runRecord
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runID).Take(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRunNotFound
		}
		return fmt.Errorf("lock Run for Approval: %w", err)
	}
	if run.Version != expectedVersion {
		return domain.ErrConcurrentModification
	}
	if run.State != string(domain.Running) {
		return domain.ErrApprovalRunState
	}
	update := repository.db.WithContext(ctx).Model(&runRecord{}).Where("id = ? AND version = ?", run.ID, run.Version).Updates(map[string]any{
		"state": string(domain.WaitingConfirmation), "updated_at": now.UTC(), "version": run.Version + 1,
	})
	if update.Error != nil {
		return fmt.Errorf("pause Run for Approval: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return domain.ErrConcurrentModification
	}
	return appendEvent(repository.db.WithContext(ctx), runID, "approval.requested", map[string]any{"approval_id": approvalID, "kind": kind}, now)
}

func (repository *Repository) ApplyApprovalDecision(ctx context.Context, decision domain.ApprovalDecision, now time.Time) error {
	var run runRecord
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", decision.RunID).Take(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrRunNotFound
		}
		return fmt.Errorf("lock approved Run: %w", err)
	}
	if run.State != string(domain.WaitingConfirmation) {
		return domain.ErrApprovalRunState
	}
	runUpdates := map[string]any{"updated_at": now.UTC(), "version": run.Version + 1}
	eventType := "approval.approved"
	if decision.Approved {
		runUpdates["state"] = string(domain.Running)
	} else {
		eventType = "approval.rejected"
		runUpdates["state"] = string(domain.Cancelled)
		runUpdates["ended_at"] = now.UTC()
		runUpdates["terminal_error"] = jsonValue(`{"code":"approval_rejected","message":"A requested high-risk operation was rejected"}`)
	}
	update := repository.db.WithContext(ctx).Model(&runRecord{}).Where("id = ? AND version = ?", run.ID, run.Version).Updates(runUpdates)
	if update.Error != nil {
		return fmt.Errorf("apply Run Approval decision: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return domain.ErrConcurrentModification
	}
	if !decision.Approved {
		if err := repository.db.WithContext(ctx).Model(&attemptRecord{}).Where("run_id = ? AND state IN ?", run.ID, []string{"provisioning", "running"}).Updates(map[string]any{"state": "cancelled", "ended_at": now.UTC(), "error": runUpdates["terminal_error"]}).Error; err != nil {
			return fmt.Errorf("cancel rejected Run Attempt: %w", err)
		}
		if err := repository.db.WithContext(ctx).Where("run_id = ?", run.ID).Delete(&leaseRecord{}).Error; err != nil {
			return fmt.Errorf("release rejected Run lease: %w", err)
		}
		if err := repository.db.WithContext(ctx).Where("run_id = ?", run.ID).Delete(&workspaceLeaseRecord{}).Error; err != nil {
			return fmt.Errorf("release rejected Workspace lease: %w", err)
		}
	}
	return appendEvent(repository.db.WithContext(ctx), run.ID, eventType, map[string]any{
		"approval_id": decision.ApprovalID, "actor_user_id": decision.ActorUserID, "reason": decision.Reason,
	}, now)
}

func (repository *Repository) Get(ctx context.Context, runID string) (domain.Details, error) {
	var details domain.Details
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record runRecord
		if err := tx.Where("id = ?", runID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRunNotFound
			}
			return fmt.Errorf("load Run: %w", err)
		}
		state, err := domain.ParseState(record.State)
		if err != nil {
			return err
		}
		var attemptRecords []attemptRecord
		if err := tx.Where("run_id = ?", runID).Order("attempt_number").Find(&attemptRecords).Error; err != nil {
			return fmt.Errorf("list Run Attempts: %w", err)
		}
		attempts := make([]domain.Attempt, 0, len(attemptRecords))
		for _, record := range attemptRecords {
			attemptState, err := domain.ParseAttemptState(record.State)
			if err != nil {
				return err
			}
			attempts = append(attempts, domain.Attempt{
				ID: record.ID, Number: record.AttemptNumber, WorkerID: record.WorkerID, State: attemptState,
				InfrastructureFailure: record.InfrastructureFailure, Error: cloneJSON(record.Error),
				StartedAt: record.StartedAt, EndedAt: record.EndedAt,
			})
		}
		details = domain.Details{
			ID: record.ID, SessionID: record.SessionID, AgentReleaseID: record.AgentReleaseID,
			RuntimeImageID: record.RuntimeImageID, RequestText: record.RequestText, State: state,
			ModelBinding: cloneJSON(record.ModelBinding), ModelBudget: cloneJSON(record.ModelBudget),
			ExecutionLimits: cloneJSON(record.ExecutionLimits), Usage: cloneJSON(record.Usage),
			Cost: record.CostAmount, TerminalError: cloneJSON(record.TerminalError), AttemptCount: record.AttemptCount,
			CreatedBy: record.CreatedBy, CreatedAt: record.CreatedAt, StartedAt: record.StartedAt,
			EndedAt: record.EndedAt, UpdatedAt: record.UpdatedAt, Version: record.Version, Attempts: attempts,
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return details, err
}

func (repository *Repository) Search(ctx context.Context, query domain.SearchQuery) ([]domain.Details, error) {
	database := repository.db.WithContext(ctx).Model(&runRecord{}).
		Select("runs.*").
		Joins("JOIN sessions ON sessions.id = runs.session_id").
		Joins("JOIN coding_tasks ON coding_tasks.id = sessions.coding_task_id").
		Joins("JOIN agent_releases ON agent_releases.id = runs.agent_release_id").
		Joins("JOIN runtime_images ON runtime_images.id = runs.runtime_image_id").
		Where("coding_tasks.organization_id = ? AND coding_tasks.team_id = ? AND runtime_images.organization_id = coding_tasks.organization_id", query.OrganizationID, query.TeamID)
	if query.AgentID != "" {
		database = database.Where("agent_releases.agent_id = ?", query.AgentID)
	}
	if query.RepositoryBindingID != "" {
		database = database.Where("sessions.repository_binding_id = ?", query.RepositoryBindingID)
	}
	if query.TaskID != "" {
		database = database.Where("sessions.coding_task_id = ?", query.TaskID)
	}
	if query.State != "" {
		database = database.Where("runs.state = ?", query.State)
	}
	if query.Runtime != "" {
		database = database.Where("runtime_images.runtime = ?", query.Runtime)
	}
	if query.CreatedFrom != nil {
		database = database.Where("runs.created_at >= ?", query.CreatedFrom.UTC())
	}
	if query.CreatedTo != nil {
		database = database.Where("runs.created_at <= ?", query.CreatedTo.UTC())
	}
	var records []runRecord
	if err := database.Order("runs.created_at DESC, runs.id DESC").Limit(query.Limit).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("search Runs: %w", err)
	}
	results := make([]domain.Details, 0, len(records))
	for _, record := range records {
		details, err := repository.Get(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		results = append(results, details)
	}
	return results, nil
}

func (repository *Repository) Claim(ctx context.Context, workerID string, duration time.Duration, now time.Time) (domain.Lease, bool, error) {
	var claimed domain.Lease
	found := false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record runRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("state IN ?", []string{string(domain.Queued), string(domain.Resuming)}).
			Where("NOT EXISTS (SELECT 1 FROM run_leases WHERE run_leases.run_id = runs.id)").
			Where("NOT EXISTS (SELECT 1 FROM workspace_write_leases WHERE workspace_write_leases.session_id = runs.session_id AND workspace_write_leases.expires_at > ?)", now.UTC()).
			Order("created_at, id").
			Take(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim queued Run: %w", err)
		}
		var session sessionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", record.SessionID).Take(&session).Error; err != nil {
			return fmt.Errorf("lock claimed Run Session: %w", err)
		}
		var activeWorkspaceLeases int64
		if err := tx.Model(&workspaceLeaseRecord{}).
			Where("session_id = ? AND expires_at > ?", record.SessionID, now.UTC()).
			Count(&activeWorkspaceLeases).Error; err != nil {
			return fmt.Errorf("check Workspace Write Lease: %w", err)
		}
		if activeWorkspaceLeases > 0 {
			return nil
		}
		run, err := restore(record)
		if err != nil {
			return err
		}
		attemptNumber, err := run.Claim(now)
		if err != nil {
			return err
		}
		if err := updateRun(tx, run, now, nil); err != nil {
			return fmt.Errorf("persist claimed Run: %w", err)
		}

		attemptID := uuid.NewString()
		attempt := attemptRecord{
			ID: attemptID, RunID: run.ID, AttemptNumber: attemptNumber,
			WorkerID: workerID, State: string(domain.Provisioning), StartedAt: now.UTC(),
		}
		if err := tx.Create(&attempt).Error; err != nil {
			return fmt.Errorf("create Run Attempt: %w", err)
		}
		expiresAt := now.UTC().Add(duration)
		leaseToken := uuid.NewString()
		lease := leaseRecord{
			RunID: run.ID, AttemptID: attemptID, WorkerID: workerID,
			LeaseToken: leaseToken, ExpiresAt: expiresAt, UpdatedAt: now.UTC(),
		}
		if err := tx.Create(&lease).Error; err != nil {
			return fmt.Errorf("create Run lease: %w", err)
		}
		workspaceLease := workspaceLeaseRecord{
			SessionID: run.SessionID, RunID: run.ID, LeaseToken: leaseToken,
			ExpiresAt: expiresAt, UpdatedAt: now.UTC(),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "session_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"run_id", "lease_token", "expires_at", "updated_at"}),
		}).Create(&workspaceLease).Error; err != nil {
			return fmt.Errorf("create Workspace Write Lease: %w", err)
		}
		if err := appendEvent(tx, run.ID, "run.attempt_started", map[string]any{
			"attempt_id": attemptID, "attempt_number": attemptNumber, "worker_id": workerID,
		}, now); err != nil {
			return err
		}
		var runtimeImage runtimeImageRecord
		if err := tx.Where("id = ?", record.RuntimeImageID).Take(&runtimeImage).Error; err != nil {
			return fmt.Errorf("load claimed Run Runtime Image: %w", err)
		}
		var binding repositoryBindingRecord
		if err := tx.Where("id = ?", session.RepositoryBindingID).Take(&binding).Error; err != nil {
			return fmt.Errorf("load claimed Run Repository Binding: %w", err)
		}
		claimed = domain.Lease{
			RunID: run.ID, SessionID: run.SessionID, AttemptID: attemptID, AttemptNumber: attemptNumber,
			Token: lease.LeaseToken, RequestText: record.RequestText,
			ModelBinding: cloneJSON(record.ModelBinding), CredentialBindings: cloneJSON(record.CredentialBindings),
			ModelBudget: cloneJSON(record.ModelBudget), ExecutionLimits: cloneJSON(record.ExecutionLimits), ExpiresAt: expiresAt,
			RuntimeName: runtimeImage.Runtime, RuntimeCLIVersion: runtimeImage.CLIVersion,
			AdapterVersion: runtimeImage.AdapterVersion, ImageDigest: runtimeImage.ImageDigest,
			Capabilities: cloneJSON(runtimeImage.Capabilities), WorkspaceVolume: session.WorkspaceVolume,
			RepositorySSHURL: binding.RepositorySSHURL, TargetBranch: session.TargetBranch, ReviewBranch: session.ReviewBranch,
			GitAuthorName: binding.GitAuthorName, GitAuthorEmail: binding.GitAuthorEmail,
			QualityCommands: cloneJSON(binding.QualityCommands),
		}
		found = true
		return nil
	})
	if err != nil {
		return domain.Lease{}, false, err
	}
	return claimed, found, nil
}

func (repository *Repository) Renew(ctx context.Context, token string, duration time.Duration, now time.Time) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var leaseState struct {
			State string `gorm:"column:state"`
		}
		result := tx.Raw(`
			SELECT run.state
			FROM run_leases lease
			JOIN runs run ON run.id = lease.run_id
			WHERE lease.lease_token = ? AND lease.expires_at > ?
			FOR UPDATE OF lease, run`, token, now.UTC()).Scan(&leaseState)
		if result.Error != nil {
			return fmt.Errorf("inspect Run lease for renewal: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrLeaseLost
		}
		if leaseState.State == string(domain.Interrupting) {
			return domain.ErrInterruptionRequested
		}
		if leaseState.State != string(domain.Provisioning) && leaseState.State != string(domain.Running) && leaseState.State != string(domain.WaitingConfirmation) {
			return domain.ErrLeaseLost
		}
		expiresAt := now.UTC().Add(duration)
		result = tx.Model(&leaseRecord{}).
			Where("lease_token = ? AND expires_at > ?", token, now.UTC()).
			Updates(map[string]any{"expires_at": expiresAt, "updated_at": now.UTC()})
		if result.Error != nil {
			return fmt.Errorf("renew Run lease: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrLeaseLost
		}
		workspace := tx.Model(&workspaceLeaseRecord{}).
			Where("lease_token = ? AND expires_at > ?", token, now.UTC()).
			Updates(map[string]any{"expires_at": expiresAt, "updated_at": now.UTC()})
		if workspace.Error != nil {
			return fmt.Errorf("renew Workspace Write Lease: %w", workspace.Error)
		}
		if workspace.RowsAffected != 1 {
			return domain.ErrLeaseLost
		}
		return nil
	})
}

func (repository *Repository) Control(ctx context.Context, runID string, expectedVersion int64, action domain.ControlAction, actorUserID string, now time.Time) (domain.Details, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", runID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrRunNotFound
			}
			return fmt.Errorf("lock controlled Run: %w", err)
		}
		if record.Version != expectedVersion {
			return domain.ErrConcurrentModification
		}
		run, err := restore(record)
		if err != nil {
			return err
		}
		eventType := ""
		extra := map[string]any{}
		switch action {
		case domain.ControlInterrupt:
			err = run.RequestInterrupt()
			eventType = "run.interrupt_requested"
		case domain.ControlResume:
			err = run.Resume()
			eventType = "run.resume_requested"
			extra["terminal_error"] = nil
		case domain.ControlCancel, domain.ControlKill:
			err = run.Cancel(now)
			eventType = "run.cancelled"
			if action == domain.ControlKill {
				eventType = "run.killed"
				extra["terminal_error"] = jsonValue(`{"code":"operator_killed","message":"Run was killed by an operator"}`)
			}
		default:
			return fmt.Errorf("unknown Run control action %q", action)
		}
		if err != nil {
			return err
		}
		if err := updateRun(tx, run, now, extra); err != nil {
			return fmt.Errorf("persist Run control action: %w", err)
		}
		if action == domain.ControlCancel || action == domain.ControlKill {
			attemptUpdates := map[string]any{"state": string(domain.AttemptCancelled), "ended_at": now.UTC()}
			if action == domain.ControlKill {
				attemptUpdates["error"] = extra["terminal_error"]
			}
			if err := tx.Model(&attemptRecord{}).Where("run_id = ? AND state IN ?", runID, []string{string(domain.AttemptProvisioning), string(domain.AttemptRunning)}).Updates(attemptUpdates).Error; err != nil {
				return fmt.Errorf("cancel active Run Attempt: %w", err)
			}
			if err := tx.Where("run_id = ?", runID).Delete(&leaseRecord{}).Error; err != nil {
				return fmt.Errorf("release controlled Run lease: %w", err)
			}
			if err := tx.Where("run_id = ?", runID).Delete(&workspaceLeaseRecord{}).Error; err != nil {
				return fmt.Errorf("release controlled Workspace Write Lease: %w", err)
			}
		}
		return appendEvent(tx, runID, eventType, map[string]any{"actor_user_id": actorUserID}, now)
	})
	if err != nil {
		return domain.Details{}, err
	}
	return repository.Get(ctx, runID)
}

func (repository *Repository) MarkRunning(ctx context.Context, token string, now time.Time) error {
	return repository.withLease(ctx, token, now, func(tx *gorm.DB, run *domain.Run, attemptID string) error {
		if err := run.MarkRunning(); err != nil {
			return err
		}
		if err := updateRun(tx, *run, now, nil); err != nil {
			return fmt.Errorf("mark Run running: %w", err)
		}
		if err := tx.Model(&attemptRecord{}).Where("id = ?", attemptID).Update("state", string(domain.Running)).Error; err != nil {
			return fmt.Errorf("mark Attempt running: %w", err)
		}
		return appendEvent(tx, run.ID, "run.running", map[string]any{"attempt_id": attemptID}, now)
	})
}

func (repository *Repository) FinishOwned(ctx context.Context, token string, outcome domain.Outcome, now time.Time) (domain.CompletionProjection, error) {
	var projection domain.CompletionProjection
	err := repository.withLease(ctx, token, now, func(tx *gorm.DB, run *domain.Run, attemptID string) error {
		projection = domain.CompletionProjection{RunID: run.ID, SessionID: run.SessionID}
		if run.State == domain.Interrupting {
			if err := run.MarkInterrupted(); err != nil {
				return err
			}
			extra := map[string]any{"usage": jsonValue(outcome.Usage), "cost_amount": outcome.Cost, "terminal_error": nil}
			if err := updateRun(tx, *run, now, extra); err != nil {
				return fmt.Errorf("mark Run interrupted: %w", err)
			}
			if err := tx.Model(&attemptRecord{}).Where("id = ?", attemptID).Updates(map[string]any{
				"state": string(domain.AttemptCancelled), "ended_at": now.UTC(),
			}).Error; err != nil {
				return fmt.Errorf("mark interrupted Attempt cancelled: %w", err)
			}
			if err := tx.Where("lease_token = ?", token).Delete(&leaseRecord{}).Error; err != nil {
				return fmt.Errorf("release interrupted Run lease: %w", err)
			}
			if result := tx.Where("lease_token = ?", token).Delete(&workspaceLeaseRecord{}); result.Error != nil || result.RowsAffected != 1 {
				if result.Error != nil {
					return fmt.Errorf("release interrupted Workspace Write Lease: %w", result.Error)
				}
				return domain.ErrLeaseLost
			}
			return appendEvent(tx, run.ID, "run.interrupted", map[string]any{"attempt_id": attemptID}, now)
		}
		if err := run.Finish(outcome, now); err != nil {
			return err
		}
		extra := map[string]any{
			"usage": jsonValue(outcome.Usage), "cost_amount": outcome.Cost,
			"terminal_error": nullableJSON(outcome.Error),
		}
		if err := updateRun(tx, *run, now, extra); err != nil {
			return fmt.Errorf("finish Run: %w", err)
		}
		attemptUpdates := map[string]any{
			"state": string(outcome.State), "error": nullableJSON(outcome.Error), "ended_at": now.UTC(),
		}
		if err := tx.Model(&attemptRecord{}).Where("id = ?", attemptID).Updates(attemptUpdates).Error; err != nil {
			return fmt.Errorf("finish Attempt: %w", err)
		}
		if err := tx.Where("lease_token = ?", token).Delete(&leaseRecord{}).Error; err != nil {
			return fmt.Errorf("release Run lease: %w", err)
		}
		if result := tx.Where("lease_token = ?", token).Delete(&workspaceLeaseRecord{}); result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return fmt.Errorf("release Workspace Write Lease: %w", result.Error)
			}
			return domain.ErrLeaseLost
		}
		projection.Completed = outcome.State == domain.Completed
		return appendEvent(tx, run.ID, "run."+string(outcome.State), map[string]any{"attempt_id": attemptID}, now)
	})
	return projection, err
}

func (repository *Repository) AppendEvent(ctx context.Context, token string, event domain.EventInput, now time.Time) error {
	return repository.withLease(ctx, token, now, func(tx *gorm.DB, run *domain.Run, _ string) error {
		return appendEvent(tx, run.ID, event.Type, json.RawMessage(event.Payload), now)
	})
}

func (repository *Repository) ReconcileExpired(ctx context.Context, maxAttempts int, now time.Time) (domain.ReconcileResult, error) {
	result := domain.ReconcileResult{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var expired []struct {
			RunID        string     `gorm:"column:run_id"`
			AttemptID    string     `gorm:"column:attempt_id"`
			SessionID    string     `gorm:"column:session_id"`
			State        string     `gorm:"column:state"`
			AttemptCount int        `gorm:"column:attempt_count"`
			Version      int64      `gorm:"column:version"`
			StartedAt    *time.Time `gorm:"column:started_at"`
			EndedAt      *time.Time `gorm:"column:ended_at"`
		}
		query := `
			SELECT l.run_id, l.attempt_id, r.session_id, r.state, r.attempt_count, r.version, r.started_at, r.ended_at
			FROM run_leases l
			JOIN runs r ON r.id = l.run_id
			WHERE l.expires_at <= ?
			ORDER BY l.expires_at
			LIMIT ?
			FOR UPDATE OF l, r SKIP LOCKED`
		if err := tx.Raw(query, now.UTC(), reconcileBatchSize).Scan(&expired).Error; err != nil {
			return fmt.Errorf("list expired Run leases: %w", err)
		}
		failure := jsonValue(`{"code":"infrastructure_lease_expired","message":"Worker lease expired"}`)
		for _, lease := range expired {
			run, err := domain.RestoreRun(lease.RunID, lease.SessionID, lease.State, lease.AttemptCount, lease.Version, lease.StartedAt, lease.EndedAt)
			if err != nil {
				return err
			}
			decision, err := run.ReconcileExpired(maxAttempts, now)
			if err != nil {
				return err
			}
			attemptUpdates := map[string]any{
				"state": "lost", "infrastructure_failure": true, "error": failure, "ended_at": now.UTC(),
			}
			if err := tx.Model(&attemptRecord{}).Where("id = ?", lease.AttemptID).Updates(attemptUpdates).Error; err != nil {
				return fmt.Errorf("mark expired Attempt lost: %w", err)
			}
			extra := map[string]any{"terminal_error": nil}
			if decision.State == domain.Failed {
				extra["terminal_error"] = failure
			}
			if err := updateRun(tx, run, now, extra); err != nil {
				return fmt.Errorf("reconcile expired Run: %w", err)
			}
			if err := tx.Where("run_id = ?", lease.RunID).Delete(&leaseRecord{}).Error; err != nil {
				return fmt.Errorf("delete expired Run lease: %w", err)
			}
			if err := tx.Where("run_id = ?", lease.RunID).Delete(&workspaceLeaseRecord{}).Error; err != nil {
				return fmt.Errorf("delete expired Workspace Write Lease: %w", err)
			}
			if err := appendEvent(tx, lease.RunID, decision.EventType, map[string]any{
				"attempt_id": lease.AttemptID, "reason": "infrastructure_lease_expired",
			}, now); err != nil {
				return err
			}
			if decision.State == domain.Resuming {
				result.Rescheduled++
			} else {
				result.Failed++
			}
		}
		return nil
	})
	return result, err
}

func (repository *Repository) ListEventsAfter(ctx context.Context, runID string, after int64, limit int) ([]domain.Event, error) {
	var records []eventRecord
	err := repository.db.WithContext(ctx).Where("run_id = ? AND sequence > ?", runID, after).
		Order("sequence").Limit(limit).Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list Run events: %w", err)
	}
	events := make([]domain.Event, 0, len(records))
	for _, record := range records {
		events = append(events, domain.Event{
			Sequence: record.Sequence, Type: record.EventType, Payload: cloneJSON(record.Payload), CreatedAt: record.CreatedAt,
		})
	}
	return events, nil
}

func (repository *Repository) withLease(ctx context.Context, token string, now time.Time, operation func(*gorm.DB, *domain.Run, string) error) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			RunID        string     `gorm:"column:run_id"`
			AttemptID    string     `gorm:"column:attempt_id"`
			SessionID    string     `gorm:"column:session_id"`
			State        string     `gorm:"column:state"`
			AttemptCount int        `gorm:"column:attempt_count"`
			Version      int64      `gorm:"column:version"`
			StartedAt    *time.Time `gorm:"column:started_at"`
			EndedAt      *time.Time `gorm:"column:ended_at"`
		}
		query := `
			SELECT l.run_id, l.attempt_id, r.session_id, r.state, r.attempt_count, r.version, r.started_at, r.ended_at
			FROM run_leases l
			JOIN runs r ON r.id = l.run_id
			JOIN workspace_write_leases workspace ON workspace.run_id = r.id
			WHERE l.lease_token = ? AND l.expires_at > ?
			  AND workspace.lease_token = l.lease_token AND workspace.expires_at > ?
			FOR UPDATE OF l, r, workspace`
		result := tx.Raw(query, token, now.UTC(), now.UTC()).Scan(&row)
		if result.Error != nil {
			return fmt.Errorf("load leased Run: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrLeaseLost
		}
		run, err := domain.RestoreRun(row.RunID, row.SessionID, row.State, row.AttemptCount, row.Version, row.StartedAt, row.EndedAt)
		if err != nil {
			return err
		}
		return operation(tx, &run, row.AttemptID)
	})
}

func restore(record runRecord) (domain.Run, error) {
	return domain.RestoreRun(record.ID, record.SessionID, record.State, record.AttemptCount, record.Version, record.StartedAt, record.EndedAt)
}

func updateRun(tx *gorm.DB, run domain.Run, now time.Time, extra map[string]any) error {
	updates := map[string]any{
		"state": run.State, "attempt_count": run.AttemptCount, "started_at": run.StartedAt,
		"ended_at": run.EndedAt, "updated_at": now.UTC(), "version": run.Version,
	}
	for key, value := range extra {
		updates[key] = value
	}
	result := tx.Model(&runRecord{}).Where("id = ? AND version = ?", run.ID, run.Version-1).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConcurrentModification
	}
	return nil
}

func appendEvent(tx *gorm.DB, runID, eventType string, payload any, now time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Run event: %w", err)
	}
	var sequence int64
	if err := tx.Model(&eventRecord{}).Where("run_id = ?", runID).
		Select("COALESCE(MAX(sequence), 0)").Scan(&sequence).Error; err != nil {
		return fmt.Errorf("read Run event sequence: %w", err)
	}
	record := eventRecord{
		RunID: runID, Sequence: sequence + 1, EventType: eventType,
		Payload: jsonValue(encoded), CreatedAt: now.UTC(),
	}
	if err := tx.Create(&record).Error; err != nil {
		return fmt.Errorf("append Run event: %w", err)
	}
	return nil
}

func cloneJSON(value jsonValue) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 || string(value) == "null" {
		return nil
	}
	return jsonValue(value)
}
