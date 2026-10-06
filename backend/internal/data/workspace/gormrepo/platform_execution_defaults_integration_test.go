package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func TestPlatformExecutionDefaultSavesWithoutRunAndPropagates(t *testing.T) {
	for _, verificationStatus := range []string{"verified", "unverified"} {
		t.Run(verificationStatus, func(t *testing.T) {
			testPlatformExecutionDefaultSavesWithoutRunAndPropagates(t, verificationStatus)
		})
	}
}

func testPlatformExecutionDefaultSavesWithoutRunAndPropagates(t *testing.T, verificationStatus string) {
	t.Helper()
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	administrator, userID := uuid.NewString(), uuid.NewString()
	connectionID, modelID := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name,administrator) VALUES(?,?,?,?,?,true)", administrator, administrator, administrator, administrator+"@example.test", administrator)
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", userID, userID, userID, userID+"@example.test", userID)
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext,verification_status) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','ciphertext',?)`, connectionID, administrator, verificationStatus)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'ciphertext')`, connectionID)
	compatibility, _ := json.Marshal([]domain.RuntimeModelCompatibility{{RuntimeEngine: domain.RuntimeCodex, Status: "unverified"}})
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name,compatibility) VALUES(?,?,'model-1','Model',?::jsonb)`, modelID, connectionID, string(compatibility))
	defaults, _ := json.Marshal(map[string]string{"codex": modelID})
	exec(`INSERT INTO personal_settings(user_id,default_runtime_engine,runtime_model_defaults,execution_inherited) VALUES(?,'codex',?::jsonb,false)`, administrator, string(defaults))
	exec(`INSERT INTO personal_settings(user_id) VALUES(?)`, userID)
	for _, test := range []struct {
		name    string
		prepare func()
		restore func()
	}{
		{
			name:    "missing API Key",
			prepare: func() { exec(`UPDATE model_provider_connections SET api_key_ciphertext='' WHERE id=?`, connectionID) },
			restore: func() {
				exec(`UPDATE model_provider_connections SET api_key_ciphertext='ciphertext' WHERE id=?`, connectionID)
			},
		},
		{
			name:    "unavailable model",
			prepare: func() { exec(`UPDATE provider_models SET available=false WHERE id=?`, modelID) },
			restore: func() { exec(`UPDATE provider_models SET available=true WHERE id=?`, modelID) },
		},
		{
			name: "incompatible pair",
			prepare: func() {
				exec(`UPDATE provider_models SET compatibility='[{"runtime_engine":"codex","status":"incompatible"}]'::jsonb WHERE id=?`, modelID)
			},
			restore: func() {
				exec(`UPDATE provider_models SET compatibility=?::jsonb WHERE id=?`, string(compatibility), modelID)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.prepare()
			defer test.restore()
			if _, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, 0); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("%s error = %v", test.name, err)
			}
			if _, err := repository.GetPlatformExecutionDefault(ctx); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("rejected update created a default: %v", err)
			}
		})
	}

	created, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.ValidationRunID != "" {
		t.Fatalf("created default = %#v", created)
	}
	var persistedConnection modelProviderConnectionRecord
	if err := db.Where("id = ?", connectionID).Take(&persistedConnection).Error; err != nil {
		t.Fatal(err)
	}
	if persistedConnection.VerificationStatus != verificationStatus {
		t.Fatalf("validation changed provider status to %q", persistedConnection.VerificationStatus)
	}
	var promoted struct {
		Compatibility []byte `gorm:"column:compatibility"`
	}
	if err := db.Table("provider_models").Select("compatibility").Where("id = ?", modelID).Scan(&promoted).Error; err != nil {
		t.Fatal(err)
	}
	var promotedCompatibility []domain.RuntimeModelCompatibility
	if err := json.Unmarshal(promoted.Compatibility, &promotedCompatibility); err != nil || len(promotedCompatibility) != 1 || promotedCompatibility[0].Status != "unverified" {
		t.Fatalf("unexpected compatibility mutation = %#v, %v", promotedCompatibility, err)
	}
	inherited, err := repository.GetSettings(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !inherited.ExecutionInherited || inherited.DefaultRuntimeEngine != domain.RuntimeCodex || inherited.RuntimeModelDefaults[domain.RuntimeCodex] != modelID {
		t.Fatalf("inherited settings = %#v", inherited)
	}
	newUser := uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", newUser, newUser, newUser, newUser+"@example.test", newUser)
	exec(`INSERT INTO personal_settings(user_id) VALUES(?)`, newUser)
	newSettings, err := repository.GetSettings(ctx, newUser)
	if err != nil || newSettings.RuntimeModelDefaults[domain.RuntimeCodex] != modelID {
		t.Fatalf("new User inheritance = %#v, %v", newSettings, err)
	}
	session, err := repository.CreateSession(ctx, newUser, nil, nil)
	if err != nil {
		t.Fatal("new User could not create a Session", err)
	}
	_, firstReply, err := repository.CreatePlannedMessagePair(ctx, newUser, session.ID, "Use the inherited enterprise default", nil, "", "")
	if err != nil {
		t.Fatal("new User could not submit the first inherited message", err)
	}
	if firstReply.State != "queued" || firstReply.ResponseSnapshot == nil || len(firstReply.ResponseSnapshot.Stages) != 1 {
		t.Fatal("first inherited message was not queued with an execution snapshot")
	}
	firstStage := firstReply.ResponseSnapshot.Stages[0]
	if firstStage.RuntimeEngine != domain.RuntimeCodex || firstStage.ProviderModel.ID != modelID || firstStage.ProviderModel.Compatibility != "unverified" {
		t.Fatal("first message changed or ignored the inherited unverified pair")
	}
	var runCount int64
	if err := db.Table("runs").Count(&runCount).Error; err != nil || runCount != 0 {
		t.Fatal("saving created a Run", runCount, err)
	}
	if _, err := repository.SetPlatformExecutionDefault(ctx, userID, domain.RuntimeCodex, modelID, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("ordinary User changed enterprise settings", err)
	}
	workflow, err := repository.CreateWorkflow(ctx, userID, domain.WorkflowInput{Name: "Use enterprise default", Goal: "Check execution selection"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	run, err := repository.CreateRun(ctx, userID, workflow.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal("inherited unverified pair could not start a Run", err)
	}
	var frozen runRecord
	if err := db.Where("id=?", run.ID).Take(&frozen).Error; err != nil {
		t.Fatal(err)
	}
	var snapshot domain.ExecutionSnapshot
	if err := json.Unmarshal(frozen.WorkflowSnapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	stages, err := snapshot.OrderedStages()
	if err != nil || len(stages) == 0 || stages[0].RuntimeEngine != domain.RuntimeCodex || stages[0].ProviderModel.ID != modelID {
		t.Fatal("execution ignored inherited selection", err)
	}
	// Preserve historical evidence until the next save, then clear it without a Run lookup.
	exec("UPDATE platform_execution_defaults SET validation_run_id=? WHERE singleton", run.ID)
	historical, err := repository.GetPlatformExecutionDefault(ctx)
	if err != nil || historical.ValidationRunID != run.ID {
		t.Fatal("historical reference was lost", err)
	}
	updated, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, 1)
	if err != nil || updated.Version != 2 || updated.ValidationRunID != "" {
		t.Fatal("resave still required or retained validation proof", err)
	}
	var stored platformExecutionDefaultRecord
	if err := db.Where("singleton").Take(&stored).Error; err != nil || stored.ValidationRunID != nil {
		t.Fatal("missing proof was not persisted as NULL", err)
	}
	if _, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale update error = %v", err)
	}
}
