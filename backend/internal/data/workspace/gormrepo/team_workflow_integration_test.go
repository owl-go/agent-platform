package gormrepo

import (
	"context"
	"encoding/json"
	"testing"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSessionWorkflowConversionRetainsFrozenTeamCoordinationAndMemberIdentity(t *testing.T) {
	fixture := newWorkerRecoveryFixture(t)
	repo, ctx := fixture.repository, context.Background()
	base, err := fixture.assistant.Snapshot.OrderedStages()
	if err != nil {
		t.Fatal(err)
	}
	lead, member := base[0], base[0]
	lead.TeamMemberID, lead.TeamMemberName = "lead", "Lead"
	lead.Expert = &domain.ExpertSnapshot{ID: uuid.NewString(), Name: "Lead", Guidance: "# Frozen lead"}
	member.Position, member.TeamMemberID, member.TeamMemberName = 2, "reviewer", "Reviewer"
	member.Expert = &domain.ExpertSnapshot{ID: uuid.NewString(), Name: "Reviewer", Guidance: "# Frozen reviewer"}
	snapshot := domain.ResponseSnapshot{SchemaVersion: 3, Stages: []domain.ExecutionStageSnapshot{lead, member}, TeamProfile: &domain.ExpertTeamProfileSnapshot{ID: uuid.NewString(), Name: "Frozen Team", LeadMemberID: "lead", Version: 7}, Coordination: &domain.TeamCoordinationSnapshot{LeadMemberID: "lead", MaxParallel: 2}}
	metadata, _ := json.Marshal(snapshot)
	if err := fixture.db.Model(&messageRecord{}).Where("id = ?", fixture.assistant.AssistantMessageID).Updates(map[string]any{"state": "completed", "content": "Official reviewed answer", "response_snapshot": metadata}).Error; err != nil {
		t.Fatal(err)
	}
	creation, err := repo.CreateWorkflowFromSession(ctx, fixture.ownerID, uuid.NewString(), fixture.sessionID, fixture.assistant.AssistantMessageID, "Frozen workflow", "Review again")
	if err != nil {
		t.Fatal(err)
	}
	readSnapshot := func(id string) []byte {
		t.Helper()
		var row runRecord
		if err := fixture.db.Where("id = ?", id).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		return row.WorkflowSnapshot
	}
	var initial domain.ExecutionSnapshot
	if err := json.Unmarshal(readSnapshot(creation.Run.ID), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.SchemaVersion != 3 || initial.Coordination == nil || initial.Coordination.LeadMemberID != "lead" || initial.Coordination.MaxParallel != 2 || initial.TeamProfile == nil || initial.TeamProfile.Version != 7 || initial.Stages[1].Expert.Guidance != "# Frozen reviewer" {
		t.Fatalf("conversion reverted the frozen Team: %+v", initial)
	}
	if err := fixture.db.Model(&runRecord{}).Where("id = ?", creation.Run.ID).Update("state", "succeeded").Error; err != nil {
		t.Fatal(err)
	}
	run, err := repo.CreateRun(ctx, fixture.ownerID, creation.Workflow.ID, "manual", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := structpb.NewStruct(run.WorkflowSnapshot); err != nil {
		t.Fatalf("public Workflow snapshot is not serializable: %v", err)
	}
	team, ok := run.WorkflowSnapshot["team"].(map[string]any)
	if !ok || team["lead_member_id"] != "lead" {
		t.Fatalf("public Team context lost: %+v", run.WorkflowSnapshot)
	}
	var repeat domain.ExecutionSnapshot
	if err := json.Unmarshal(readSnapshot(run.ID), &repeat); err != nil {
		t.Fatal(err)
	}
	if repeat.SchemaVersion != 3 || repeat.Coordination == nil || repeat.TeamProfile == nil || repeat.TeamProfile.Version != 7 || repeat.Stages[0].TeamMemberID != "lead" || repeat.Stages[1].Expert.Guidance != "# Frozen reviewer" {
		t.Fatal("later Workflow Run lost retained coordination")
	}
}
