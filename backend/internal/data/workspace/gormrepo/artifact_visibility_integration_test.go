package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestArtifactQueriesHideIntermediateWorkspaceFiles(t *testing.T) {
	db := conversationTestDatabase(t)
	ctx := context.Background()
	repository := New(db, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	ownerID, sessionID := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", ownerID, ownerID, ownerID, ownerID+"@example.test", ownerID)
	exec("INSERT INTO sessions(id,owner_user_id) VALUES(?,?)", sessionID, ownerID)
	var messageID int64
	if err := db.Raw("INSERT INTO session_messages(session_id,role,state,content) VALUES(?,'assistant','completed','Created `/workspace/output/report.pdf`.') RETURNING id", sessionID).Scan(&messageID).Error; err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 64)
	finalSessionArtifact, intermediateSessionArtifact := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO session_artifacts(id,owner_user_id,session_id,message_id,name,path,object_key,size_bytes,sha256,expires_at) VALUES(?,?,?,?,?,?,?,?,?,now() + interval '1 day')", finalSessionArtifact, ownerID, sessionID, messageID, "report.pdf", "output/report.pdf", "artifacts/report", 12, sha)
	exec("INSERT INTO session_artifacts(id,owner_user_id,session_id,message_id,name,path,object_key,size_bytes,sha256,expires_at) VALUES(?,?,?,?,?,?,?,?,?,now() + interval '1 day')", intermediateSessionArtifact, ownerID, sessionID, messageID, "package.json", "node_modules/example/package.json", "artifacts/package", 12, sha)

	messages, err := repository.ListMessages(ctx, ownerID, sessionID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || len(messages[0].Artifacts) != 1 || messages[0].Artifacts[0].ID != finalSessionArtifact {
		t.Fatalf("Session Artifacts = %#v", messages)
	}
	if _, err = repository.GetSessionArtifact(ctx, ownerID, sessionID, intermediateSessionArtifact); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("intermediate Session Artifact remains downloadable: %v", err)
	}

	workflowID, runID := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','Create report',?)", workflowID, ownerID, "owners/"+ownerID+"/workflows/"+workflowID)
	exec("INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot,final_result) VALUES(?,?,1,?,?,'Workflow','manual','succeeded','{}'::jsonb,?::jsonb)", runID, runID, ownerID, workflowID, `{"text":"Created /workspace/output/report.pdf."}`)
	finalRunArtifact, intermediateRunArtifact := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO artifacts(id,owner_user_id,workflow_id,run_id,kind,name,path,object_key,size_bytes,sha256) VALUES(?,?,?,?,'file','report.pdf','output/report.pdf','artifacts/report',12,?)", finalRunArtifact, ownerID, workflowID, runID, sha)
	exec("INSERT INTO artifacts(id,owner_user_id,workflow_id,run_id,kind,name,path,object_key,size_bytes,sha256) VALUES(?,?,?,?,'file','create_report.js','create_report.js','artifacts/script',12,?)", intermediateRunArtifact, ownerID, workflowID, runID, sha)

	artifacts, err := repository.ListArtifacts(ctx, ownerID, workflowID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].ID != finalRunArtifact {
		t.Fatalf("Workflow Artifacts = %#v", artifacts)
	}
	if _, err = repository.GetArtifact(ctx, ownerID, workflowID, intermediateRunArtifact); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("intermediate Workflow Artifact remains downloadable: %v", err)
	}
}
