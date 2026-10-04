package gormrepo

import (
	"context"
	"encoding/json"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func (r *Repository) GetChannelTypingJob(ctx context.Context, job application.ExecutionJob) (*application.ChannelTypingJob, error) {
	if !messageChannelsEnabled(r.db) || job.Kind != application.JobWorkflow {
		return nil, nil
	}
	var source struct {
		ID, ChannelID, EventID   string
		ConfigVersion            int64
		Message, ReplyCiphertext []byte
		RunState                 string
	}
	err := r.db.WithContext(ctx).Table("message_channel_inbox AS inbox").Select("inbox.*, run.state AS run_state").
		Joins("JOIN runs run ON run.id=inbox.run_id AND run.message_channel_id=inbox.channel_id").
		Joins("JOIN workflow_message_channels channel ON channel.id=inbox.channel_id").
		Joins("JOIN workflows workflow ON workflow.id=channel.workflow_id").
		Joins("JOIN users owner ON owner.id=channel.owner_user_id").
		Where("run.id=? AND run.owner_user_id=? AND run.workflow_id=? AND channel.owner_user_id=? AND channel.workflow_id=?", job.ID, job.OwnerID, job.WorkflowID, job.OwnerID, job.WorkflowID).
		Where("inbox.state='admitted' AND run.state IN ('running','waiting_for_user') AND run.cancel_requested_at IS NULL AND channel.enabled AND channel.deleted_at IS NULL AND workflow.deleted_at IS NULL AND owner.disabled_at IS NULL AND inbox.config_version=channel.config_version AND inbox.generation=channel.generation").Scan(&source).Error
	if err != nil {
		return nil, err
	}
	if source.ID == "" {
		return nil, nil
	}
	stored, err := r.GetMessageChannel(ctx, job.OwnerID, job.WorkflowID, source.ChannelID)
	if err != nil {
		return nil, err
	}
	var message domain.ChannelMessage
	if err = json.Unmarshal(source.Message, &message); err != nil {
		return nil, err
	}
	if !stored.Channel.Enabled || stored.Channel.ConfigVersion != source.ConfigVersion || !stored.Channel.Audience.Allows(message) {
		return nil, nil
	}
	return &application.ChannelTypingJob{Stored: stored, Message: message, ReplyCiphertext: source.ReplyCiphertext, Running: source.RunState == "running"}, nil
}
