package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ application.WorkerRepository = (*Repository)(nil)

func (repository *Repository) ClaimNext(ctx context.Context) (*application.ExecutionJob, error) {
	var job *application.ExecutionJob
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := cancelDisabledOwnerWork(tx, time.Now().UTC()); err != nil {
			return err
		}
		claimed, err := claimMCPTest(tx)
		if err != nil {
			return err
		}
		if claimed != nil {
			job = claimed
			return nil
		}
		if err := enqueueDueSchedule(tx, time.Now().UTC()); err != nil {
			return err
		}
		claimed, err = claimWorkflowRun(tx)
		if err != nil {
			return err
		}
		if claimed != nil {
			job = claimed
			return nil
		}
		claimed, err = claimSessionMessage(tx)
		job = claimed
		return err
	})
	return job, err
}

func claimMCPTest(tx *gorm.DB) (*application.ExecutionJob, error) {
	var row mcpRecord
	err := tx.Raw(`
		SELECT server.* FROM mcp_servers server
		JOIN users owner ON owner.id = server.owner_user_id AND owner.disabled_at IS NULL
		WHERE server.test_requested_at IS NOT NULL AND server.tested_at IS NULL
		ORDER BY server.test_requested_at, server.id
		FOR UPDATE OF server SKIP LOCKED LIMIT 1`).Scan(&row).Error
	if err != nil || row.ID == "" {
		return nil, err
	}
	if err := tx.Model(&mcpRecord{}).Where("id = ? AND tested_at IS NULL", row.ID).
		Updates(map[string]any{"tested_at": gorm.Expr("now()"), "test_error": "running"}).Error; err != nil {
		return nil, err
	}
	return &application.ExecutionJob{
		Kind:        application.JobMCPTest,
		ID:          "mcp-test-" + row.ID,
		OwnerID:     row.OwnerID,
		MCPServerID: row.ID,
		MCPServer: domain.MCPServerSnapshot{
			ID: row.ID, Name: row.Name, Transport: row.Transport,
			Configuration: json.RawMessage(row.Configuration), SecretCiphertext: row.SecretCiphertext,
		},
	}, nil
}

func cancelDisabledOwnerWork(tx *gorm.DB, now time.Time) error {
	var runs []struct {
		ID string `gorm:"column:id"`
	}
	if err := tx.Raw(`
		UPDATE runs SET state = 'cancelled', cancel_requested_at = ?, ended_at = ?, version = version + 1
		WHERE state = 'queued' AND owner_user_id IN (SELECT id FROM users WHERE disabled_at IS NOT NULL)
		RETURNING id`, now, now).Scan(&runs).Error; err != nil {
		return err
	}
	for _, run := range runs {
		if err := tx.Table("run_events").Create(map[string]any{"run_id": run.ID, "sequence": 1, "event_type": "run.cancelled", "payload": []byte(`{}`), "occurred_at": now}).Error; err != nil {
			return err
		}
	}
	return tx.Exec(`
		UPDATE session_messages SET state = 'cancelled', progress_stage = '', completed_at = ?
		WHERE role = 'assistant' AND state = 'queued' AND session_id IN (
			SELECT session.id FROM sessions session JOIN users owner ON owner.id = session.owner_user_id WHERE owner.disabled_at IS NOT NULL
		)`, now).Error
}

func enqueueDueSchedule(tx *gorm.DB, now time.Time) error {
	var workflow workflowRecord
	err := tx.Raw(`
		SELECT workflow.* FROM workflows workflow
		JOIN users owner ON owner.id = workflow.owner_user_id AND owner.disabled_at IS NULL
		WHERE workflow.deleted_at IS NULL AND workflow.next_scheduled_at IS NOT NULL AND workflow.next_scheduled_at <= ?
		ORDER BY workflow.next_scheduled_at, workflow.id
		FOR UPDATE OF workflow SKIP LOCKED LIMIT 1`, now).Scan(&workflow).Error
	if err != nil || workflow.ID == "" {
		return err
	}
	var schedule domain.Schedule
	if err := json.Unmarshal(workflow.Schedule, &schedule); err != nil {
		return fmt.Errorf("decode due Workflow schedule: %w", err)
	}
	if _, err := createRunOnTx(tx, workflow.OwnerID, workflow.ID, "scheduled", nil, nil); err != nil {
		return err
	}
	next := nextScheduledAt(&schedule, now)
	return tx.Model(&workflowRecord{}).Where("id = ?", workflow.ID).Update("next_scheduled_at", next).Error
}

func claimWorkflowRun(tx *gorm.DB) (*application.ExecutionJob, error) {
	var row runRecord
	err := tx.Raw(`
		SELECT candidate.* FROM runs candidate
		JOIN users owner ON owner.id = candidate.owner_user_id AND owner.disabled_at IS NULL
		JOIN workflows workflow ON workflow.id = candidate.workflow_id AND workflow.deleted_at IS NULL
		WHERE candidate.state = 'queued'
		  AND NOT EXISTS (SELECT 1 FROM runs active WHERE active.workflow_id = candidate.workflow_id AND active.state = 'running')
		ORDER BY candidate.queued_at, candidate.id
		FOR UPDATE OF candidate, workflow SKIP LOCKED LIMIT 1`).Scan(&row).Error
	if err != nil || row.ID == "" {
		return nil, err
	}
	var snapshot domain.ExecutionSnapshot
	if err := json.Unmarshal(row.WorkflowSnapshot, &snapshot); err != nil {
		return nil, fmt.Errorf("decode claimed Workflow snapshot: %w", err)
	}
	var credential modelProviderCredentialVersionRecord
	if err := tx.Where("connection_id = ? AND connection_version = ?", snapshot.ProviderModel.ConnectionID, snapshot.ProviderModel.ConnectionVersion).Take(&credential).Error; err != nil {
		return nil, fmt.Errorf("load versioned Model Provider credential: %w", mapNotFound(err))
	}
	snapshot.ProviderModel.APIKeyCiphertext = credential.APIKeyCiphertext
	var input struct {
		Text *string        `json:"text"`
		JSON map[string]any `json:"json"`
	}
	if err := json.Unmarshal(row.Input, &input); err != nil {
		return nil, fmt.Errorf("decode claimed Run input: %w", err)
	}
	var prior []runRecord
	if row.TurnNumber > 1 {
		if err := tx.Where("conversation_id = ? AND turn_number < ?", row.ConversationID, row.TurnNumber).Order("turn_number").Find(&prior).Error; err != nil {
			return nil, fmt.Errorf("load prior Run Conversation turns: %w", err)
		}
	}
	instruction := workflowRunInstruction(snapshot.Goal, prior, input.Text, input.JSON)
	result := tx.Model(&runRecord{}).Where("id = ? AND state = 'queued'", row.ID).Updates(map[string]any{"state": "running", "started_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, result.Error
	}
	if err := tx.Table("run_events").Create(map[string]any{"run_id": row.ID, "sequence": 1, "event_type": "run.started", "payload": []byte(`{}`)}).Error; err != nil {
		return nil, err
	}
	workflowID := ""
	if row.WorkflowID != nil {
		workflowID = *row.WorkflowID
	}
	return &application.ExecutionJob{Kind: application.JobWorkflow, ID: row.ID, OwnerID: row.OwnerID, WorkflowID: workflowID, Instruction: instruction, Snapshot: snapshot}, nil
}

func workflowRunInstruction(goal string, prior []runRecord, textInput *string, jsonInput map[string]any) string {
	var builder strings.Builder
	builder.WriteString("Workflow goal:\n")
	builder.WriteString(strings.TrimSpace(goal))
	if len(prior) > 0 {
		builder.WriteString("\n\nPrevious conversation, oldest first:\n")
		for _, turn := range prior {
			var input struct {
				Text *string        `json:"text"`
				JSON map[string]any `json:"json"`
			}
			_ = json.Unmarshal(turn.Input, &input)
			if input.Text != nil && strings.TrimSpace(*input.Text) != "" {
				builder.WriteString("user: ")
				builder.WriteString(strings.TrimSpace(*input.Text))
				builder.WriteByte('\n')
			} else if input.JSON != nil {
				encoded, _ := json.Marshal(input.JSON)
				builder.WriteString("user: ")
				builder.Write(encoded)
				builder.WriteByte('\n')
			}
			var result struct {
				Text *string        `json:"text"`
				JSON map[string]any `json:"json"`
			}
			_ = json.Unmarshal(turn.FinalResult, &result)
			builder.WriteString("assistant: ")
			switch {
			case result.Text != nil:
				builder.WriteString(*result.Text)
			case result.JSON != nil:
				encoded, _ := json.Marshal(result.JSON)
				builder.Write(encoded)
			case turn.TerminalError != nil:
				builder.WriteString(*turn.TerminalError)
			}
			builder.WriteByte('\n')
		}
	}
	if textInput != nil && strings.TrimSpace(*textInput) != "" {
		builder.WriteString("\nCurrent user message:\n")
		builder.WriteString(strings.TrimSpace(*textInput))
	} else if jsonInput != nil {
		encoded, _ := json.Marshal(jsonInput)
		builder.WriteString("\nCurrent user message (JSON):\n")
		builder.Write(encoded)
	}
	return builder.String()
}

func claimSessionMessage(tx *gorm.DB) (*application.ExecutionJob, error) {
	var assistant messageRecord
	err := tx.Raw(`
		SELECT message.* FROM session_messages message
		JOIN sessions session ON session.id = message.session_id
		JOIN users owner ON owner.id = session.owner_user_id
		WHERE message.role = 'assistant' AND message.state = 'queued'
		  AND session.archived_at IS NULL AND owner.disabled_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM session_messages active
			WHERE active.session_id = message.session_id AND active.state = 'generating'
		  )
		ORDER BY message.id FOR UPDATE OF message SKIP LOCKED LIMIT 1`).Scan(&assistant).Error
	if err != nil || assistant.ID == 0 {
		return nil, err
	}
	var session sessionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", assistant.SessionID).Take(&session).Error; err != nil {
		return nil, err
	}
	var user messageRecord
	if err := tx.Where("session_id = ? AND id < ? AND role = 'user'", session.ID, assistant.ID).Order("id DESC").Take(&user).Error; err != nil {
		return nil, err
	}
	var recent []messageRecord
	if err := tx.Where("session_id = ? AND id < ? AND state = 'completed'", session.ID, user.ID).Order("id DESC").Limit(20).Find(&recent).Error; err != nil {
		return nil, err
	}
	if len(assistant.ResponseSnapshot) == 0 {
		return nil, fmt.Errorf("queued Session response has no Response Snapshot")
	}
	var responseSnapshot domain.ResponseSnapshot
	if err := json.Unmarshal(assistant.ResponseSnapshot, &responseSnapshot); err != nil {
		return nil, fmt.Errorf("decode Response Snapshot: %w", err)
	}
	snapshot, err := loadSessionSnapshot(tx, session, responseSnapshot)
	if err != nil {
		return nil, err
	}
	checkpoint := ""
	if session.RuntimeEngine != nil && *session.RuntimeEngine == string(snapshot.RuntimeEngine) {
		var checkpointRow struct {
			NativeCheckpoint string `gorm:"column:native_checkpoint"`
		}
		if err := tx.Table("sessions").Select("native_checkpoint").Where("id = ?", session.ID).Take(&checkpointRow).Error; err == nil {
			checkpoint = checkpointRow.NativeCheckpoint
		}
	}
	if err := tx.Model(&messageRecord{}).Where("id = ? AND state = 'queued'", assistant.ID).Updates(map[string]any{"state": "generating", "progress_stage": "thinking"}).Error; err != nil {
		return nil, err
	}
	instruction := sessionInstruction(session.RollingSummary, recent, user.Content)
	return &application.ExecutionJob{Kind: application.JobSession, ID: fmt.Sprintf("session-%s-%d", session.ID, assistant.ID), OwnerID: session.OwnerID, SessionID: session.ID, AssistantMessageID: assistant.ID, Instruction: instruction, CheckpointRef: checkpoint, Snapshot: snapshot}, nil
}

func loadSessionSnapshot(tx *gorm.DB, session sessionRecord, response domain.ResponseSnapshot) (domain.ExecutionSnapshot, error) {
	runtimeName := string(response.RuntimeEngine)
	providerModelID := response.ProviderModelID
	fake := workflowRecord{OwnerID: session.OwnerID, Name: session.Title, Goal: "", ExpertID: session.ExpertID, ProviderModelID: &providerModelID, RuntimeEngine: &runtimeName, WorkspacePath: "sessions/" + session.OwnerID + "/" + session.ID}
	current, err := loadExecutionSnapshot(tx, fake)
	if err != nil {
		return domain.ExecutionSnapshot{}, err
	}
	var credential modelProviderCredentialVersionRecord
	if err := tx.Where("connection_id = ? AND connection_version = ?", response.ConnectionID, response.ConnectionVersion).Take(&credential).Error; err != nil {
		return domain.ExecutionSnapshot{}, fmt.Errorf("load versioned Model Provider credential: %w", mapNotFound(err))
	}
	current.ProviderModel = domain.ProviderModelSnapshot{ID: response.ProviderModelID, ConnectionID: response.ConnectionID, ConnectionVersion: response.ConnectionVersion, ConnectionName: response.ConnectionName, ProviderType: response.ProviderType, ModelID: response.ModelID, Name: response.ModelName, Endpoint: response.Endpoint, Protocols: response.Protocols, APIKeyCiphertext: credential.APIKeyCiphertext}
	if session.ExpertID == nil {
		return current, nil
	}
	stored := session.ExpertSnapshot
	if len(stored) == 0 || string(stored) == "null" {
		encoded, err := marshal(current)
		if err != nil {
			return domain.ExecutionSnapshot{}, err
		}
		if err := tx.Table("sessions").Where("id = ?", session.ID).Update("expert_snapshot", encoded).Error; err != nil {
			return domain.ExecutionSnapshot{}, err
		}
		return current, nil
	}
	var frozen domain.ExecutionSnapshot
	if err := json.Unmarshal(stored, &frozen); err != nil {
		return domain.ExecutionSnapshot{}, err
	}
	current.Expert, current.MCPServers, current.Skills = frozen.Expert, frozen.MCPServers, frozen.Skills
	return current, nil
}

func (repository *Repository) FinishSucceeded(ctx context.Context, job application.ExecutionJob, result application.ExecutionResult) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if job.Kind == application.JobSession {
			update := tx.Model(&messageRecord{}).Where("id = ? AND session_id = ? AND state = 'generating' AND cancel_requested_at IS NULL", job.AssistantMessageID, job.SessionID).Updates(map[string]any{"state": "completed", "content": result.FinalMessage, "progress_stage": "", "elapsed_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now), "completed_at": now})
			if update.Error != nil || update.RowsAffected != 1 {
				if update.Error != nil {
					return fmt.Errorf("complete Session message: %w", update.Error)
				}
				return finishCancelledSessionMessage(tx, job, now)
			}
			summary, through, err := advanceRollingSummary(tx, job.SessionID, job.AssistantMessageID)
			if err != nil {
				return err
			}
			return tx.Model(&sessionRecord{}).Where("id = ?", job.SessionID).Updates(map[string]any{"runtime_engine": string(job.Snapshot.RuntimeEngine), "native_checkpoint": result.CheckpointRef, "rolling_summary": summary, "summary_through_message_id": through, "updated_at": now, "version": gorm.Expr("version + 1")}).Error
		}
		finalResult, err := marshal(map[string]any{"text": result.FinalMessage, "json": result.FinalJSON})
		if err != nil {
			return err
		}
		update := tx.Model(&runRecord{}).Where("id = ? AND state = 'running'", job.ID).Updates(map[string]any{"state": "succeeded", "final_result": finalResult, "ended_at": now, "version": gorm.Expr("version + 1")})
		if update.Error != nil || update.RowsAffected != 1 {
			return fmt.Errorf("complete Workflow Run: %w", update.Error)
		}
		if err := appendRunEvents(tx, job.ID, result.Events, "run.succeeded", now); err != nil {
			return err
		}
		for _, artifact := range fileArtifactRecords(job, result.Artifacts, now) {
			if err := tx.Create(&artifact).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func fileArtifactRecords(job application.ExecutionJob, artifacts []application.ExecutionArtifact, createdAt time.Time) []artifactRecord {
	records := make([]artifactRecord, 0, len(artifacts))
	for _, artifact := range artifacts {
		objectKey := artifact.ObjectKey
		var preview []byte
		if artifact.TextPreview != "" {
			preview, _ = marshal(artifact.TextPreview)
		}
		expiresAt := artifact.ExpiresAt
		sha := artifact.SHA256
		records = append(records, artifactRecord{ID: uuid.NewString(), OwnerID: job.OwnerID, WorkflowID: &job.WorkflowID, RunID: job.ID, Kind: "file", Name: artifact.Name, Path: artifact.Path, ObjectKey: &objectKey, TextResult: preview, Size: artifact.Size, SHA256: &sha, CreatedAt: createdAt, ExpiresAt: &expiresAt})
	}
	return records
}

func advanceRollingSummary(tx *gorm.DB, sessionID string, completedMessageID int64) (string, *int64, error) {
	var session sessionRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", sessionID).Take(&session).Error; err != nil {
		return "", nil, err
	}
	through := int64(0)
	if session.SummaryThrough != nil {
		through = *session.SummaryThrough
	}
	var messages []messageRecord
	if err := tx.Raw(`
		SELECT message.* FROM session_messages message
		WHERE message.session_id = ? AND message.state = 'completed' AND message.id > ?
		  AND message.id IN (
			SELECT recent.id FROM session_messages recent
			WHERE recent.session_id = ? AND recent.state = 'completed' AND recent.id <= ?
			ORDER BY recent.id DESC OFFSET 20
		  )
		ORDER BY message.id ASC LIMIT 1000`, sessionID, through, sessionID, completedMessageID).Scan(&messages).Error; err != nil {
		return "", nil, err
	}
	summary := session.RollingSummary
	for _, message := range messages {
		if summary != "" {
			summary += "\n"
		}
		summary += message.Role + ": " + message.Content
		through = message.ID
	}
	if through == 0 {
		return boundedSummary(summary), nil, nil
	}
	return boundedSummary(summary), &through, nil
}

func (repository *Repository) FinishFailed(ctx context.Context, job application.ExecutionJob, message string) error {
	if len(message) > 4_096 {
		message = message[:4_096]
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if job.Kind == application.JobSession {
			update := tx.Model(&messageRecord{}).Where("id = ? AND session_id = ? AND state = 'generating' AND cancel_requested_at IS NULL", job.AssistantMessageID, job.SessionID).Updates(map[string]any{"state": "failed", "error": message, "progress_stage": "", "elapsed_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now), "completed_at": now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 1 {
				return nil
			}
			return finishCancelledSessionMessage(tx, job, now)
		}
		update := tx.Model(&runRecord{}).Where("id = ? AND state = 'running'", job.ID).Updates(map[string]any{"state": "failed", "terminal_error": message, "ended_at": now, "version": gorm.Expr("version + 1")})
		if update.Error != nil || update.RowsAffected != 1 {
			return update.Error
		}
		return appendRunEvents(tx, job.ID, nil, "run.failed", now)
	})
}

func (repository *Repository) FinishCancelled(ctx context.Context, job application.ExecutionJob) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if job.Kind == application.JobSession {
			return tx.Model(&messageRecord{}).Where("id = ? AND session_id = ? AND state = 'generating'", job.AssistantMessageID, job.SessionID).Updates(map[string]any{"state": "cancelled", "progress_stage": "", "elapsed_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now), "completed_at": now}).Error
		}
		update := tx.Model(&runRecord{}).Where("id = ? AND state = 'running'", job.ID).Updates(map[string]any{"state": "cancelled", "ended_at": now, "version": gorm.Expr("version + 1")})
		if update.Error != nil || update.RowsAffected != 1 {
			return update.Error
		}
		return appendRunEvents(tx, job.ID, nil, "run.cancelled", now)
	})
}

func finishCancelledSessionMessage(tx *gorm.DB, job application.ExecutionJob, now time.Time) error {
	result := tx.Model(&messageRecord{}).
		Where("id = ? AND session_id = ? AND state = 'generating' AND cancel_requested_at IS NOT NULL", job.AssistantMessageID, job.SessionID).
		Updates(map[string]any{"state": "cancelled", "progress_stage": "", "elapsed_ms": gorm.Expr("GREATEST(0, EXTRACT(EPOCH FROM (? - created_at)) * 1000)::bigint", now), "completed_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) FinishMCPTest(ctx context.Context, job application.ExecutionJob, message string) error {
	if len(message) > 4_096 {
		message = message[:4_096]
	}
	updates := map[string]any{
		"test_requested_at": nil,
		"tested_at":         gorm.Expr("now()"),
		"test_error":        nil,
		"updated_at":        gorm.Expr("now()"),
		"version":           gorm.Expr("version + 1"),
	}
	if message != "" {
		updates["test_error"] = message
	}
	result := repository.db.WithContext(ctx).Model(&mcpRecord{}).
		Where("id = ? AND owner_user_id = ? AND test_requested_at IS NOT NULL", job.MCPServerID, job.OwnerID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("finish MCP test: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) RecordProgress(ctx context.Context, job application.ExecutionJob, event application.ExecutionEvent) error {
	if len(event.Payload) > 256<<10 || !json.Valid(event.Payload) {
		return fmt.Errorf("invalid Runtime progress payload")
	}
	if job.Kind == application.JobSession {
		updates := map[string]any{}
		switch event.Type {
		case "runtime.started", "command.completed":
			updates["progress_stage"] = "thinking"
		case "command.requested":
			updates["progress_stage"] = "using_tool"
		case "file.changed":
			updates["progress_stage"] = "working"
		case "message.delta":
			var payload struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				return err
			}
			if payload.Delta == "" {
				return nil
			}
			updates["content"] = gorm.Expr("content || ?", payload.Delta)
			updates["progress_stage"] = "responding"
		default:
			return nil
		}
		result := repository.db.WithContext(ctx).Model(&messageRecord{}).
			Where("id = ? AND session_id = ? AND state = 'generating' AND cancel_requested_at IS NULL", job.AssistantMessageID, job.SessionID).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("record Session response progress: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return nil
	}
	if job.Kind != application.JobWorkflow {
		return nil
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var run runRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "state").Where("id = ? AND owner_user_id = ?", job.ID, job.OwnerID).Take(&run).Error; err != nil {
			return mapNotFound(err)
		}
		if run.State != "running" {
			return domain.ErrConflict
		}
		var sequence int64
		if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", job.ID).Scan(&sequence).Error; err != nil {
			return err
		}
		return tx.Table("run_events").Create(map[string]any{"run_id": job.ID, "sequence": sequence + 1, "event_type": event.Type, "payload": event.Payload, "occurred_at": time.Now().UTC()}).Error
	})
}

func (repository *Repository) CancellationRequested(ctx context.Context, job application.ExecutionJob) (bool, error) {
	if job.Kind == application.JobMCPTest {
		return false, nil
	}
	var count int64
	if job.Kind == application.JobSession {
		err := repository.db.WithContext(ctx).Table("session_messages message").
			Joins("JOIN sessions session ON session.id = message.session_id").
			Joins("JOIN users owner ON owner.id = session.owner_user_id").
			Where("message.id = ? AND message.state = 'generating' AND message.cancel_requested_at IS NULL AND session.archived_at IS NULL AND owner.disabled_at IS NULL", job.AssistantMessageID).
			Count(&count).Error
		return count == 0, err
	}
	err := repository.db.WithContext(ctx).Table("runs run").
		Joins("JOIN users owner ON owner.id = run.owner_user_id").
		Where("run.id = ? AND run.state = 'running' AND run.cancel_requested_at IS NULL AND owner.disabled_at IS NULL", job.ID).
		Count(&count).Error
	return count == 0, err
}

func sessionInstruction(summary string, recent []messageRecord, current string) string {
	var builder strings.Builder
	if summary = strings.TrimSpace(summary); summary != "" {
		builder.WriteString("Rolling summary from the previous conversation:\n")
		builder.WriteString(summary)
		builder.WriteString("\n\n")
	}
	if len(recent) > 0 {
		builder.WriteString("Recent conversation, oldest first:\n")
		for index := len(recent) - 1; index >= 0; index-- {
			message := recent[index]
			builder.WriteString(message.Role)
			builder.WriteString(": ")
			builder.WriteString(message.Content)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("Current user message:\n")
	builder.WriteString(current)
	return builder.String()
}

func appendRunEvents(tx *gorm.DB, runID string, events []application.ExecutionEvent, terminal string, now time.Time) error {
	var sequence int64
	if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", runID).Scan(&sequence).Error; err != nil {
		return err
	}
	for _, event := range events {
		sequence++
		if err := tx.Table("run_events").Create(map[string]any{"run_id": runID, "sequence": sequence, "event_type": event.Type, "payload": event.Payload, "occurred_at": now}).Error; err != nil {
			return err
		}
	}
	sequence++
	return tx.Table("run_events").Create(map[string]any{"run_id": runID, "sequence": sequence, "event_type": terminal, "payload": []byte(`{}`), "occurred_at": now}).Error
}

func boundedSummary(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 20_000 {
		start := len(value) - 20_000
		for start < len(value) && !utf8.RuneStart(value[start]) {
			start++
		}
		return value[start:]
	}
	return value
}
