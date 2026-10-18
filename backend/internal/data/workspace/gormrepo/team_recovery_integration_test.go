package gormrepo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

func TestWorkerRestartDoesNotReplayCoordinatedSessionOrWorkflow(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	invocation := uuid.NewString()
	stages, _ := json.Marshal([]domain.ExpertStage{
		{InvocationID: uuid.NewString(), Position: 1, State: "succeeded", ModelInvoked: true, FinalText: "Safe completed evidence"},
		{InvocationID: invocation, Position: 2, State: "waiting_for_user", StartedAt: time.Now()},
		{InvocationID: uuid.NewString(), Position: 3, State: "queued"},
	})
	snapshot := []byte(`{"schema_version":3,"coordination":{"lead_member_id":"lead"}}`)
	credit := []byte(`{"total_hundredths":17,"stages":[]}`)
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Updates(map[string]any{
		"response_snapshot": snapshot, "expert_stages": stages, "credit_consumption": credit, "state": "waiting_for_user",
	}).Error; err != nil {
		t.Fatal(err)
	}
	workflowID, runID := uuid.NewString(), uuid.NewString()
	if err := fixture.db.Exec(`INSERT INTO workflows(id,owner_user_id,name,goal,workspace_path) VALUES(?,?,'Workflow','Goal',?)`, workflowID, fixture.ownerID, "workspaces/"+workflowID).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.db.Exec(`INSERT INTO runs(id,conversation_id,turn_number,owner_user_id,workflow_id,workflow_name,trigger,state,workflow_snapshot,expert_stages,credit_consumption) VALUES(?,?,1,?,?,'Workflow','manual','running',?::jsonb,?::jsonb,?::jsonb)`, runID, runID, fixture.ownerID, workflowID, string(snapshot), string(stages), string(credit)).Error; err != nil {
		t.Fatal(err)
	}
	restarted := restartWorkerRepository(t, fixture.db, fixture.repository)
	if claimed, err := restarted.ClaimNext(context.Background()); err != nil || claimed != nil {
		t.Fatalf("interrupted coordination replayed: %#v %v", claimed, err)
	}
	var message messageRecord
	var run runRecord
	fixture.db.Where("id = ?", fixture.assistant.AssistantMessageID).Take(&message)
	fixture.db.Where("id = ?", runID).Take(&run)
	if message.State != "failed" || message.Error == nil || !strings.Contains(*message.Error, "not retried automatically") || run.State != "failed" || run.TerminalError == nil {
		t.Fatalf("missing interrupted terminal state: message=%s run=%s", message.State, run.State)
	}
	for _, encoded := range [][]byte{message.ExpertStages, run.ExpertStages} {
		var actual []domain.ExpertStage
		if err := json.Unmarshal(encoded, &actual); err != nil {
			t.Fatal(err)
		}
		if len(actual) != 3 || actual[0].State != "succeeded" || actual[0].FinalText != "Safe completed evidence" || actual[1].State != "failed" || actual[2].State != "failed" || actual[2].ElapsedMS != 0 {
			t.Fatalf("lost terminal evidence or left queued work: %+v", actual)
		}
	}
	for _, encoded := range [][]byte{message.CreditConsumption, run.CreditConsumption} {
		if !strings.Contains(string(encoded), "17") {
			t.Fatal("recovery discarded consumed Credits")
		}
	}
	if claimed, err := restarted.ClaimNext(context.Background()); err != nil || claimed != nil {
		t.Fatalf("terminal attempt requeued: %#v %v", claimed, err)
	}
}
