package gormrepo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/collaboration/application"
	"agent-platform/backend/internal/biz/collaboration/domain"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/collaboration/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fixedClock time.Time

func (clock fixedClock) Now() time.Time { return time.Time(clock) }

type randomIDs struct{}

func (randomIDs) NewID() string { return uuid.NewString() }

type sequenceIDs struct {
	values []string
	index  int
}

func (ids *sequenceIDs) NewID() string {
	value := ids.values[ids.index]
	ids.index++
	return value
}

func TestCollaborationLaunchContinueAndMemoryAreTransactional(t *testing.T) {
	database := openIntegrationDatabase(t)
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	organizationID, teamID, userID, agentID, releaseID := seedFixture(t, tx)
	now := time.Now().UTC()
	repository := gormrepo.New(tx)
	service := application.NewWithDependencies(repository, bizworkflow.NewLaunch(gormtx.New(tx)), fixedClock(now), randomIDs{})
	if err := tx.Exec(`UPDATE configured_models SET model_id = 'mutated-model', endpoint = 'https://mutated.example.test' WHERE id = (SELECT configured_model_id FROM agent_releases WHERE id = ?)`, releaseID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(`UPDATE repository_bindings SET default_branch = 'mutated-branch' WHERE id = (SELECT repository_binding_id FROM agent_releases WHERE id = ?)`, releaseID).Error; err != nil {
		t.Fatal(err)
	}

	launch, err := service.CreateTask(context.Background(), application.CreateTaskCommand{
		OrganizationID: organizationID, TeamID: teamID, AgentReleaseID: releaseID, CreatedBy: userID,
		Title: "Fix parser", RequestText: "Handle empty input and run tests.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if launch.Task.State != domain.TaskStateActive || launch.Session.RunCount != 1 || launch.RunID == "" {
		t.Fatalf("unexpected launch: %#v", launch)
	}
	if launch.Session.TargetBranch != "main" {
		t.Fatalf("Session target branch = %q, want frozen Release snapshot", launch.Session.TargetBranch)
	}
	var frozenModel string
	if err := tx.Raw(`SELECT model_binding::text FROM runs WHERE id = ?`, launch.RunID).Scan(&frozenModel).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frozenModel, `"model_id": "model-a"`) || !strings.Contains(frozenModel, `"endpoint": "https://model.example.test"`) || strings.Contains(frozenModel, "mutated") {
		t.Fatalf("Run Model Binding did not use the Release snapshot: %s", frozenModel)
	}
	assertCounts(t, tx, launch.Task.ID, launch.Session.ID, launch.RunID, 1, 1, 1)

	assertUnavailableLaunch := func(update, restore string, expected error) {
		t.Helper()
		if err := tx.Exec(update, releaseID).Error; err != nil {
			t.Fatal(err)
		}
		_, err := service.CreateTask(context.Background(), application.CreateTaskCommand{
			OrganizationID: organizationID, TeamID: teamID, AgentReleaseID: releaseID, CreatedBy: userID,
			Title: "Unavailable dependency", RequestText: "This launch must fail closed.",
		})
		if !errors.Is(err, expected) {
			t.Fatalf("CreateTask() dependency error = %v, want %v", err, expected)
		}
		if err := tx.Exec(restore, releaseID).Error; err != nil {
			t.Fatal(err)
		}
	}
	assertUnavailableLaunch(
		`UPDATE runtime_images SET status = 'blocked', blocked_reason = 'test block' WHERE id = (SELECT runtime_image_id FROM agent_releases WHERE id = ?)`,
		`UPDATE runtime_images SET status = 'production', blocked_reason = NULL WHERE id = (SELECT runtime_image_id FROM agent_releases WHERE id = ?)`,
		domain.ErrRuntimeUnavailable,
	)
	assertUnavailableLaunch(
		`UPDATE configured_models SET enabled = false WHERE id = (SELECT configured_model_id FROM agent_releases WHERE id = ?)`,
		`UPDATE configured_models SET enabled = true WHERE id = (SELECT configured_model_id FROM agent_releases WHERE id = ?)`,
		domain.ErrModelUnavailable,
	)
	assertUnavailableLaunch(
		`UPDATE repository_bindings SET validation_report = '{"valid":false,"errors":{"repository":"test"},"checked_at":"2026-08-20T00:00:00Z"}'::jsonb WHERE id = (SELECT repository_binding_id FROM agent_releases WHERE id = ?)`,
		`UPDATE repository_bindings SET validation_report = '{"valid":true,"errors":{},"checked_at":"2026-08-20T00:00:00Z"}'::jsonb WHERE id = (SELECT repository_binding_id FROM agent_releases WHERE id = ?)`,
		domain.ErrBindingUnavailable,
	)
	if err := tx.Exec(`UPDATE credential_profiles SET team_id = NULL WHERE id::text IN (
		SELECT repository_binding_snapshot->>'ssh_credential_profile_id' FROM agent_releases WHERE id = ?
		UNION ALL
		SELECT repository_binding_snapshot->'build_credential_profile_ids'->>0 FROM agent_releases WHERE id = ?
	)`, releaseID, releaseID).Error; err != nil {
		t.Fatal(err)
	}
	catalog, err := repository.ListLaunchOptions(context.Background(), organizationID, teamID)
	if err != nil || len(catalog.Options) != 1 {
		t.Fatalf("Organization-scoped Repository credentials made a valid Release unavailable: (%#v, %v)", catalog, err)
	}

	duplicateRunTaskID, duplicateRunSessionID := uuid.NewString(), uuid.NewString()
	duplicateIDs := &sequenceIDs{values: []string{duplicateRunTaskID, duplicateRunSessionID, launch.RunID, uuid.NewString()}}
	rollbackService := application.NewWithDependencies(repository, bizworkflow.NewLaunch(gormtx.New(tx)), fixedClock(now), duplicateIDs)
	if _, err := rollbackService.CreateTask(context.Background(), application.CreateTaskCommand{
		OrganizationID: organizationID, TeamID: teamID, AgentReleaseID: releaseID, CreatedBy: userID,
		Title: "Must roll back", RequestText: "Force the Execution write to fail.",
	}); err == nil {
		t.Fatal("duplicate Run ID unexpectedly committed a partial launch")
	}
	assertLaunchRolledBack(t, tx, duplicateRunTaskID, duplicateRunSessionID)

	waiting, err := service.ChangeTaskState(context.Background(), organizationID, teamID, launch.Task.ID, launch.Task.Version, domain.TaskStateWaitingForUser)
	if err != nil {
		t.Fatal(err)
	}
	continued, err := service.ContinueTask(context.Background(), application.ContinueTaskCommand{
		OrganizationID: organizationID, TeamID: teamID, TaskID: launch.Task.ID, CreatedBy: userID,
		RequestText: "Also add a regression test.", ExpectedTaskVersion: waiting.Version,
		ExpectedSessionVersion: launch.Session.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	if continued.Task.State != domain.TaskStateActive || continued.Session.RunCount != 2 {
		t.Fatalf("unexpected continuation: %#v", continued)
	}
	assertCounts(t, tx, launch.Task.ID, launch.Session.ID, continued.RunID, 2, 2, 1)

	candidate, err := service.ProposeMemory(context.Background(), organizationID, teamID, launch.Task.ID, agentID, "Run parser regression tests before committing.")
	if err != nil {
		t.Fatal(err)
	}
	decided, memory, err := service.DecideMemory(context.Background(), organizationID, teamID, candidate.ID, userID, true)
	if err != nil {
		t.Fatal(err)
	}
	if decided.State != domain.MemoryCandidateApproved || memory == nil || !memory.Enabled {
		t.Fatalf("unexpected Memory approval: %#v %#v", decided, memory)
	}
	persisted, err := repository.GetAgentMemory(context.Background(), organizationID, teamID, memory.ID)
	if err != nil || persisted.Content != candidate.ProposedContent {
		t.Fatalf("persisted Agent Memory = (%#v, %v)", persisted, err)
	}
}

func TestCodingTaskLaunchSerializesBuildCredentialDisable(t *testing.T) {
	database := openIntegrationDatabase(t)
	organizationID, teamID, userID, _, releaseID := seedFixture(t, database.ORM())
	t.Cleanup(func() { cleanupFixture(t, database.ORM(), organizationID) })

	var buildCredentialID string
	if err := database.ORM().Raw(`SELECT repository_binding_snapshot->'build_credential_profile_ids'->>0 FROM agent_releases WHERE id = ?`, releaseID).Scan(&buildCredentialID).Error; err != nil {
		t.Fatal(err)
	}
	tx := database.ORM().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	now := time.Now().UTC()
	task, err := domain.RegisterTask(domain.TaskRegistration{
		ID: uuid.NewString(), OrganizationID: organizationID, TeamID: teamID, AgentReleaseID: releaseID,
		CreatedBy: userID, Title: "Lock credentials", RequestText: "Keep launch credentials stable.", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = gormrepo.New(tx).CreateLaunchOwned(context.Background(), domain.LaunchRegistration{
		Task: task, SessionID: uuid.NewString(),
		Run:          domain.RunSeed{ID: uuid.NewString(), RequestText: task.RequestText, CreatedBy: userID},
		ReviewBranch: "agent/credential-lock", WorkspaceVolume: "workspace-lock", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	disableResult := make(chan error, 1)
	go func() {
		disableResult <- database.ORM().Exec(`UPDATE credential_profiles SET disabled_at = now() WHERE id = ?`, buildCredentialID).Error
	}()
	select {
	case err := <-disableResult:
		t.Fatalf("Build Credential disable completed while launch transaction held its snapshot lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := tx.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-disableResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Build Credential disable did not resume after launch transaction completed")
	}
}

func cleanupFixture(t *testing.T, db *gorm.DB, organizationID string) {
	t.Helper()
	statements := []string{
		`DELETE FROM agent_releases WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = ?)`,
		`DELETE FROM agent_drafts WHERE agent_id IN (SELECT id FROM agents WHERE organization_id = ?)`,
		`DELETE FROM agents WHERE organization_id = ?`,
		`DELETE FROM repository_bindings WHERE organization_id = ?`,
		`DELETE FROM configured_models WHERE organization_id = ?`,
		`DELETE FROM runtime_images WHERE organization_id = ?`,
		`DELETE FROM credential_profiles WHERE organization_id = ?`,
		`DELETE FROM source_control_providers WHERE organization_id = ?`,
		`DELETE FROM users WHERE organization_id = ?`,
		`DELETE FROM teams WHERE organization_id = ?`,
		`DELETE FROM organizations WHERE id = ?`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement, organizationID).Error; err != nil {
			t.Errorf("clean Collaboration fixture: %v", err)
		}
	}
}

func assertLaunchRolledBack(t *testing.T, db *gorm.DB, taskID, sessionID string) {
	t.Helper()
	var result struct{ Tasks, Sessions, Messages int64 }
	if err := db.Raw(`SELECT
		(SELECT count(*) FROM coding_tasks WHERE id = ?) AS tasks,
		(SELECT count(*) FROM sessions WHERE id = ?) AS sessions,
		(SELECT count(*) FROM session_messages WHERE session_id = ?) AS messages`, taskID, sessionID, sessionID).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.Tasks != 0 || result.Sessions != 0 || result.Messages != 0 {
		t.Fatalf("failed cross-context launch left rows: tasks=%d sessions=%d messages=%d", result.Tasks, result.Sessions, result.Messages)
	}
}

func assertCounts(t *testing.T, db *gorm.DB, taskID, sessionID, runID string, runs, messages, events int64) {
	t.Helper()
	var result struct{ Runs, Messages, Events int64 }
	if err := db.Raw(`SELECT
		(SELECT count(*) FROM runs WHERE session_id = ?) AS runs,
		(SELECT count(*) FROM session_messages WHERE session_id = ?) AS messages,
		(SELECT count(*) FROM run_events WHERE run_id = ?) AS events`, sessionID, sessionID, runID).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.Runs != runs || result.Messages != messages || result.Events != events {
		t.Fatalf("counts for task %s = (%d,%d,%d), want (%d,%d,%d)", taskID, result.Runs, result.Messages, result.Events, runs, messages, events)
	}
}

func seedFixture(t *testing.T, db *gorm.DB) (organizationID, teamID, userID, agentID, releaseID string) {
	t.Helper()
	organizationID, teamID, userID, agentID, releaseID = uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	providerID, sshCredentialID, buildCredentialID, modelCredentialID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	modelID, runtimeID, bindingID, draftID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	suffix := fmt.Sprintf("collaboration-%d", time.Now().UnixNano())
	digest := fmt.Sprintf("registry.example/agent-platform/codex@sha256:%064x", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO organizations (id, slug, name) VALUES (?, ?, ?)`, []any{organizationID, suffix, suffix}},
		{`INSERT INTO teams (id, organization_id, slug, name) VALUES (?, ?, ?, ?)`, []any{teamID, organizationID, suffix, suffix}},
		{`INSERT INTO users (id, organization_id, oidc_subject, email, display_name) VALUES (?, ?, ?, ?, 'User')`, []any{userID, organizationID, suffix, suffix + "@example.test"}},
		{`INSERT INTO source_control_providers (id, organization_id, name, kind, base_url) VALUES (?, ?, ?, 'github_com', 'https://github.com')`, []any{providerID, organizationID, suffix}},
		{`INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES (?, ?, ?, ?, 'git_ssh', 'secret://git')`, []any{sshCredentialID, organizationID, teamID, suffix + "-ssh"}},
		{`INSERT INTO credential_profiles (id, organization_id, team_id, name, kind, secret_ref) VALUES (?, ?, ?, ?, 'build', 'secret://build')`, []any{buildCredentialID, organizationID, teamID, suffix + "-build"}},
		{`INSERT INTO credential_profiles (id, organization_id, name, kind, secret_ref) VALUES (?, ?, ?, 'model', 'secret://model')`, []any{modelCredentialID, organizationID, suffix + "-model"}},
		{`INSERT INTO configured_models (id, organization_id, name, model_id, endpoint, credential_profile_id) VALUES (?, ?, ?, 'model-a', 'https://model.example.test', ?)`, []any{modelID, organizationID, suffix, modelCredentialID}},
		{`INSERT INTO runtime_images (id, organization_id, runtime, cli_version, adapter_version, image_digest, capabilities, status, conformance_evidence_key, conformance_evidence_sha256) VALUES (?, ?, 'codex', '1', '1', ?, '{}', 'production', 'test/collaboration/evidence.tar', ?)`, []any{runtimeID, organizationID, digest, strings.Repeat("e", 64)}},
		{`INSERT INTO repository_bindings (id, organization_id, team_id, source_control_provider_id, name, repository_ssh_url, default_branch, ssh_credential_profile_id, build_credential_profile_ids, git_author_name, git_author_email, allowed_runtime_image_ids, default_runtime_image_id, default_model_id, model_budget, instructions, quality_commands, egress_policy, validation_report, validated_at) VALUES (?, ?, ?, ?, ?, 'git@github.com:acme/repository.git', 'main', ?, ?::jsonb, 'Agent', 'agent@example.test', ?::jsonb, ?, ?, '{"max_input_tokens":2000,"max_output_tokens":1000,"max_cost_amount":"20.00"}', '', '[]', '{"mode":"public"}', ?::jsonb, ?)`, []any{bindingID, organizationID, teamID, providerID, suffix, sshCredentialID, `["` + buildCredentialID + `"]`, `["` + runtimeID + `"]`, runtimeID, modelID, `{"valid":true,"errors":{},"checked_at":"` + now + `"}`, now}},
		{`INSERT INTO agents (id, organization_id, team_id, name, created_by) VALUES (?, ?, ?, ?, ?)`, []any{agentID, organizationID, teamID, suffix, userID}},
		{`INSERT INTO agent_drafts (id, agent_id, revision, state, configuration, release_risk, validation_report, created_by) VALUES (?, ?, 1, 'ready', '{}', 'low', ?::jsonb, ?)`, []any{draftID, agentID, `{"valid":true,"errors":{},"checked_at":"` + now + `"}`, userID}},
		{`INSERT INTO agent_releases (id, agent_id, release_number, source_draft_id, runtime_image_id, configured_model_id, repository_binding_id, configuration_snapshot, model_budget, execution_limits, release_risk, repository_binding_snapshot, runtime_image_snapshot, configured_model_snapshot, status, released_by) VALUES (?, ?, 1, ?, ?, ?, ?, '{}', '{"max_input_tokens":1000,"max_output_tokens":500,"max_cost_amount":"10.00"}', '{"timeout_seconds":1800,"cpus":2,"memory_bytes":4294967296,"pids":256,"temp_bytes":10737418240,"egress":"public"}', 'low', ?::jsonb, ?::jsonb, ?::jsonb, 'released', ?)`, []any{
			releaseID, agentID, draftID, runtimeID, modelID, bindingID,
			`{"id":"` + bindingID + `","source_control_provider_id":"` + providerID + `","name":"Repository","repository_ssh_url":"git@github.com:acme/repository.git","default_branch":"main","ssh_credential_profile_id":"` + sshCredentialID + `","build_credential_profile_ids":["` + buildCredentialID + `"],"git_author_name":"Agent","git_author_email":"agent@example.test","instructions":"","quality_commands":[],"egress_policy":"public","required_runtime_capabilities":[]}`,
			`{"id":"` + runtimeID + `","runtime":"codex","cli_version":"1","adapter_version":"1","image_digest":"` + digest + `","capabilities":{}}`,
			`{"id":"` + modelID + `","name":"Model","model_id":"model-a","endpoint":"https://model.example.test","credential_profile_id":"` + modelCredentialID + `"}`,
			userID,
		}},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.query, statement.args...).Error; err != nil {
			t.Fatalf("seed Collaboration fixture: %v", err)
		}
	}
	return
}

func openIntegrationDatabase(t *testing.T) *gormdb.Database {
	t.Helper()
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("EXECUTION_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2, ConnectionMaxIdle: time.Minute, ConnectionMaxLife: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
