package promptoptimizer

import (
	"context"
	"errors"
	"fmt"

	"agent-platform/backend/internal/data/aicreation/openaiimages"
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

func (resolver *DatabaseConnectionResolver) ResolvePromptOptimization(ctx context.Context) (openaiimages.Connection, error) {
	var row struct {
		Endpoint   string `gorm:"column:endpoint"`
		Ciphertext []byte `gorm:"column:api_key_ciphertext"`
	}
	err := resolver.db.WithContext(ctx).Table("prompt_optimization_settings").Select("endpoint, api_key_ciphertext").Where("singleton = ?", true).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || row.Endpoint == "" || len(row.Ciphertext) == 0 {
		return openaiimages.Connection{}, fmt.Errorf("Prompt Optimization connection is unavailable")
	}
	if err != nil {
		return openaiimages.Connection{}, fmt.Errorf("resolve Prompt Optimization connection: %w", err)
	}
	key, err := resolver.box.Decrypt(row.Ciphertext, "prompt-optimization")
	if err != nil {
		return openaiimages.Connection{}, fmt.Errorf("decrypt Prompt Optimization credential: %w", err)
	}
	return openaiimages.Connection{Endpoint: row.Endpoint, APIKey: key}, nil
}
