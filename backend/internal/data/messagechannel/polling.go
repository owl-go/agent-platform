package messagechannel

import (
	"context"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func pollMessages(ctx context.Context, s application.ChannelStored, cursor application.ChannelReceiveCursor, sink application.ChannelMessageSink, poll func(context.Context, string) ([]domain.ChannelMessage, string, error)) error {
	if cursor == nil {
		return providerError("channel_cursor_unavailable")
	}
	current, err := cursor.Load(ctx, s)
	if err != nil {
		return err
	}
	for ctx.Err() == nil {
		messages, next, err := poll(ctx, current)
		if err != nil {
			return err
		}
		for _, message := range messages {
			commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := sink(commitCtx, message)
			cancel()
			if err != nil {
				return err
			}
		}
		if next != "" && next != current {
			if err := cursor.Save(ctx, s, next); err != nil {
				return err
			}
			current = next
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
func directAudience(a domain.ChannelAudience) error {
	if !a.AllowDirect || len(a.GroupIDs) != 0 {
		return providerError("provider_direct_messages_only")
	}
	return nil
}
func sameIdentity(ctx context.Context, a application.ChannelAccount, s application.ChannelStored, c application.ChannelCredentials) error {
	identity, err := a.Identify(ctx, c, s.Channel.Region)
	if err != nil {
		return err
	}
	if identity.ID != s.Channel.AccountID || identity.TenantID != s.Channel.TenantID || identity.BindingID != s.Channel.BindingID {
		return providerError("provider_identity_changed")
	}
	return nil
}
