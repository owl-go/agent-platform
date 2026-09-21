package gormrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
	"gorm.io/gorm"
)

type embeddingConfigurationRecord struct {
	Singleton        bool      `gorm:"column:singleton;primaryKey"`
	Endpoint         string    `gorm:"column:endpoint"`
	Model            string    `gorm:"column:model"`
	Dimensions       int       `gorm:"column:dimensions"`
	APIKeyCiphertext []byte    `gorm:"column:api_key_ciphertext"`
	Enabled          bool      `gorm:"column:enabled"`
	Version          int64     `gorm:"column:version"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (embeddingConfigurationRecord) TableName() string { return "ai_embedding_configurations" }

func (r *Repository) GetEmbeddingConfiguration(ctx context.Context) (domain.EmbeddingConfiguration, error) {
	var row embeddingConfigurationRecord
	err := r.db.WithContext(ctx).Where("singleton = ?", true).Take(&row).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return domain.EmbeddingConfiguration{}, err
	}
	if err == gorm.ErrRecordNotFound {
		return domain.EmbeddingConfiguration{}, nil
	}
	return domain.EmbeddingConfiguration{Endpoint: row.Endpoint, Model: row.Model, Dimensions: row.Dimensions, APIKeyConfigured: len(row.APIKeyCiphertext) > 0, Enabled: row.Enabled, Version: row.Version, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) SaveEmbeddingConfiguration(ctx context.Context, configuration domain.EmbeddingConfiguration, apiKey []byte) (domain.EmbeddingConfiguration, error) {
	if r.box == nil {
		return domain.EmbeddingConfiguration{}, fmt.Errorf("embedding encryption is unavailable")
	}
	var current embeddingConfigurationRecord
	err := r.db.WithContext(ctx).Where("singleton = ?", true).Take(&current).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return domain.EmbeddingConfiguration{}, err
	}
	if err == gorm.ErrRecordNotFound {
		current.Singleton = true
		current.Version = 1
	} else {
		if configuration.Version != current.Version {
			return domain.EmbeddingConfiguration{}, domain.ErrVersionConflict
		}
		current.Version++
	}
	if len(apiKey) > 0 {
		ciphertext, encryptErr := r.box.Encrypt(apiKey, "ai-embedding:singleton")
		if encryptErr != nil {
			return domain.EmbeddingConfiguration{}, fmt.Errorf("encrypt embedding API key: %w", encryptErr)
		}
		current.APIKeyCiphertext = ciphertext
	}
	if err == gorm.ErrRecordNotFound && len(apiKey) == 0 {
		return domain.EmbeddingConfiguration{}, fmt.Errorf("%w: embedding API key is required", domain.ErrInvalid)
	}
	current.Endpoint, current.Model, current.Dimensions, current.Enabled, current.UpdatedAt = configuration.Endpoint, configuration.Model, configuration.Dimensions, configuration.Enabled, time.Now().UTC()
	if err := r.db.WithContext(ctx).Save(&current).Error; err != nil {
		return domain.EmbeddingConfiguration{}, err
	}
	return domain.EmbeddingConfiguration{Endpoint: current.Endpoint, Model: current.Model, Dimensions: current.Dimensions, APIKeyConfigured: len(current.APIKeyCiphertext) > 0, Enabled: current.Enabled, Version: current.Version, UpdatedAt: current.UpdatedAt}, nil
}

func (r *Repository) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	configuration, err := r.embeddingConfigurationWithKey(ctx)
	if err != nil {
		return nil, err
	}
	if !configuration.Enabled || len(configuration.apiKey) == 0 {
		return nil, fmt.Errorf("embedding provider is not enabled")
	}
	payload := struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{Model: configuration.Model, Input: texts}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(configuration.Endpoint, "/"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+string(configuration.apiKey))
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding provider returned status %d", response.StatusCode)
	}
	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}
	vectors := make([][]float32, len(result.Data))
	for index, item := range result.Data {
		if len(item.Embedding) != configuration.Dimensions {
			return nil, fmt.Errorf("embedding dimensions mismatch: got %d, want %d", len(item.Embedding), configuration.Dimensions)
		}
		vectors[index] = item.Embedding
	}
	return vectors, nil
}

type embeddingProviderConfiguration struct {
	domain.EmbeddingConfiguration
	apiKey []byte
}

func (r *Repository) embeddingConfigurationWithKey(ctx context.Context) (embeddingProviderConfiguration, error) {
	var row embeddingConfigurationRecord
	if err := r.db.WithContext(ctx).Where("singleton = ?", true).Take(&row).Error; err != nil {
		return embeddingProviderConfiguration{}, err
	}
	apiKey, err := r.box.Decrypt(row.APIKeyCiphertext, "ai-embedding:singleton")
	if err != nil {
		return embeddingProviderConfiguration{}, fmt.Errorf("decrypt embedding API key: %w", err)
	}
	return embeddingProviderConfiguration{EmbeddingConfiguration: domain.EmbeddingConfiguration{Endpoint: row.Endpoint, Model: row.Model, Dimensions: row.Dimensions, Enabled: row.Enabled, Version: row.Version}, apiKey: apiKey}, nil
}
