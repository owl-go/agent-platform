package gormrepo

import (
	"context"
	"encoding/json"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func (r *Repository) GetChannelResponseReceipt(ctx context.Context, stored application.ChannelStored, message domain.ChannelMessage) (*application.ChannelTypingJob, error) {
	if !messageChannelsEnabled(r.db) {
		return nil, nil
	}
	var source channelInboxRecord
	err := r.db.WithContext(ctx).Table("message_channel_inbox AS inbox").Select("inbox.*").
		Joins("LEFT JOIN runs run ON run.id=inbox.run_id").
		Joins("JOIN workflow_message_channels channel ON channel.id=inbox.channel_id").
		Joins("JOIN workflows workflow ON workflow.id=channel.workflow_id").
		Joins("JOIN users owner ON owner.id=channel.owner_user_id").
		Where("inbox.channel_id=? AND inbox.event_id=? AND channel.owner_user_id=? AND channel.workflow_id=?", stored.Channel.ID, message.EventID, stored.Channel.OwnerID, stored.Channel.WorkflowID).
		Where("(inbox.state='received' OR (inbox.state='admitted' AND run.state IN ('queued','running','waiting_for_user') AND run.cancel_requested_at IS NULL)) AND channel.enabled AND channel.deleted_at IS NULL AND workflow.deleted_at IS NULL AND owner.disabled_at IS NULL AND inbox.config_version=channel.config_version AND inbox.generation=channel.generation").Scan(&source).Error
	if err != nil || source.ID == "" {
		return nil, err
	}
	current, err := r.GetMessageChannel(ctx, stored.Channel.OwnerID, stored.Channel.WorkflowID, stored.Channel.ID)
	if err != nil {
		return nil, err
	}
	if !current.Channel.Enabled || current.Channel.ConfigVersion != source.ConfigVersion || !current.Channel.Audience.Allows(message) {
		return nil, nil
	}
	var state application.ChannelResponseState
	if err := json.Unmarshal(source.Response, &state); err != nil {
		return nil, err
	}
	return &application.ChannelTypingJob{InboxID: source.ID, Stored: current, Message: message, ReplyCiphertext: source.ReplyCiphertext, Response: state, ResponseRevision: source.ResponseRevision}, nil
}

// Revision fencing prevents receipt feedback, Worker activity and terminal
// delivery from overwriting another operation's handles. Cleanup may persist
// after revocation, but no new network answer is authorized by this write.
func (r *Repository) SaveChannelResponse(ctx context.Context, current *application.ChannelTypingJob, state application.ChannelResponseState) error {
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Exec("UPDATE message_channel_inbox SET response=?::jsonb,response_revision=response_revision+1 WHERE id=? AND channel_id=? AND config_version=? AND response_revision=?", string(encoded), current.InboxID, current.Stored.Channel.ID, current.Stored.Channel.ConfigVersion, current.ResponseRevision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	current.Response, current.ResponseRevision = state, current.ResponseRevision+1
	return nil
}
