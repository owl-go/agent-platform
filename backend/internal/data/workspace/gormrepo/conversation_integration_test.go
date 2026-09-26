package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/infrastructure/gormdb"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Every invocation gets a disposable database; the supplied database is only
// used to create/drop it. This also exercises the complete migration chain.
func conversationTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("WORKSPACE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WORKSPACE_TEST_POSTGRES_DSN is not set")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })
	database := "conversation_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = admin.Exec(`CREATE DATABASE "` + database + `"`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec(`DROP DATABASE "` + database + `" WITH (FORCE)`).Error; err != nil {
			t.Error(err)
		}
	})
	parsed.Path = "/" + database
	db, err := gorm.Open(postgres.Open(parsed.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = gormdb.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestConversationSelectionPersistenceAndFollowUpIsolation(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	repository := New(db, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, other, session, second, connection, model, skill, expert := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id)
	}
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','test')`, connection, owner)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'test')`, connection)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model','Model')`, model, connection)
	defaults, _ := json.Marshal(map[string]string{"codex": model})
	exec(`INSERT INTO personal_settings(user_id,runtime_model_defaults) VALUES(?,?::jsonb)`, owner, string(defaults))
	for _, id := range []string{session, second} {
		exec(`INSERT INTO sessions(id,owner_user_id) VALUES(?,?)`, id, owner)
	}
	exec(`INSERT INTO skills(id,owner_user_id,name,source,object_key,sha256) VALUES(?,?,'PDF','upload','skills/v1.zip',?)`, skill, owner, strings.Repeat("a", 64))
	skillIDs, _ := json.Marshal([]string{skill})
	exec(`INSERT INTO experts(id,owner_user_id,name,introduction,core_capability,operating_procedure,output_standard,skill_ids) VALUES(?,?,'Reviewer','Intro','Review','Steps','Report',?::jsonb)`, expert, owner, string(skillIDs))
	scope := domain.ConversationScope{SessionID: session}
	selected, err := repository.ResolveConversationSelection(ctx, owner, scope, domain.ConversationSelectionInput{ChangeExpert: true, ExpertID: expert, SkillIDs: []string{skill}})
	if err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE skills SET object_key='skills/v2.zip', sha256=?,version=2 WHERE id=?`, strings.Repeat("b", 64), skill)
	kept, err := repository.ResolveConversationSelection(ctx, owner, scope, domain.ConversationSelectionInput{PreviousID: selected.ID, SkillIDs: []string{skill}})
	if err != nil {
		t.Fatal(err)
	}
	if kept.Skills[0].SHA256 != strings.Repeat("a", 64) {
		t.Fatal("retained Skill silently changed revision")
	}
	refreshed, err := repository.ResolveConversationSelection(ctx, owner, scope, domain.ConversationSelectionInput{PreviousID: kept.ID, SkillIDs: []string{skill}, RefreshIDs: []string{"skill:" + skill}})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Skills[0].SHA256 != strings.Repeat("b", 64) || refreshed.Defaults[0].Skills[0].SHA256 != strings.Repeat("a", 64) {
		t.Fatal("explicit reselection changed inherited revision")
	}
	if _, err = repository.ResolveConversationSelection(ctx, owner, domain.ConversationScope{SessionID: second}, domain.ConversationSelectionInput{PreviousID: refreshed.ID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-conversation selection accepted: %v", err)
	}
	if _, err = repository.GetConversationSelection(ctx, other, scope); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-user selection accepted: %v", err)
	}
	userMessage, assistant, err := repository.CreateSelectedMessagePair(ctx, owner, session, "Review", nil, refreshed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if assistant.ResponseSnapshot.Stages[0].Skills[0].SHA256 != strings.Repeat("b", 64) {
		t.Fatal("submitted Skill revision not frozen")
	}
	current, err := repository.GetConversationSelection(ctx, owner, scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Skills) != 0 || current.ExpertID != expert || current.Defaults[0].Skills[0].SHA256 != strings.Repeat("a", 64) {
		t.Fatal("accepted submit did not retain only persistent selection")
	}
	exec(`UPDATE session_messages SET state='failed' WHERE id=?`, assistant.ID)
	exec(`UPDATE personal_settings SET default_runtime_engine='claude',runtime_model_defaults='{}' WHERE user_id=?`, owner)
	next, err := repository.ResolveConversationSelection(ctx, owner, scope, domain.ConversationSelectionInput{PreviousID: current.ID, ChangeExpert: true})
	if err != nil {
		t.Fatal(err)
	}
	_, nextAssistant, err := repository.CreateSelectedMessagePair(ctx, owner, session, "Continue without expert", nil, next.ID)
	if err != nil {
		t.Fatal(err)
	}
	stage := nextAssistant.ResponseSnapshot.Stages[0]
	if stage.Expert != nil || len(stage.Skills) != 0 || stage.ProviderModel.ID != model || stage.RuntimeEngine != domain.RuntimeCodex {
		t.Fatalf("configuration/resource separation failed: %#v", stage)
	}
	_, retry, err := repository.RetryMessage(ctx, owner, session, userMessage.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ResponseSnapshot.Stages[0].Expert == nil || retry.ResponseSnapshot.Stages[0].Skills[0].SHA256 != strings.Repeat("b", 64) {
		t.Fatal("retry used current selection")
	}
	var record sessionRecord
	if err = db.Where("id = ?", session).Take(&record).Error; err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSessionSnapshot(db, record, *nextAssistant.ResponseSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stages[0].Expert != nil || len(loaded.Stages[0].Skills) != 0 {
		t.Fatal("worker reused initial specialist")
	}

	exec(`UPDATE sessions SET native_checkpoint='stale-native-context',runtime_engine='codex',rolling_summary='prior platform summary' WHERE id=?`, session)
	var job *application.ExecutionJob
	if err = db.Transaction(func(tx *gorm.DB) error { var claimErr error; job, claimErr = claimSessionMessage(tx); return claimErr }); err != nil {
		t.Fatal(err)
	}
	if job == nil || job.CheckpointRef != "" || !strings.Contains(job.Instruction, "prior platform summary") || job.Snapshot.Stages[0].Expert != nil {
		t.Fatal("dynamic selection lost platform history or reused stale native context")
	}
	exec(`UPDATE experts SET introduction='' WHERE id=?`, expert)
	exec(`UPDATE sessions SET expert_id=? WHERE id=?`, expert, second)
	// The invalid old catalog association must not prevent removal or switching.
	recovered, err := repository.ResolveConversationSelection(ctx, owner, domain.ConversationScope{SessionID: second}, domain.ConversationSelectionInput{ChangeExpert: true})
	if err != nil || recovered.ExpertID != "" {
		t.Fatalf("cannot recover unavailable selection: %v", err)
	}

	connector := uuid.NewString()
	exec(`INSERT INTO cli_connector_definitions(id,name,npm_package,npm_version,npm_integrity,executable,authentication_driver,state,bundle_sha256,created_by_user_id) VALUES(?,'CLI','example-cli','2','sha512-test','example-cli','none','available',?,?)`, connector, strings.Repeat("c", 64), owner)
	exec(`INSERT INTO cli_connector_enablements(owner_user_id,definition_id,state) VALUES(?,?,'enabled')`, owner, connector)
	checked := loaded
	checked.Stages = append([]domain.ExecutionStageSnapshot(nil), loaded.Stages...)
	checked.Stages[0].CLIConnectors = []domain.CLIConnectorSnapshot{{ID: connector, BundleSHA256: strings.Repeat("b", 64)}}
	if err = validateQueuedSnapshotAvailability(db, checked, owner); err != nil {
		t.Fatalf("updated catalog blocked retained Connector revision: %v", err)
	}
	exec(`UPDATE cli_connector_enablements SET state='disabled' WHERE definition_id=?`, connector)
	if err = validateQueuedSnapshotAvailability(db, checked, owner); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("retained Connector bypassed current disablement")
	}
	packageRevision, err := repository.CreateConnectorRevision(ctx, domain.ConnectorRevision{
		PackageSource: "package-cli", Version: "1.0.0", Mode: domain.ConnectorModeCLI,
		PackageSHA256: strings.Repeat("d", 64), ObjectKey: "connectors/package-cli/1.0.0/package.zip",
		RuntimePolicy: []byte(`{"auth_mode":"oauth","cli_bundle_object_key":"connectors/package-cli/bundle.zip","cli_bundle_sha256":"` + strings.Repeat("e", 64) + `","cli":{"executable":"package-cli","runtime":{"digest":"sha256:` + strings.Repeat("f", 64) + `"},"capabilities":[{"id":"read","argv_prefix":["read"],"risk":"low","identities":["user"],"egress_hosts":["example.com"],"timeout_seconds":30}]}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	installation, err := repository.InstallConnector(ctx, domain.ConnectorInstallation{OwnerID: owner, PackageSource: "package-cli", ActiveRevisionID: packageRevision.ID, State: domain.ConnectorInstallationActive})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	authorization, err := repository.CreateConnectorAuthorization(ctx, domain.ConnectorAuthorization{OwnerID: owner, InstallationID: installation.ID, IdentityRef: "user", CredentialCiphertext: []byte("test-only-ciphertext"), ExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	checked.Stages[0].CLIConnectors = []domain.CLIConnectorSnapshot{{ID: installation.ID}}
	if err = validateQueuedSnapshotAvailability(db, checked, owner); err != nil {
		t.Fatalf("active Connector Package was rejected by queued validation: %v", err)
	}
	exec(`UPDATE connector_authorizations SET expires_at=now()-interval '1 second' WHERE id=?`, authorization.ID)
	if err = validateQueuedSnapshotAvailability(db, checked, owner); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("expired Connector authorization bypassed queued validation")
	}
	exec(`UPDATE connector_authorizations SET expires_at=now()+interval '1 hour' WHERE id=?`, authorization.ID)
	exec(`UPDATE connector_installations SET state='disabled' WHERE id=?`, installation.ID)
	if err = validateQueuedSnapshotAvailability(db, checked, owner); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("disabled Connector Package bypassed queued validation")
	}
	workflow, run := uuid.NewString(), uuid.NewString()
	exec(`INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','mutable goal','workspace/test')`, workflow, owner)
	var plan domain.ExecutionSnapshot
	if err = json.Unmarshal(record.ExpertSnapshot, &plan); err != nil {
		t.Fatal(err)
	}
	plan.Goal = "original goal"
	encoded, _ := json.Marshal(plan)
	exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot) VALUES(?,?,1,?,?,'Workflow','manual','succeeded',?::jsonb)`, run, run, owner, workflow, string(encoded))
	runScope := domain.ConversationScope{WorkflowID: workflow, RunID: run}
	runSelection, err := repository.ResolveConversationSelection(ctx, owner, runScope, domain.ConversationSelectionInput{ChangeExpert: true})
	if err != nil {
		t.Fatal(err)
	}
	follow, err := repository.ContinueSelectedRunConversation(ctx, owner, workflow, run, "follow-up", nil, runSelection.ID)
	if err != nil {
		t.Fatal(err)
	}
	var followRow, rootRow runRecord
	if err = db.Where("id = ?", follow.ID).Take(&followRow).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Where("id = ?", run).Take(&rootRow).Error; err != nil {
		t.Fatal(err)
	}
	var followPlan domain.ExecutionSnapshot
	if err = json.Unmarshal(followRow.WorkflowSnapshot, &followPlan); err != nil {
		t.Fatal(err)
	}
	if followPlan.Goal != "original goal" || followPlan.Stages[0].Expert != nil || followPlan.Stages[0].RuntimeEngine != domain.RuntimeCodex {
		t.Fatal("follow-up changed frozen goal/configuration or kept removed Expert")
	}
	var rootPlan domain.ExecutionSnapshot
	_ = json.Unmarshal(rootRow.WorkflowSnapshot, &rootPlan)
	if rootPlan.Stages[0].Expert == nil {
		t.Fatal("follow-up rewrote initial Run")
	}
	concurrent, err := repository.ContinueSelectedRunConversation(ctx, owner, workflow, run, "concurrent", nil, runSelection.ID)
	if err != nil {
		t.Fatalf("concurrent follow-up rejected: %v", err)
	}
	if concurrent.TurnNumber != 3 {
		t.Fatalf("concurrent follow-up turn number = %d, want 3", concurrent.TurnNumber)
	}
}
