package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func TestPlatformExecutionDefaultRequiresSuccessfulMatchingRunAndPropagates(t *testing.T) {
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
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext,verification_status) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','ciphertext','verified')`, connectionID, administrator)
	compatibility, _ := json.Marshal([]domain.RuntimeModelCompatibility{{RuntimeEngine: domain.RuntimeCodex, Status: "unverified"}})
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name,compatibility) VALUES(?,?,'model-1','Model',?::jsonb)`, modelID, connectionID, string(compatibility))
	defaults, _ := json.Marshal(map[string]string{"codex": modelID})
	exec(`INSERT INTO personal_settings(user_id,default_runtime_engine,runtime_model_defaults,execution_inherited) VALUES(?,'codex',?::jsonb,false)`, administrator, string(defaults))
	exec(`INSERT INTO personal_settings(user_id) VALUES(?)`, userID)
	workflow, err := repository.CreateWorkflow(ctx, administrator, domain.WorkflowInput{Name: "Validate default", Goal: "Return a short health check"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	run, err := repository.CreateRun(ctx, administrator, workflow.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE runs SET state='succeeded',started_at=now(),ended_at=now() WHERE id=?`, run.ID)

	created, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.ValidationRunID != run.ID {
		t.Fatalf("created default = %#v", created)
	}
	var promoted struct {
		Compatibility []byte `gorm:"column:compatibility"`
	}
	if err := db.Table("provider_models").Select("compatibility").Where("id = ?", modelID).Scan(&promoted).Error; err != nil {
		t.Fatal(err)
	}
	var promotedCompatibility []domain.RuntimeModelCompatibility
	if err := json.Unmarshal(promoted.Compatibility, &promotedCompatibility); err != nil || len(promotedCompatibility) != 1 || promotedCompatibility[0].Status != "verified" {
		t.Fatalf("promoted compatibility = %#v, %v", promotedCompatibility, err)
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
	if _, err := repository.SetPlatformExecutionDefault(ctx, administrator, domain.RuntimeCodex, modelID, run.ID, 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale update error = %v", err)
	}
}
