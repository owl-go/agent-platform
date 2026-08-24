package gormrepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/application"
	"agent-platform/backend/internal/biz/execution/domain"
	bizworkflow "agent-platform/backend/internal/biz/workflow"
	"agent-platform/backend/internal/data/execution/gormrepo"
	"agent-platform/backend/internal/data/workflow/gormtx"
	"agent-platform/backend/internal/infrastructure/gormdb"

	"gorm.io/gorm"
)

func TestGORMRepositoryRunLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "lifecycle")
	if err := database.ORM().Exec(`UPDATE runtime_images SET runtime = 'codex', cli_version = 'mutated', image_digest = ? WHERE id = (SELECT runtime_image_id FROM runs WHERE id = ?)`, "registry.example/mutated@sha256:"+strings.Repeat("f", 64), runID).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.ORM().Exec(`UPDATE repository_bindings SET repository_ssh_url = 'git@example.test:mutated/repository.git', git_author_name = 'Mutated' WHERE id = (SELECT session.repository_binding_id FROM runs run JOIN sessions session ON session.id = run.session_id WHERE run.id = ?)`, runID).Error; err != nil {
		t.Fatal(err)
	}
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))

	lease, found, err := service.Claim(context.Background(), "worker-a", time.Minute)
	if err != nil || !found {
		t.Fatalf("Claim() = (%+v, %v, %v)", lease, found, err)
	}
	if lease.RunID != runID || lease.AttemptNumber != 1 || lease.Token == "" || lease.RuntimeName != "claude" || lease.RuntimeCLIVersion != "test" || lease.ImageDigest == "" || lease.WorkspaceVolume == "" || lease.RepositorySSHURL != "git@github.com:example/repository.git" || lease.GitAuthorName != "Agent Platform" {
		t.Fatalf("unexpected lease: %+v", lease)
	}
	if _, found, err := service.Claim(context.Background(), "worker-b", time.Minute); err != nil || found {
		t.Fatalf("second Claim() found=%v error=%v", found, err)
	}
	if err := service.Renew(context.Background(), lease.Token, time.Minute); err != nil {
		t.Fatalf("Renew(): %v", err)
	}
	if err := service.MarkRunning(context.Background(), lease.Token); err != nil {
		t.Fatalf("MarkRunning(): %v", err)
	}
	if err := service.AppendEvent(context.Background(), lease.Token, domain.EventInput{Type: "message.completed", Payload: []byte(`{"message":"done"}`)}); err != nil {
		t.Fatalf("AppendEvent(): %v", err)
	}
	details, err := service.Get(context.Background(), runID)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if details.State != domain.Running || details.AttemptCount != 1 || len(details.Attempts) != 1 || details.Attempts[0].State != domain.AttemptRunning {
		t.Fatalf("Run details = %+v", details)
	}
	if err := service.Finish(context.Background(), lease.Token, domain.Outcome{
		State: domain.Completed, Usage: []byte(`{"input_tokens":12}`), Cost: "0.125",
	}); err != nil {
		t.Fatalf("Finish(): %v", err)
	}
	assertRun(t, database.ORM(), runID, domain.Completed, 1, 0)
	var taskProjection struct {
		State    string
		Messages int64
	}
	if err := database.ORM().Raw(`
		SELECT task.state, (SELECT count(*) FROM session_messages message WHERE message.session_id = session.id AND message.run_id = ?) AS messages
		FROM runs run JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id WHERE run.id = ?`, runID, runID).Scan(&taskProjection).Error; err != nil {
		t.Fatal(err)
	}
	if taskProjection.State != "waiting_for_user" || taskProjection.Messages != 1 {
		t.Fatalf("completed Run projection = %+v", taskProjection)
	}
	assertEventSequence(t, database.ORM(), runID, []string{"run.attempt_started", "run.running", "message.completed", "run.completed"})
	if err := service.Renew(context.Background(), lease.Token, time.Minute); err != domain.ErrLeaseLost {
		t.Fatalf("renew released lease error = %v, want ErrLeaseLost", err)
	}
}

func TestFailedRunReturnsCodingTaskToUserWithoutClosingIt(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "failed-continuation")
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))

	lease, found, err := service.Claim(context.Background(), "worker-failed", time.Minute)
	if err != nil || !found || lease.RunID != runID {
		t.Fatalf("Claim() = (%+v, %v, %v)", lease, found, err)
	}
	if err := service.MarkRunning(context.Background(), lease.Token); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(context.Background(), lease.Token, domain.Outcome{State: domain.Failed, Error: []byte(`{"code":"runtime_failed"}`)}); err != nil {
		t.Fatal(err)
	}

	var projection struct {
		TaskState   string
		CompletedAt *time.Time
		Status      string
	}
	if err := database.ORM().Raw(`
		SELECT task.state AS task_state, task.completed_at,
		       message.content->>'status' AS status
		FROM runs run
		JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id
		JOIN session_messages message ON message.session_id = session.id AND message.run_id = run.id
		WHERE run.id = ? AND message.content->>'type' = 'run_result'`, runID).Scan(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if projection.TaskState != "waiting_for_user" || projection.CompletedAt != nil || projection.Status != "failed" {
		t.Fatalf("failed Run task projection = %+v", projection)
	}
}

func TestGORMRepositoryGetMissingRun(t *testing.T) {
	database := openIntegrationDatabase(t)
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))
	if _, err := service.Get(context.Background(), "6ba7b810-9dad-11d1-80b4-00c04fd430c8"); err != domain.ErrRunNotFound {
		t.Fatalf("Get missing Run error = %v, want ErrRunNotFound", err)
	}
}

func TestGORMRepositorySearchesRunsWithinTeamScope(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "run-search")
	var scope struct {
		OrganizationID string
		TeamID         string
		AgentID        string
	}
	if err := database.ORM().Raw(`
		SELECT task.organization_id::text, task.team_id::text, release.agent_id::text
		FROM runs run
		JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id
		JOIN agent_releases release ON release.id = run.agent_release_id
		WHERE run.id = ?`, runID).Scan(&scope).Error; err != nil {
		t.Fatal(err)
	}
	service := application.New(gormrepo.New(database.ORM()))
	results, err := service.Search(context.Background(), domain.SearchQuery{OrganizationID: scope.OrganizationID, TeamID: scope.TeamID, AgentID: scope.AgentID, State: domain.Queued, Runtime: "claude", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, result := range results {
		found = found || result.ID == runID
	}
	if !found {
		t.Fatalf("seeded Run %s was not returned by scoped search", runID)
	}
	for _, result := range results {
		if result.ID == runID && (result.CodingTaskID == "" || result.AgentID == "" || result.RepositoryBindingID == "" || len(result.RuntimeImage) == 0 || len(result.ConfiguredModel) == 0) {
			t.Fatalf("Run diagnostics are incomplete: %+v", result)
		}
	}
	results, err = service.Search(context.Background(), domain.SearchQuery{OrganizationID: scope.OrganizationID, TeamID: "6ba7b810-9dad-11d1-80b4-00c04fd430c8", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.ID == runID {
			t.Fatal("cross-Team Run search returned the seeded Run")
		}
	}
}

func TestGORMRepositoryRunControlLifecycle(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "run-control")
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))
	lease, found, err := service.Claim(context.Background(), "control-worker", time.Minute)
	if err != nil || !found || lease.RunID != runID {
		t.Fatalf("Claim() = (%+v, %v, %v)", lease, found, err)
	}
	if err := service.MarkRunning(context.Background(), lease.Token); err != nil {
		t.Fatal(err)
	}
	details, err := service.Get(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	details, err = service.Control(context.Background(), runID, details.Version, domain.ControlInterrupt, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "")
	if err != nil || details.State != domain.Interrupting {
		t.Fatalf("Interrupt() = (%+v, %v)", details, err)
	}
	if err := service.Renew(context.Background(), lease.Token, time.Minute); !errors.Is(err, domain.ErrInterruptionRequested) {
		t.Fatalf("Renew() error = %v, want ErrInterruptionRequested", err)
	}
	if err := service.Finish(context.Background(), lease.Token, domain.Outcome{State: domain.Cancelled}); err != nil {
		t.Fatal(err)
	}
	details, err = service.Get(context.Background(), runID)
	if err != nil || details.State != domain.Interrupted || details.Attempts[0].State != domain.AttemptCancelled {
		t.Fatalf("interrupted Run = (%+v, %v)", details, err)
	}
	details, err = service.Control(context.Background(), runID, details.Version, domain.ControlResume, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "")
	if err != nil || details.State != domain.Resuming {
		t.Fatalf("Resume() = (%+v, %v)", details, err)
	}
	resumed, found, err := service.Claim(context.Background(), "resume-worker", time.Minute)
	if err != nil || !found || resumed.RunID != runID {
		t.Fatalf("resumed Claim() = (%+v, %v, %v)", resumed, found, err)
	}
	if err := service.MarkRunning(context.Background(), resumed.Token); err != nil {
		t.Fatal(err)
	}
	details, _ = service.Get(context.Background(), runID)
	details, err = service.Control(context.Background(), runID, details.Version, domain.ControlKill, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "emergency containment")
	if err != nil || details.State != domain.Cancelled || !strings.Contains(string(details.TerminalError), "operator_killed") {
		t.Fatalf("Kill() = (%+v, %v)", details, err)
	}
	if err := service.Renew(context.Background(), resumed.Token, time.Minute); !errors.Is(err, domain.ErrLeaseLost) {
		t.Fatalf("killed Run Renew() error = %v, want ErrLeaseLost", err)
	}
	assertTaskRunResult(t, database.ORM(), runID, "killed")
}

func TestGORMRepositoryClaimsTenIndependentSessionsConcurrently(t *testing.T) {
	database := openIntegrationDatabase(t)
	const concurrency = 10
	for index := 0; index < concurrency; index++ {
		seedRun(t, database.ORM(), fmt.Sprintf("capacity-%02d", index))
	}
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))
	start := make(chan struct{})
	type claimResult struct {
		lease domain.Lease
		found bool
		err   error
	}
	results := make(chan claimResult, concurrency)
	var workers sync.WaitGroup
	for index := 0; index < concurrency; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			lease, found, err := service.Claim(context.Background(), fmt.Sprintf("capacity-worker-%02d", index), time.Minute)
			results <- claimResult{lease: lease, found: found, err: err}
		}(index)
	}
	startedAt := time.Now()
	close(start)
	workers.Wait()
	if elapsed := time.Since(startedAt); elapsed >= 60*time.Second {
		t.Fatalf("10 concurrent Runs took %s to claim, want less than 60s", elapsed)
	}
	close(results)
	claimed := make(map[string]struct{}, concurrency)
	for result := range results {
		if result.err != nil || !result.found {
			t.Fatalf("concurrent Claim() = (%+v, %v, %v)", result.lease, result.found, result.err)
		}
		if _, duplicate := claimed[result.lease.RunID]; duplicate {
			t.Fatalf("Run %s was claimed more than once", result.lease.RunID)
		}
		claimed[result.lease.RunID] = struct{}{}
	}
	if len(claimed) != concurrency {
		t.Fatalf("claimed %d Runs, want %d", len(claimed), concurrency)
	}
}

func TestGORMRepositoryCancelBeginsWithinTenSeconds(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "cancel-slo")
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))
	lease, found, err := service.Claim(context.Background(), "cancel-slo-worker", time.Minute)
	if err != nil || !found || lease.RunID != runID {
		t.Fatalf("Claim() = (%+v, %v, %v)", lease, found, err)
	}
	if err := service.MarkRunning(context.Background(), lease.Token); err != nil {
		t.Fatal(err)
	}
	details, err := service.Get(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now()
	details, err = service.Control(context.Background(), runID, details.Version, domain.ControlCancel, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "")
	if err != nil || details.State != domain.Cancelled {
		t.Fatalf("Cancel() = (%+v, %v)", details, err)
	}
	if elapsed := time.Since(startedAt); elapsed >= 10*time.Second {
		t.Fatalf("Cancel took %s to revoke execution ownership, want less than 10s", elapsed)
	}
	if err := service.Renew(context.Background(), lease.Token, time.Minute); !errors.Is(err, domain.ErrLeaseLost) {
		t.Fatalf("cancelled Run Renew() error = %v, want ErrLeaseLost", err)
	}
	assertTaskRunResult(t, database.ORM(), runID, "cancelled")
}

func TestGORMRepositorySerializesWorkspaceWritersPerSession(t *testing.T) {
	database := openIntegrationDatabase(t)
	firstRunID := seedRun(t, database.ORM(), "workspace-writer")
	var secondRunID string
	if err := database.ORM().Raw(`
		INSERT INTO runs (session_id, agent_release_id, runtime_image_id, model_binding, credential_bindings, request_text, model_budget, execution_limits, created_by)
		SELECT session_id, agent_release_id, runtime_image_id, model_binding, credential_bindings, 'Second write', model_budget, execution_limits, created_by
		FROM runs WHERE id = ? RETURNING id`, firstRunID).Scan(&secondRunID).Error; err != nil {
		t.Fatal(err)
	}
	registerRunCleanup(t, database.ORM(), secondRunID)
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))
	first, found, err := service.Claim(context.Background(), "worker-a", time.Minute)
	if err != nil || !found || first.RunID != firstRunID {
		t.Fatalf("first Claim() = (%+v, %v, %v)", first, found, err)
	}
	if _, found, err := service.Claim(context.Background(), "worker-b", time.Minute); err != nil || found {
		t.Fatalf("concurrent Session Claim() found=%v error=%v", found, err)
	}
	if err := service.MarkRunning(context.Background(), first.Token); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(context.Background(), first.Token, domain.Outcome{State: domain.Completed, Usage: []byte(`{}`), Cost: "0"}); err != nil {
		t.Fatal(err)
	}
	second, found, err := service.Claim(context.Background(), "worker-b", time.Minute)
	if err != nil || !found || second.RunID != secondRunID {
		t.Fatalf("second Claim() after release = (%+v, %v, %v)", second, found, err)
	}
	if err := service.MarkRunning(context.Background(), second.Token); err != nil {
		t.Fatal(err)
	}
	if err := service.Finish(context.Background(), second.Token, domain.Outcome{State: domain.Completed, Usage: []byte(`{}`), Cost: "0"}); err != nil {
		t.Fatal(err)
	}
}

func TestGORMRepositoryReconcilesExpiredAttempts(t *testing.T) {
	database := openIntegrationDatabase(t)
	runID := seedRun(t, database.ORM(), "reconcile")
	service := application.New(gormrepo.New(database.ORM()), bizworkflow.NewCompletion(gormtx.New(database.ORM())))

	first, found, err := service.Claim(context.Background(), "worker-a", time.Microsecond)
	if err != nil || !found {
		t.Fatalf("first Claim() found=%v error=%v", found, err)
	}
	time.Sleep(10 * time.Millisecond)
	reconcileStartedAt := time.Now()
	result, err := service.ReconcileExpired(context.Background(), 2)
	if err != nil || result.Rescheduled != 1 || result.Failed != 0 {
		t.Fatalf("first ReconcileExpired() = (%+v, %v)", result, err)
	}
	if elapsed := time.Since(reconcileStartedAt); elapsed >= 5*time.Minute {
		t.Fatalf("expired lease reconciliation took %s, want less than five minutes", elapsed)
	}
	assertRun(t, database.ORM(), runID, domain.Resuming, 1, 0)

	second, found, err := service.Claim(context.Background(), "worker-b", time.Microsecond)
	if err != nil || !found || second.AttemptNumber != 2 || second.Token == first.Token {
		t.Fatalf("second Claim() = (%+v, %v, %v)", second, found, err)
	}
	time.Sleep(10 * time.Millisecond)
	result, err = service.ReconcileExpired(context.Background(), 2)
	if err != nil || result.Rescheduled != 0 || result.AwaitingRecovery != 1 || result.Failed != 0 {
		t.Fatalf("second ReconcileExpired() = (%+v, %v)", result, err)
	}
	assertRun(t, database.ORM(), runID, domain.RecoveryRequired, 2, 0)
	assertEventSequence(t, database.ORM(), runID, []string{
		"run.attempt_started", "run.retry_scheduled", "run.attempt_started", "run.recovery_required",
	})
	details, err := service.Get(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	details, err = service.Control(context.Background(), runID, details.Version, domain.ControlRecover, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "worker capacity restored")
	if err != nil || details.State != domain.Resuming || details.TerminalError != nil {
		t.Fatalf("Recover() = (%+v, %v)", details, err)
	}
	third, found, err := service.Claim(context.Background(), "worker-c", time.Minute)
	if err != nil || !found || third.RunID != runID || third.AttemptNumber != 3 {
		t.Fatalf("recovered Claim() = (%+v, %v, %v)", third, found, err)
	}
}

func assertTaskRunResult(t *testing.T, database *gorm.DB, runID, status string) {
	t.Helper()
	var projection struct {
		TaskState string
		Status    string
	}
	if err := database.Raw(`
		SELECT task.state AS task_state, message.content->>'status' AS status
		FROM runs run
		JOIN sessions session ON session.id = run.session_id
		JOIN coding_tasks task ON task.id = session.coding_task_id
		JOIN session_messages message ON message.session_id = session.id AND message.run_id = run.id
		WHERE run.id = ? AND message.content->>'type' = 'run_result'`, runID).Scan(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if projection.TaskState != "waiting_for_user" || projection.Status != status {
		t.Fatalf("Run %s task projection = %+v, want waiting_for_user/%s", runID, projection, status)
	}
}

func openIntegrationDatabase(t *testing.T) *gormdb.Database {
	t.Helper()
	dsn := os.Getenv("EXECUTION_DATABASE_DSN")
	if dsn == "" {
		t.Skip("EXECUTION_DATABASE_DSN is required for PostgreSQL integration")
	}
	database, err := gormdb.Open(context.Background(), gormdb.Config{
		DSN: dsn, MaxOpenConnections: 5, MaxIdleConnections: 2,
		ConnectionMaxIdle: time.Minute, ConnectionMaxLife: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func seedRun(t *testing.T, db *gorm.DB, label string) string {
	t.Helper()
	suffix := fmt.Sprintf("%s-%d", label, time.Now().UnixNano())
	digestBytes := sha256.Sum256([]byte(suffix))
	digest := "registry.example/agent-platform/claude@sha256:" + hex.EncodeToString(digestBytes[:])
	var result struct {
		ID string `gorm:"column:id"`
	}
	query := `
		WITH org AS (
			INSERT INTO organizations (slug, name) VALUES (?, ?) RETURNING id
		), team AS (
			INSERT INTO teams (organization_id, slug, name) SELECT id, 'platform', 'Platform' FROM org RETURNING id, organization_id
		), platform_user AS (
			INSERT INTO users (organization_id, oidc_subject, email, display_name)
			SELECT organization_id, ?, ? || '@example.test', 'Test User' FROM team RETURNING id, organization_id
		), model_credential AS (
			INSERT INTO credential_profiles (organization_id, name, kind, secret_ref)
			SELECT organization_id, 'model', 'model', 'secret://model' FROM platform_user RETURNING id, organization_id
		), git_credential AS (
			INSERT INTO credential_profiles (organization_id, name, kind, secret_ref)
			SELECT organization_id, 'git', 'git_ssh', 'secret://git' FROM platform_user RETURNING id
		), model AS (
			INSERT INTO configured_models (organization_id, name, model_id, endpoint, credential_profile_id)
			SELECT organization_id, 'configured-model', 'model-id', 'https://models.example.test', id FROM model_credential RETURNING id, credential_profile_id
		), runtime AS (
			INSERT INTO runtime_images (organization_id, runtime, cli_version, adapter_version, image_digest)
			SELECT organization_id, 'claude', 'test', 'test', ? FROM platform_user RETURNING id, runtime, cli_version, adapter_version, image_digest, capabilities
		), provider AS (
			INSERT INTO source_control_providers (organization_id, name, kind, base_url)
			SELECT organization_id, 'github', 'github_com', 'https://github.com' FROM platform_user RETURNING id
		), binding AS (
			INSERT INTO repository_bindings (
				organization_id, team_id, source_control_provider_id, name, repository_ssh_url,
				default_branch, ssh_credential_profile_id, git_author_name, git_author_email,
				allowed_runtime_image_ids, default_runtime_image_id, default_model_id
			)
			SELECT t.organization_id, t.id, p.id, 'repository', 'git@github.com:example/repository.git',
			       'main', gc.id, 'Agent Platform', 'agent@example.test', jsonb_build_array(r.id), r.id, m.id
			FROM team t CROSS JOIN provider p CROSS JOIN git_credential gc CROSS JOIN runtime r CROSS JOIN model m RETURNING id
		), agent AS (
			INSERT INTO agents (organization_id, team_id, name, created_by)
			SELECT t.organization_id, t.id, 'Coding Agent', u.id FROM team t CROSS JOIN platform_user u RETURNING id
		), draft AS (
			INSERT INTO agent_drafts (agent_id, revision, state, configuration, created_by)
			SELECT a.id, 1, 'ready', '{}'::jsonb, u.id FROM agent a CROSS JOIN platform_user u RETURNING id, agent_id
		), release AS (
			INSERT INTO agent_releases (
				agent_id, release_number, source_draft_id, runtime_image_id, configured_model_id,
				repository_binding_id, configuration_snapshot, model_budget, execution_limits, release_risk,
				repository_binding_snapshot, runtime_image_snapshot, configured_model_snapshot, released_by
			)
			SELECT d.agent_id, 1, d.id, r.id, m.id, b.id, '{}'::jsonb,
			       '{"amount":10}'::jsonb, '{"duration_seconds":1800}'::jsonb, 'low',
			       jsonb_build_object('id', b.id, 'source_control_provider_id', p.id, 'name', 'repository',
			         'repository_ssh_url', 'git@github.com:example/repository.git', 'default_branch', 'main',
			         'ssh_credential_profile_id', gc.id, 'build_credential_profile_ids', '[]'::jsonb,
			         'git_author_name', 'Agent Platform', 'git_author_email', 'agent@example.test',
			         'instructions', '', 'quality_commands', '[]'::jsonb, 'egress_policy', 'public', 'required_runtime_capabilities', '[]'::jsonb),
			       jsonb_build_object('id', r.id, 'runtime', r.runtime, 'cli_version', r.cli_version,
			         'adapter_version', r.adapter_version, 'image_digest', r.image_digest, 'capabilities', r.capabilities),
			       jsonb_build_object('id', m.id, 'name', 'configured-model', 'model_id', 'model-id',
			         'endpoint', 'https://models.example.test', 'credential_profile_id', m.credential_profile_id), u.id
			FROM draft d CROSS JOIN runtime r CROSS JOIN model m CROSS JOIN binding b CROSS JOIN provider p
			CROSS JOIN git_credential gc CROSS JOIN platform_user u RETURNING id
		), task AS (
			INSERT INTO coding_tasks (organization_id, team_id, agent_release_id, created_by, title, request_text, state)
			SELECT t.organization_id, t.id, r.id, u.id, 'Test Task', 'Change the fixture', 'active'
			FROM team t CROSS JOIN release r CROSS JOIN platform_user u RETURNING id
		), session AS (
			INSERT INTO sessions (coding_task_id, repository_binding_id, target_branch, review_branch, workspace_volume)
			SELECT task.id, b.id, 'main', 'agent/' || ?, 'workspace-' || ? FROM task CROSS JOIN binding b RETURNING id
		)
		INSERT INTO runs (
			session_id, agent_release_id, runtime_image_id, model_binding, request_text,
			model_budget, execution_limits, created_by
		)
		SELECT s.id, rel.id, runtime.id, '{"model_id":"model-id","endpoint":"https://models.example.test","credential_profile_id":"model-credential"}'::jsonb, 'Change the fixture',
		       '{"amount":10}'::jsonb, '{"duration_seconds":1800}'::jsonb, u.id
		FROM session s CROSS JOIN release rel CROSS JOIN runtime CROSS JOIN platform_user u
		RETURNING id::text AS id`
	if err := db.Raw(query, suffix, suffix, suffix, suffix, digest, suffix, suffix).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	registerRunCleanup(t, db, result.ID)
	return result.ID
}

func registerRunCleanup(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	t.Cleanup(func() {
		_ = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`DELETE FROM workspace_write_leases WHERE run_id = ?`, runID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`DELETE FROM run_leases WHERE run_id = ?`, runID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`UPDATE run_attempts SET state = 'cancelled', ended_at = COALESCE(ended_at, now()) WHERE run_id = ? AND state IN ('provisioning', 'running')`, runID).Error; err != nil {
				return err
			}
			return tx.Exec(`UPDATE runs SET state = 'cancelled', ended_at = COALESCE(ended_at, now()), updated_at = now(), version = version + 1 WHERE id = ? AND state NOT IN ('completed', 'failed', 'cancelled')`, runID).Error
		})
	})
}

func assertRun(t *testing.T, db *gorm.DB, runID string, state domain.State, attempts, leases int) {
	t.Helper()
	var result struct {
		State    string `gorm:"column:state"`
		Attempts int    `gorm:"column:attempts"`
		Leases   int    `gorm:"column:leases"`
	}
	query := `
		SELECT r.state, r.attempt_count AS attempts,
		       (SELECT count(*) FROM run_leases l WHERE l.run_id = r.id) AS leases
		FROM runs r WHERE r.id = ?::uuid`
	if err := db.Raw(query, runID).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.State != string(state) || result.Attempts != attempts || result.Leases != leases {
		t.Fatalf("Run = (state=%s attempts=%d leases=%d), want (%s, %d, %d)", result.State, result.Attempts, result.Leases, state, attempts, leases)
	}
}

func assertEventSequence(t *testing.T, db *gorm.DB, runID string, want []string) {
	t.Helper()
	var records []struct {
		Sequence  int64  `gorm:"column:sequence"`
		EventType string `gorm:"column:event_type"`
	}
	if err := db.Raw(`SELECT sequence, event_type FROM run_events WHERE run_id = ?::uuid ORDER BY sequence`, runID).Scan(&records).Error; err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(records))
	for index, record := range records {
		if record.Sequence != int64(index+1) {
			t.Fatalf("event sequence = %d, want %d", record.Sequence, index+1)
		}
		got = append(got, record.EventType)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
}
