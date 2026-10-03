package application

import (
	"context"
	"fmt"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// ChannelReceiveCursor advances only after every message in a batch crosses the sink.
// The cursor is account/config-scoped and never enters a Runtime Request.
type ChannelReceiveCursor interface {
	Load(context.Context, ChannelStored) (string, error)
	Save(context.Context, ChannelStored, string) error
}
type ChannelReceiveCursorRepository interface {
	LoadChannelReceiveCursor(context.Context, ChannelStored) ([]byte, error)
	StoreChannelReceiveCursor(context.Context, ChannelStored, []byte) error
}
type channelReceiveCursor struct {
	repository ChannelReceiveCursorRepository
	cipher     ChannelCipher
}

func NewChannelReceiveCursor(repository ChannelReceiveCursorRepository, cipher ChannelCipher) ChannelReceiveCursor {
	return &channelReceiveCursor{repository: repository, cipher: cipher}
}
func channelCursorAAD(s ChannelStored) string {
	return ChannelCredentialAAD(s.Channel) + ":receive-cursor"
}
func (c *channelReceiveCursor) Load(ctx context.Context, s ChannelStored) (string, error) {
	data, err := c.repository.LoadChannelReceiveCursor(ctx, s)
	if err != nil || len(data) == 0 {
		return "", err
	}
	plain, err := c.cipher.Decrypt(data, channelCursorAAD(s))
	if err != nil {
		return "", fmt.Errorf("channel_cursor_unavailable")
	}
	return string(plain), nil
}
func (c *channelReceiveCursor) Save(ctx context.Context, s ChannelStored, value string) error {
	if len(value) > 16384 {
		return domain.ErrInvalid
	}
	data, err := c.cipher.Encrypt([]byte(value), channelCursorAAD(s))
	if err != nil {
		return fmt.Errorf("channel_cursor_unavailable")
	}
	return c.repository.StoreChannelReceiveCursor(ctx, s, data)
}

// Some transports only support direct messages or require an explicit room scope.
type ChannelAudienceValidator interface {
	ValidateAudience(domain.ChannelAudience) error
}
