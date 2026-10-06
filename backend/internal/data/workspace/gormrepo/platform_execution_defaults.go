package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"gorm.io/gorm"
)

func (repository *Repository) GetPlatformExecutionDefault(ctx context.Context) (domain.PlatformExecutionDefault, error) {
	var row platformExecutionDefaultRecord
	if err := repository.db.WithContext(ctx).Where("singleton", true).Take(&row).Error; err != nil {
		return domain.PlatformExecutionDefault{}, mapNotFound(err)
	}
	return platformExecutionDefaultDomain(row)
}

func (repository *Repository) SetPlatformExecutionDefault(ctx context.Context, administratorID string, runtime domain.RuntimeEngine, modelID string, expectedVersion int64) (domain.PlatformExecutionDefault, error) {
	if administratorID == "" || modelID == "" || expectedVersion < 0 {
		return domain.PlatformExecutionDefault{}, fmt.Errorf("%w: platform execution default fields are required", domain.ErrInvalid)
	}
	if _, err := domain.ParseRuntime(string(runtime)); err != nil {
		return domain.PlatformExecutionDefault{}, err
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var administrator bool
		if err := tx.Table("users").Select("administrator").Where("id = ? AND disabled_at IS NULL", administratorID).Scan(&administrator).Error; err != nil {
			return err
		}
		if !administrator {
			return domain.ErrNotFound
		}
		var model providerModelRecord
		if err := tx.Where("id = ? AND available", modelID).Take(&model).Error; err != nil {
			return fmt.Errorf("%w: Provider Model is unavailable", domain.ErrInvalid)
		}
		var connection modelProviderConnectionRecord
		if err := tx.Where("id = ?", model.ConnectionID).Take(&connection).Error; err != nil {
			return err
		}
		if len(connection.APIKeyCiphertext) == 0 {
			return fmt.Errorf("%w: Model Provider Connection has no API Key", domain.ErrInvalid)
		}
		var compatibility []domain.RuntimeModelCompatibility
		if err := json.Unmarshal(model.Compatibility, &compatibility); err != nil {
			return fmt.Errorf("decode Provider Model compatibility: %w", err)
		}
		compatibilityIndex := -1
		for index, item := range compatibility {
			if item.RuntimeEngine == runtime {
				compatibilityIndex = index
				if item.Status == "incompatible" {
					return fmt.Errorf("%w: Runtime and Provider Model are incompatible", domain.ErrInvalid)
				}
				break
			}
		}
		if compatibilityIndex < 0 {
			return fmt.Errorf("%w: Runtime and Provider Model compatibility is unavailable", domain.ErrInvalid)
		}

		var current platformExecutionDefaultRecord
		lookupErr := tx.Where("singleton", true).Take(&current).Error
		now := time.Now().UTC()
		switch {
		case errors.Is(lookupErr, gorm.ErrRecordNotFound):
			if expectedVersion != 0 {
				return domain.ErrConflict
			}
			if err := tx.Create(&platformExecutionDefaultRecord{Singleton: true, RuntimeEngine: string(runtime), ProviderModelID: modelID, UpdatedByUserID: administratorID, CreatedAt: now, UpdatedAt: now, Version: 1}).Error; err != nil {
				return err
			}
		case lookupErr != nil:
			return lookupErr
		default:
			if current.Version != expectedVersion {
				return domain.ErrConflict
			}
			result := tx.Model(&platformExecutionDefaultRecord{}).Where("singleton AND version = ?", expectedVersion).Updates(map[string]any{"runtime_engine": string(runtime), "provider_model_id": modelID, "validation_run_id": nil, "updated_by_user_id": administratorID, "updated_at": now, "version": gorm.Expr("version + 1")})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrConflict
			}
		}
		defaults, err := marshal(map[string]string{string(runtime): modelID})
		if err != nil {
			return err
		}
		return tx.Model(&settingsRecord{}).Where("execution_inherited").Updates(map[string]any{"default_runtime_engine": string(runtime), "runtime_model_defaults": defaults, "updated_at": now, "version": gorm.Expr("version + 1")}).Error
	})
	if err != nil {
		return domain.PlatformExecutionDefault{}, fmt.Errorf("set platform execution default: %w", err)
	}
	return repository.GetPlatformExecutionDefault(ctx)
}

func platformExecutionDefaultDomain(row platformExecutionDefaultRecord) (domain.PlatformExecutionDefault, error) {
	runtime, err := domain.ParseRuntime(row.RuntimeEngine)
	if err != nil {
		return domain.PlatformExecutionDefault{}, err
	}
	validationRunID := ""
	if row.ValidationRunID != nil {
		validationRunID = *row.ValidationRunID
	}
	return domain.PlatformExecutionDefault{RuntimeEngine: runtime, ProviderModelID: row.ProviderModelID, ValidationRunID: validationRunID, UpdatedBy: row.UpdatedByUserID, Version: row.Version, UpdatedAt: row.UpdatedAt}, nil
}
