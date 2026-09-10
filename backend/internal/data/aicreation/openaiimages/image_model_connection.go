package openaiimages

import (
	"context"
	"errors"
	"fmt"

	"agent-platform/backend/internal/secretcrypto"

	"gorm.io/gorm"
)

type ImageModelDatabaseConnectionResolver struct {
	db  *gorm.DB
	box *secretcrypto.Box
}

func NewImageModelDatabaseConnectionResolver(db *gorm.DB, box *secretcrypto.Box) (*ImageModelDatabaseConnectionResolver, error) {
	if db == nil || box == nil {
		return nil, fmt.Errorf("database and encryption box are required")
	}
	return &ImageModelDatabaseConnectionResolver{db: db, box: box}, nil
}

func (resolver *ImageModelDatabaseConnectionResolver) ResolveImageModel(ctx context.Context, revisionID string) (Connection, error) {
	var row struct {
		ModelID    string `gorm:"column:image_model_id"`
		Endpoint   string `gorm:"column:endpoint"`
		Ciphertext []byte `gorm:"column:api_key_ciphertext"`
	}
	err := resolver.db.WithContext(ctx).Table("image_model_revisions").
		Select("image_model_id, endpoint, api_key_ciphertext").Where("id = ?", revisionID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || row.Endpoint == "" || len(row.Ciphertext) == 0 {
		return Connection{}, fmt.Errorf("Image Model connection is unavailable")
	}
	if err != nil {
		return Connection{}, fmt.Errorf("resolve Image Model connection: %w", err)
	}
	key, err := resolver.box.Decrypt(row.Ciphertext, "image-model:"+row.ModelID)
	if err != nil {
		return Connection{}, fmt.Errorf("decrypt Image Model credential: %w", err)
	}
	return Connection{Endpoint: row.Endpoint, APIKey: key}, nil
}
