package gormrepo

import (
	"context"
	"errors"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type channelReceiveCursorRecord struct {
	ChannelID     string `gorm:"primaryKey"`
	ConfigVersion int64
	Ciphertext    []byte
	UpdatedAt     time.Time
}

func (channelReceiveCursorRecord) TableName() string { return "message_channel_receive_cursors" }
func (r *Repository) LoadChannelReceiveCursor(ctx context.Context, s application.ChannelStored) ([]byte, error) {
	var row channelReceiveCursorRecord
	err := r.db.WithContext(ctx).Where("channel_id=? AND config_version=?", s.Channel.ID, s.Channel.ConfigVersion).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return row.Ciphertext, err
}
func (r *Repository) StoreChannelReceiveCursor(ctx context.Context, s application.ChannelStored, data []byte) error {
	if len(data) > 32768 {
		return domain.ErrInvalid
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var channel channelRecord
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND owner_user_id=? AND workflow_id=? AND deleted_at IS NULL", s.Channel.ID, s.Channel.OwnerID, s.Channel.WorkflowID).Take(&channel).Error; err != nil {
			return mapNotFound(err)
		}
		if channel.ConfigVersion != s.Channel.ConfigVersion || channel.Version != s.Channel.Version || (!channel.Enabled && !(channel.ValidationState == "testing" && channel.ValidationUntil != nil && time.Now().Before(*channel.ValidationUntil))) {
			return domain.ErrConflict
		}
		row := channelReceiveCursorRecord{ChannelID: s.Channel.ID, ConfigVersion: s.Channel.ConfigVersion, Ciphertext: data, UpdatedAt: time.Now().UTC()}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "channel_id"}}, DoUpdates: clause.AssignmentColumns([]string{"config_version", "ciphertext", "updated_at"})}).Create(&row).Error
	})
}
