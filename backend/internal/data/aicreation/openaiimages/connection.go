package openaiimages

import (
	"context"
	"errors"
	"fmt"

	"agent-platform/backend/internal/secretcrypto"

	"gorm.io/gorm"
)

type DatabaseConnectionResolver struct {
	db  *gorm.DB
	box *secretcrypto.Box
}

func NewDatabaseConnectionResolver(db *gorm.DB, box *secretcrypto.Box) (*DatabaseConnectionResolver, error) {
	if db == nil || box == nil {
		return nil, fmt.Errorf("database and encryption box are required")
	}
	return &DatabaseConnectionResolver{db: db, box: box}, nil
}

func (resolver *DatabaseConnectionResolver) Resolve(ctx context.Context, id string, version int64) (Connection, error) {
	var row struct {
		Endpoint          string `gorm:"column:endpoint"`
		CredentialOwnerID string `gorm:"column:credential_owner_user_id"`
		Ciphertext        []byte `gorm:"column:api_key_ciphertext"`
	}
	err := resolver.db.WithContext(ctx).Table("model_provider_connections connection").
		Select("connection.endpoint, connection.credential_owner_user_id, credential.api_key_ciphertext").
		Joins("JOIN model_provider_credential_versions credential ON credential.connection_id = connection.id AND credential.connection_version = ?", version).
		Where("connection.id = ?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Connection{}, fmt.Errorf("Image Model connection is unavailable")
	}
	if err != nil {
		return Connection{}, fmt.Errorf("resolve Image Model connection: %w", err)
	}
	key, err := resolver.box.Decrypt(row.Ciphertext, "model-provider:"+row.CredentialOwnerID)
	if err != nil {
		return Connection{}, fmt.Errorf("decrypt Image Model credential: %w", err)
	}
	return Connection{Endpoint: row.Endpoint, APIKey: key}, nil
}
