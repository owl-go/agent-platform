package gormrepo

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestChannelResponseHandlesPersistAndRevisionFence(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "response-question", "alice", "question")
	stored := f.stored(t)
	message := f.message("response-question", "alice", "question")
	current, err := f.repo.GetChannelResponseReceipt(ctx, stored, message)
	if err != nil || current == nil {
		t.Fatal("receipt unavailable", err)
	}
	stale := *current
	state := application.ChannelResponseState{Phase: "ready", MessageID: "card", ReactionID: "reaction", Summary: "已检查配置的公开摘要"}
	if err := f.repo.SaveChannelResponse(ctx, current, state); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.SaveChannelResponse(ctx, &stale, application.ChannelResponseState{Phase: "creating"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale state accepted", err)
	}
	restored, err := f.repo.GetChannelResponseReceipt(ctx, stored, message)
	if err != nil || restored.Response != state || restored.ResponseRevision != 1 {
		t.Fatal("restart lost response", restored, err)
	}
	f.admit(t)
	var run runRecord
	if err := f.db.Where("message_channel_id=?", f.channel.ID).Take(&run).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&run).Update("state", "running").Error; err != nil {
		t.Fatal(err)
	}
	job := application.ExecutionJob{Kind: application.JobWorkflow, ID: run.ID, OwnerID: f.owner, WorkflowID: f.workflow}
	running, err := f.repo.GetChannelTypingJob(ctx, job)
	if err != nil || running == nil || running.Response != state {
		t.Fatal("running response unavailable", err)
	}
	if err := f.db.Model(&run).Update("state", "succeeded").Error; err != nil {
		t.Fatal(err)
	}
	if receipt, err := f.repo.GetChannelResponseReceipt(ctx, stored, message); err != nil || receipt != nil {
		t.Fatal("late reaction allowed after terminal Run", err)
	}
	if err := f.db.Model(&run).Update("state", "running").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&channelRecord{}).Where("id=?", stored.Channel.ID).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if receipt, err := f.repo.GetChannelResponseReceipt(ctx, stored, message); err != nil || receipt != nil {
		t.Fatal("revoked receipt allowed", err)
	}
	if running, err := f.repo.GetChannelTypingJob(ctx, job); err != nil || running != nil {
		t.Fatal("revoked stream allowed", err)
	}
}

func TestFeishuTerminalCardsPreserveCompleteAnswerAndConfirmedRecovery(t *testing.T) {
	f := newChannelFixture(t)
	f.enable(t)
	ctx := context.Background()
	f.receive(t, "long-card-question", "alice", "question")
	var inbox channelInboxRecord
	if err := f.db.Where("event_id=?", "long-card-question").Take(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	var channel channelRecord
	if err := f.db.Where("id=?", f.channel.ID).Take(&channel).Error; err != nil {
		t.Fatal(err)
	}
	channel.Provider = "feishu"
	text := strings.Repeat("完整答案🙂", 1500)
	if err := enqueueChannelDelivery(f.db, channel, inbox, "answer", text); err != nil {
		t.Fatal(err)
	}
	var deliveries []channelDeliveryRecord
	if err := f.db.Where("inbox_id=? AND kind='answer'", inbox.ID).Order("chunk").Find(&deliveries).Error; err != nil {
		t.Fatal(err)
	}
	var reconstructed string
	for _, delivery := range deliveries {
		if len([]rune(delivery.Payload)) > 1800 {
			t.Fatal("oversized card")
		}
		reconstructed += delivery.Payload
	}
	if len(deliveries) != 5 || reconstructed != text {
		t.Fatal("final answer was truncated")
	}
	current, err := f.repo.GetChannelResponseReceipt(ctx, f.stored(t), f.message("long-card-question", "alice", "question"))
	if err != nil || current == nil {
		t.Fatal(err)
	}
	if err := f.repo.SaveChannelResponse(ctx, current, application.ChannelResponseState{Phase: "outcome_unknown", Summary: "公开的执行摘要"}); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&channelDeliveryRecord{}).Where("id=?", deliveries[0].ID).Update("state", "outcome_unknown").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.repo.RetryChannelDelivery(ctx, f.owner, f.workflow, f.channel.ID, deliveries[0].ID, f.channel.Version, false); !errors.Is(err, domain.ErrInvalid) {
		t.Fatal("unknown resend did not require confirmation", err)
	}
	if err := f.repo.RetryChannelDelivery(ctx, f.owner, f.workflow, f.channel.ID, deliveries[0].ID, f.channel.Version, true); err != nil {
		t.Fatal(err)
	}
	current, err = f.repo.GetChannelResponseReceipt(ctx, f.stored(t), f.message("long-card-question", "alice", "question"))
	if err != nil || current.Response.Phase != "received" || current.Response.Summary != "公开的执行摘要" {
		t.Fatal("confirmed recovery did not release creation", err)
	}
}
