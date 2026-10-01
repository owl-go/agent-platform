package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
)

func TestSessionWorkflowConversionIsAtomicAndIdempotent(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, sessionID, connectionID, modelID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", owner, owner, owner, owner+"@example.test", owner)
	exec(`INSERT INTO model_provider_connections(id,credential_owner_user_id,name,provider_type,endpoint,protocols,api_key_ciphertext) VALUES(?,?,'Provider','openai','https://example.test','["openai_responses"]','test')`, connectionID, owner)
	exec(`INSERT INTO model_provider_credential_versions(connection_id,connection_version,api_key_ciphertext) VALUES(?,1,'test')`, connectionID)
	exec(`INSERT INTO provider_models(id,connection_id,model_id,display_name) VALUES(?,?,'model-1','Model')`, modelID, connectionID)
	defaults, _ := json.Marshal(map[string]string{"codex": modelID})
	exec(`INSERT INTO personal_settings(user_id,default_runtime_engine,runtime_model_defaults) VALUES(?,'codex',?::jsonb)`, owner, string(defaults))
	exec(`INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,'Investigate incident')`, sessionID, owner)
	exec(`INSERT INTO session_messages(session_id,role,content,state) VALUES(?,'user','Find the root cause','completed')`, sessionID)
	stage := domain.ExecutionStageSnapshot{Position: 1, RuntimeEngine: domain.RuntimeCodex, ProviderModel: domain.ProviderModelSnapshot{ID: modelID, ModelID: "model-1", Protocols: []string{"openai_responses"}}, Skills: []domain.SkillSnapshot{{ID: uuid.NewString(), Name: "Report", ObjectKey: "skills/report.zip", SHA256: "digest"}}}
	snapshot, _ := json.Marshal(domain.ResponseSnapshot{SchemaVersion: 2, Stages: []domain.ExecutionStageSnapshot{stage}})
	exec(`INSERT INTO session_messages(session_id,role,content,state,response_snapshot) VALUES(?,'assistant','Root cause found','completed',?::jsonb)`, sessionID, string(snapshot))
	var messageID int64
	if err := db.Raw("SELECT max(id) FROM session_messages WHERE session_id = ?", sessionID).Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}

	firstID := uuid.NewString()
	created, err := repository.CreateWorkflowFromSession(ctx, owner, firstID, sessionID, messageID, "Incident workflow", "Find the root cause")
	if err != nil {
		t.Fatal(err)
	}
	if created.Workflow.ID != firstID || created.Run.State != "queued" || created.Run.ExecutionPlan == nil || created.Run.ExecutionPlan.State != "approved" || created.Link.ValidationRunID != created.Run.ID || created.Replayed {
		t.Fatalf("creation = %#v", created)
	}
	var persisted struct {
		Snapshot []byte `gorm:"column:workflow_snapshot"`
	}
	if err := db.Table("runs").Select("workflow_snapshot").Where("id = ?", created.Run.ID).Take(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	var frozen domain.ExecutionSnapshot
	if err := json.Unmarshal(persisted.Snapshot, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen.Stages) != 1 || len(frozen.Stages[0].Skills) != 1 || frozen.Stages[0].Skills[0].Name != "Report" {
		t.Fatalf("converted frozen resources = %#v", frozen.Stages)
	}
	replayed, err := repository.CreateWorkflowFromSession(ctx, owner, uuid.NewString(), sessionID, messageID, "Incident workflow", "Find the root cause")
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.Workflow.ID != firstID || replayed.Run.ID != created.Run.ID {
		t.Fatalf("replay = %#v", replayed)
	}
	connectorSession := uuid.NewString()
	exec(`INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,'Feishu session')`, connectorSession, owner)
	exec(`INSERT INTO session_messages(session_id,role,content,state) VALUES(?,'user','Send a message','completed')`, connectorSession)
	connectorStage := stage
	connectorStage.Skills = nil
	connectorStage.CLIConnectors = []domain.CLIConnectorSnapshot{{
		ID: uuid.NewString(), Name: "飞书", Executable: "lark-cli", AuthenticationDriver: "feishu",
		BundleObjectKey: "cli-connectors/feishu/bundle.tgz", BundleSHA256: "bundle-digest",
		RuntimeDigests: []string{"sha256:runtime"},
	}}
	connectorSnapshot, _ := json.Marshal(domain.ResponseSnapshot{SchemaVersion: 2, Stages: []domain.ExecutionStageSnapshot{connectorStage}})
	exec(`INSERT INTO session_messages(session_id,role,content,state,response_snapshot) VALUES(?,'assistant','Message delivered','completed',?::jsonb)`, connectorSession, string(connectorSnapshot))
	var connectorMessageID int64
	if err := db.Raw("SELECT max(id) FROM session_messages WHERE session_id = ?", connectorSession).Scan(&connectorMessageID).Error; err != nil {
		t.Fatal(err)
	}
	feishuRequest := strings.Repeat("中", 35)
	if _, err := repository.CreateWorkflowFromSession(ctx, owner, uuid.NewString(), connectorSession, connectorMessageID, feishuRequest, feishuRequest); err != nil {
		t.Fatalf("completed response using a CLI Connector could not become a Workflow: %v", err)
	}

	badSession := uuid.NewString()
	exec(`INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,'Bad response')`, badSession, owner)
	exec(`INSERT INTO session_messages(session_id,role,content,state) VALUES(?,'assistant','Failed','failed')`, badSession)
	var badMessage int64
	if err := db.Raw("SELECT max(id) FROM session_messages WHERE session_id = ?", badSession).Scan(&badMessage).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateWorkflowFromSession(ctx, owner, uuid.NewString(), badSession, badMessage, "Bad", "Must not persist"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("failed response error = %v", err)
	}
	var workflowCount int64
	if err := db.Table("workflows").Where("owner_user_id = ?", owner).Count(&workflowCount).Error; err != nil {
		t.Fatal(err)
	}
	if workflowCount != 2 {
		t.Fatalf("workflow count = %d, want only the two valid conversions", workflowCount)
	}
}
