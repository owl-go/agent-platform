package gormrepo

import (
	"context"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"github.com/google/uuid"
)

func TestChannelTypingJobRechecksExecutionAndCurrentAuthorization(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "typing-question", "alice", "question")
	f.admit(t)
	var run runRecord
	if err := f.db.Where("message_channel_id=?", f.channel.ID).Take(&run).Error; err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: run.ID, OwnerID: f.owner, WorkflowID: f.workflow}
	assertState := func(want bool, running bool) {
		t.Helper()
		current, err := f.repo.GetChannelTypingJob(ctx, job)
		if err != nil || (current != nil) != want {
			t.Fatalf("typing eligibility: present=%v error=%v", current != nil, err)
		}
		if current != nil && (current.Running != running || current.Message.SenderID != "alice" || current.Stored.Channel.ID != f.channel.ID || len(current.ReplyCiphertext) == 0 || current.Message.Reply != nil) {
			t.Fatal("typing job lost protected identity or state")
		}
	}
	update := func(model any, where string, id string, field string, value any) {
		t.Helper()
		if err := f.db.Model(model).Where(where, id).Update(field, value).Error; err != nil {
			t.Fatal(err)
		}
	}
	assertState(false, false) // queued is not typing
	update(&runRecord{}, "id=?", run.ID, "state", "running")
	assertState(true, true)
	wrong := job
	wrong.OwnerID = uuid.NewString()
	if current, err := f.repo.GetChannelTypingJob(ctx, wrong); err != nil || current != nil {
		t.Fatal("owner scope bypassed")
	}
	wrong = job
	wrong.WorkflowID = uuid.NewString()
	if current, err := f.repo.GetChannelTypingJob(ctx, wrong); err != nil || current != nil {
		t.Fatal("workflow scope bypassed")
	}
	update(&runRecord{}, "id=?", run.ID, "state", "waiting_for_user")
	assertState(true, false)
	update(&runRecord{}, "id=?", run.ID, "state", "running")
	update(&channelRecord{}, "id=?", f.channel.ID, "enabled", false)
	assertState(false, false)
	update(&channelRecord{}, "id=?", f.channel.ID, "enabled", true)
	var channel channelRecord
	if err := f.db.Where("id=?", f.channel.ID).Take(&channel).Error; err != nil {
		t.Fatal(err)
	}
	update(&channelRecord{}, "id=?", f.channel.ID, "config_version", channel.ConfigVersion+1)
	assertState(false, false)
	update(&channelRecord{}, "id=?", f.channel.ID, "config_version", channel.ConfigVersion)
	update(&channelRecord{}, "id=?", f.channel.ID, "generation", channel.Generation+1)
	assertState(false, false)
	update(&channelRecord{}, "id=?", f.channel.ID, "generation", channel.Generation)
	now := time.Now()
	update(&channelRecord{}, "id=?", f.channel.ID, "deleted_at", now)
	assertState(false, false)
	update(&channelRecord{}, "id=?", f.channel.ID, "deleted_at", nil)
	if err := f.db.Exec("UPDATE users SET disabled_at=? WHERE id=?", now, f.owner).Error; err != nil {
		t.Fatal(err)
	}
	assertState(false, false)
	if err := f.db.Exec("UPDATE users SET disabled_at=NULL WHERE id=?", f.owner).Error; err != nil {
		t.Fatal(err)
	}
	update(&workflowRecord{}, "id=?", f.workflow, "deleted_at", now)
	assertState(false, false)
	update(&workflowRecord{}, "id=?", f.workflow, "deleted_at", nil)
	update(&runRecord{}, "id=?", run.ID, "cancel_requested_at", now)
	assertState(false, false)
	update(&runRecord{}, "id=?", run.ID, "cancel_requested_at", nil)
	update(&runRecord{}, "id=?", run.ID, "state", "succeeded")
	assertState(false, false)
}
