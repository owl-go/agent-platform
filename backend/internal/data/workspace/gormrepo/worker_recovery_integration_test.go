package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workerRecoveryFixture struct {
	db         *gorm.DB
	repository *Repository
	ownerID    string
	sessionID  string
	assistant  application.ExecutionJob
}

func newWorkerRecoveryFixture(t *testing.T) workerRecoveryFixture {
	t.Helper()
	db := conversationTestDatabase(t)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}

	ownerID, connectionID, modelID, sessionID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", ownerID, ownerID, ownerID, ownerID+"@example.test", ownerID)
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','test')`, connectionID, ownerID)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'test')`, connectionID)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model','Model')`, modelID, connectionID)
	defaults, err := json.Marshal(map[string]string{"codex": modelID})
	if err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO personal_settings(user_id,runtime_model_defaults) VALUES(?,?::jsonb)`, ownerID, string(defaults))
	exec(`INSERT INTO sessions(id,owner_user_id) VALUES(?,?)`, sessionID, ownerID)

	repository := New(db, nil)
	t.Cleanup(func() {
		if err := repository.releaseWorkerClaimLock(context.Background()); err != nil {
			t.Error(err)
		}
	})
	_, assistant, err := repository.CreateMessagePair(ctx, ownerID, sessionID, "continue the task", nil)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := repository.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.Kind != application.JobSession || claimed.AssistantMessageID != assistant.ID {
		t.Fatalf("initial claim = %#v", claimed)
	}
	return workerRecoveryFixture{db: db, repository: repository, ownerID: ownerID, sessionID: sessionID, assistant: *claimed}
}

func restartWorkerRepository(t *testing.T, db *gorm.DB, previous *Repository) *Repository {
	t.Helper()
	if err := previous.releaseWorkerClaimLock(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted := New(db, nil)
	t.Cleanup(func() {
		if err := restarted.releaseWorkerClaimLock(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return restarted
}

func TestClaimNextRecoversInterruptedSessionAfterWorkerRestart(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ctx := context.Background()
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Updates(map[string]any{
		"content": "partial output", "progress_stage": "responding", "runtime_activities": []byte(`[{"type":"runtime.started"}]`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	restartedRepository := restartWorkerRepository(t, fixture.db, fixture.repository)
	recovered, err := restartedRepository.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.Kind != application.JobSession || recovered.AssistantMessageID != fixture.assistant.AssistantMessageID {
		t.Fatalf("claim after Worker restart = %#v, want interrupted Session message %d", recovered, fixture.assistant.AssistantMessageID)
	}

	var row messageRecord
	if err := fixture.db.Where("id = ?", fixture.assistant.AssistantMessageID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "generating" || row.Content != "" || row.ProgressStage != "thinking" || string(row.RuntimeActivities) != "[]" {
		t.Fatalf("recovered Session message retained interrupted output: %#v", row)
	}
	if cancelled, err := restartedRepository.CancellationRequested(ctx, application.ExecutionJob{
		Kind: application.JobSession, OwnerID: fixture.ownerID, SessionID: fixture.sessionID, AssistantMessageID: fixture.assistant.AssistantMessageID,
	}); err != nil || cancelled {
		t.Fatalf("recovered Session message cancellation = %t, %v", cancelled, err)
	}
}

func TestWorkerRestartFinalizesInterruptedSessionCancellation(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ctx := context.Background()
	if _, err := fixture.repository.CancelMessage(ctx, fixture.ownerID, fixture.sessionID, fixture.assistant.AssistantMessageID); err != nil {
		t.Fatal(err)
	}

	restartedRepository := restartWorkerRepository(t, fixture.db, fixture.repository)
	if job, err := restartedRepository.ClaimNext(ctx); err != nil || job != nil {
		t.Fatalf("claim after recovering cancelled Session response = %#v, %v", job, err)
	}
	var row messageRecord
	if err := fixture.db.Where("id = ?", fixture.assistant.AssistantMessageID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "cancelled" || row.Completed == nil {
		t.Fatalf("interrupted cancellation was not finalized: %#v", row)
	}
}

func TestWorkerRestartRequeuesUserActionWaitAndClosesStaleApproval(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ctx := context.Background()
	approvalID := uuid.NewString()
	if err := fixture.db.Exec(`
		INSERT INTO cli_command_approvals(
			id, owner_user_id, execution_kind, execution_id, stage_id, connector_name,
			operation, target, redacted_arguments, command_digest, nonce_hash, state, expires_at
		) VALUES (?, ?, 'session', ?, ?, 'Connector', 'write', 'record', '{}', ?, ?, 'pending', now() + interval '5 minutes')`,
		approvalID, fixture.ownerID, fmt.Sprintf("%d", fixture.assistant.AssistantMessageID), "stage-"+approvalID,
		strings.Repeat("a", 64), strings.Repeat("b", 64)).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Update("state", "waiting_for_user").Error; err != nil {
		t.Fatal(err)
	}

	restartedRepository := restartWorkerRepository(t, fixture.db, fixture.repository)
	recovered, err := restartedRepository.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.AssistantMessageID != fixture.assistant.AssistantMessageID {
		t.Fatalf("claim after recovering User Action Wait = %#v", recovered)
	}
	var approvalState string
	if err := fixture.db.Table("cli_command_approvals").Select("state").Where("id = ?", approvalID).Scan(&approvalState).Error; err != nil {
		t.Fatal(err)
	}
	if approvalState != "closed" {
		t.Fatalf("stale Approval state = %q, want closed", approvalState)
	}
}

func TestWorkerRestartDoesNotReplayConsumedConnectorApproval(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ctx := context.Background()
	approvalID := uuid.NewString()
	if err := fixture.db.Exec(`
		INSERT INTO cli_command_approvals(
			id, owner_user_id, execution_kind, execution_id, stage_id, connector_name,
			operation, target, redacted_arguments, command_digest, nonce_hash, state, expires_at, consumed_at
		) VALUES (?, ?, 'session', ?, ?, 'Connector', 'write', 'record', '{}', ?, ?, 'consumed', now() + interval '5 minutes', now())`,
		approvalID, fixture.ownerID, fmt.Sprintf("%d", fixture.assistant.AssistantMessageID), "stage-"+approvalID,
		strings.Repeat("c", 64), strings.Repeat("d", 64)).Error; err != nil {
		t.Fatal(err)
	}

	restartedRepository := restartWorkerRepository(t, fixture.db, fixture.repository)
	if job, err := restartedRepository.ClaimNext(ctx); err != nil || job != nil {
		t.Fatalf("claim after consumed Connector Approval = %#v, %v", job, err)
	}
	var row messageRecord
	if err := fixture.db.Where("id = ?", fixture.assistant.AssistantMessageID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "failed" || row.Error == nil || !strings.Contains(*row.Error, "outcome is unknown") {
		t.Fatalf("consumed Connector Approval recovery = %#v", row)
	}
}

func TestClaimNextRecoversInterruptedWorkflowRunAfterWorkerRestart(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	ctx := context.Background()
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Updates(map[string]any{
		"state": "failed", "progress_stage": "", "completed_at": gorm.Expr("now()"),
	}).Error; err != nil {
		t.Fatal(err)
	}

	workflowID, runID := uuid.NewString(), uuid.NewString()
	if err := fixture.db.Exec(`INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','Goal',?)`, workflowID, fixture.ownerID, "workspaces/"+workflowID).Error; err != nil {
		t.Fatal(err)
	}
	snapshot, err := json.Marshal(fixture.assistant.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot,expert_stages) VALUES(?,?,1,?,?,'Workflow','manual','queued',?::jsonb,'[]'::jsonb)`, runID, runID, fixture.ownerID, workflowID, string(snapshot)).Error; err != nil {
		t.Fatal(err)
	}

	firstRepository := restartWorkerRepository(t, fixture.db, fixture.repository)
	claimed, err := firstRepository.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if claimed == nil || claimed.Kind != application.JobWorkflow || claimed.ID != runID {
		t.Fatalf("initial Workflow claim = %#v", claimed)
	}
	if err := fixture.db.Model(&runRecord{}).Where("id = ?", runID).Updates(map[string]any{
		"terminal_error": "partial failure", "expert_stages": []byte(`[{"position":1,"state":"running"}]`),
	}).Error; err != nil {
		t.Fatal(err)
	}

	restartedRepository := restartWorkerRepository(t, fixture.db, firstRepository)
	recovered, err := restartedRepository.ClaimNext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.Kind != application.JobWorkflow || recovered.ID != runID {
		t.Fatalf("claim after Worker restart = %#v, want interrupted Workflow Run %s", recovered, runID)
	}
	var row runRecord
	if err := fixture.db.Where("id = ?", runID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "running" || row.TerminalError != nil || string(row.ExpertStages) != "[]" {
		t.Fatalf("recovered Workflow Run retained interrupted state: %#v", row)
	}
	var eventTypes []string
	if err := fixture.db.Table("run_events").Where("run_id = ?", runID).Order("sequence").Pluck("event_type", &eventTypes).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(eventTypes, ",") != "run.started,run.started" {
		t.Fatalf("recovered Workflow event sequence = %v", eventTypes)
	}
}

func TestConcurrentWorkerCannotRecoverAnActiveExecution(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	contender := New(fixture.db, nil)
	if job, err := contender.ClaimNext(context.Background()); err == nil || !strings.Contains(err.Error(), "already active") || job != nil {
		t.Fatalf("concurrent Worker claim = %#v, %v", job, err)
	}
}
