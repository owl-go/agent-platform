package gormrepo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestHomeOverviewIsOwnerScopedAndMetadataOnly(t *testing.T) {
	db := conversationTestDatabase(t)
	repository := New(db, nil)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	owner, other := uuid.NewString(), uuid.NewString()
	for _, id := range []string{owner, other} {
		exec("INSERT INTO users(id,oidc_subject,username,email,display_name) VALUES(?,?,?,?,?)", id, id, id, id+"@example.test", id)
	}
	ownerSession, otherSession := uuid.NewString(), uuid.NewString()
	exec("INSERT INTO sessions(id,owner_user_id,title) VALUES(?,?,'Quarterly review'),(?,?,'Other private session')", ownerSession, owner, otherSession, other)
	var assistantID int64
	if err := db.Raw(`INSERT INTO session_messages(session_id,role,state,content,error,execution_plan)
		VALUES(?,'assistant','waiting_for_user','private prompt and answer','private failure detail','{}'::jsonb) RETURNING id`, ownerSession).Scan(&assistantID).Error; err != nil {
		t.Fatal(err)
	}
	exec("INSERT INTO session_messages(session_id,role,state,content) VALUES(?,'assistant','failed','other private content')", otherSession)

	ownerWorkflow, otherWorkflow, failedRun := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec("INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Weekly report','private goal',?),(?,?,'Other private workflow','private goal',?)",
		ownerWorkflow, owner, "workspace/"+ownerWorkflow, otherWorkflow, other, "workspace/"+otherWorkflow)
	exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot,terminal_error)
		VALUES(?,?,1,?,?,'Weekly report','manual','failed','{}'::jsonb,'private provider error')`, failedRun, failedRun, owner, ownerWorkflow)
	otherRun := uuid.NewString()
	exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot)
		VALUES(?,?,1,?,?,'Other private workflow','manual','failed','{}'::jsonb)`, otherRun, otherRun, other, otherWorkflow)

	approvalID := uuid.NewString()
	exec(`INSERT INTO cli_command_approvals(id,owner_user_id,execution_kind,execution_id,stage_id,connector_name,operation,target,redacted_arguments,command_digest,nonce_hash,state,expires_at)
		VALUES(?,?,'session',?,?, 'Feishu Tasks','write','private target','private arguments',?,?,'pending',now()+interval '5 minutes')`,
		approvalID, owner, fmt.Sprintf("%d", assistantID), "stage-"+approvalID, strings.Repeat("a", 64), strings.Repeat("b", 64))

	overview, err := repository.GetHomeOverview(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.RecentTasks) != 2 {
		t.Fatalf("recent tasks = %#v, want owner Session and Run", overview.RecentTasks)
	}
	for _, task := range overview.RecentTasks {
		if task.Title == "Other private session" || task.Title == "Other private workflow" {
			t.Fatalf("cross-owner task leaked: %#v", task)
		}
	}
	if len(overview.CommonWorkflows) != 1 || overview.CommonWorkflows[0].ID != ownerWorkflow {
		t.Fatalf("common Workflows = %#v, want only owner Workflow", overview.CommonWorkflows)
	}
	if len(overview.ActionItems) != 2 {
		t.Fatalf("actions = %#v, want approval and failed Run", overview.ActionItems)
	}
	if overview.ActionItems[0].Kind != "approval" || overview.ActionItems[0].ExecutionID != fmt.Sprintf("%d", assistantID) {
		t.Fatalf("approval did not replace duplicate Plan action: %#v", overview.ActionItems)
	}
	for _, action := range overview.ActionItems {
		if strings.Contains(action.Title, "private") {
			t.Fatalf("content or error leaked through Home title: %#v", action)
		}
	}
}
