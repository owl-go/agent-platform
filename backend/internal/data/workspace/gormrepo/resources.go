package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) ListExperts(ctx context.Context, ownerID string) ([]domain.Expert, error) {
	var rows []expertRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Experts: %w", err)
	}
	items := make([]domain.Expert, 0, len(rows))
	for _, row := range rows {
		item, err := expertDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateExpert(ctx context.Context, ownerID string, input domain.ExpertInput) (domain.Expert, error) {
	if err := input.Validate(); err != nil {
		return domain.Expert{}, err
	}
	mcp, _ := marshal(input.MCPServerIDs)
	skills, _ := marshal(input.SkillIDs)
	row := expertRecord{ID: uuid.NewString(), OwnerID: ownerID, Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), MCPServerIDs: mcp, SkillIDs: skills, Version: 1}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateExpertReferences(tx, ownerID, input); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		return domain.Expert{}, fmt.Errorf("create Expert: %w", err)
	}
	return expertDomain(row)
}

func (repository *Repository) UpdateExpert(ctx context.Context, ownerID, expertID string, input domain.ExpertInput, expectedVersion int64) (domain.Expert, error) {
	if err := input.Validate(); err != nil {
		return domain.Expert{}, err
	}
	mcp, _ := marshal(input.MCPServerIDs)
	skills, _ := marshal(input.SkillIDs)
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateExpertReferences(tx, ownerID, input); err != nil {
			return err
		}
		result := tx.Model(&expertRecord{}).
			Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, expertID, expectedVersion).
			Updates(map[string]any{"name": strings.TrimSpace(input.Name), "description": strings.TrimSpace(input.Description), "mcp_server_ids": mcp, "skill_ids": skills, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.Expert{}, fmt.Errorf("update Expert: %w", err)
	}
	return repository.getExpert(ctx, ownerID, expertID)
}

func validateExpertReferences(tx *gorm.DB, ownerID string, input domain.ExpertInput) error {
	if err := validateUniqueUUIDs(input.MCPServerIDs); err != nil {
		return err
	}
	if err := validateUniqueUUIDs(input.SkillIDs); err != nil {
		return err
	}
	if len(input.MCPServerIDs) > 0 {
		var count int64
		if err := tx.Model(&mcpRecord{}).
			Where("owner_user_id = ? AND id IN ? AND test_requested_at IS NULL AND tested_at IS NOT NULL AND test_error IS NULL", ownerID, input.MCPServerIDs).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(input.MCPServerIDs)) {
			return fmt.Errorf("%w: every MCP Server must belong to the User and pass its isolated test", domain.ErrInvalid)
		}
	}
	if len(input.SkillIDs) > 0 {
		var count int64
		if err := tx.Model(&skillRecord{}).Where("owner_user_id = ? AND id IN ?", ownerID, input.SkillIDs).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(input.SkillIDs)) {
			return fmt.Errorf("%w: every Skill must belong to the User", domain.ErrInvalid)
		}
	}
	return nil
}

func validateUniqueUUIDs(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: invalid Extension identifier", domain.ErrInvalid)
		}
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("%w: duplicate Extension identifier", domain.ErrInvalid)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func (repository *Repository) DeleteExpert(ctx context.Context, ownerID, expertID string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sessionIDs []string
		if err := tx.Model(&sessionRecord{}).Where("owner_user_id = ? AND expert_id = ?", ownerID, expertID).Pluck("id", &sessionIDs).Error; err != nil {
			return err
		}
		if len(sessionIDs) > 0 {
			now := time.Now().UTC()
			if err := tx.Model(&sessionRecord{}).Where("id IN ?", sessionIDs).Updates(map[string]any{"archived_at": now, "updated_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
			if err := tx.Model(&messageRecord{}).Where("session_id IN ? AND role = 'assistant' AND state IN ?", sessionIDs, []string{"queued", "generating"}).Updates(map[string]any{"state": "cancelled", "progress_stage": "", "completed_at": now}).Error; err != nil {
				return err
			}
		}
		result := tx.Where("owner_user_id = ? AND id = ?", ownerID, expertID).Delete(&expertRecord{})
		if result.Error != nil {
			return fmt.Errorf("delete Expert: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		// Existing Session snapshots remain frozen and archived; future Workflow runs fall back to personal defaults.
		return tx.Model(&workflowRecord{}).Where("owner_user_id = ? AND expert_id = ?", ownerID, expertID).Update("expert_id", nil).Error
	})
}

func (repository *Repository) getExpert(ctx context.Context, ownerID, expertID string) (domain.Expert, error) {
	var row expertRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, expertID).Take(&row).Error; err != nil {
		return domain.Expert{}, mapNotFound(err)
	}
	return expertDomain(row)
}

func expertDomain(row expertRecord) (domain.Expert, error) {
	item := domain.Expert{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Description: row.Description, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
	if err := json.Unmarshal(row.MCPServerIDs, &item.MCPServerIDs); err != nil {
		return domain.Expert{}, fmt.Errorf("decode Expert MCP Servers: %w", err)
	}
	if err := json.Unmarshal(row.SkillIDs, &item.SkillIDs); err != nil {
		return domain.Expert{}, fmt.Errorf("decode Expert Skills: %w", err)
	}
	return item, nil
}

func (repository *Repository) GetSettings(ctx context.Context, ownerID string) (domain.Settings, error) {
	var row settingsRecord
	if err := repository.db.WithContext(ctx).Where("user_id = ?", ownerID).Take(&row).Error; err != nil {
		return domain.Settings{}, mapNotFound(err)
	}
	runtime, err := domain.ParseRuntime(row.DefaultRuntimeEngine)
	if err != nil {
		return domain.Settings{}, err
	}
	defaults := map[string]string{}
	if len(row.RuntimeModelDefaults) > 0 {
		if err := json.Unmarshal(row.RuntimeModelDefaults, &defaults); err != nil {
			return domain.Settings{}, fmt.Errorf("decode Runtime model defaults: %w", err)
		}
	}
	runtimeDefaults := make(map[domain.RuntimeEngine]string, len(defaults))
	for name, modelID := range defaults {
		parsed, err := domain.ParseRuntime(name)
		if err != nil {
			return domain.Settings{}, err
		}
		runtimeDefaults[parsed] = modelID
	}
	return domain.Settings{Personality: row.Personality, PersonalityInstructions: row.PersonalityInstructions, RuntimeModelDefaults: runtimeDefaults, DefaultRuntimeEngine: runtime, Language: row.Language, Timezone: row.Timezone, Version: row.Version}, nil
}

func (repository *Repository) UpdateSettings(ctx context.Context, ownerID string, settings domain.Settings, expectedVersion int64) (domain.Settings, error) {
	if err := settings.Validate(); err != nil {
		return domain.Settings{}, err
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		defaults := make(map[string]string, len(settings.RuntimeModelDefaults))
		for runtime, modelID := range settings.RuntimeModelDefaults {
			if _, err := uuid.Parse(modelID); err != nil {
				return fmt.Errorf("%w: invalid default Provider Model", domain.ErrInvalid)
			}
			var model providerModelRecord
			if err := tx.Where("owner_user_id = ? AND id = ? AND available", ownerID, modelID).Take(&model).Error; err != nil {
				return fmt.Errorf("%w: default Provider Model is unavailable or does not belong to the User", domain.ErrInvalid)
			}
			parsed, err := providerModelDomain(model)
			if err != nil {
				return err
			}
			for _, compatibility := range parsed.Compatibility {
				if compatibility.RuntimeEngine == runtime && compatibility.Status == "incompatible" {
					return fmt.Errorf("%w: default Provider Model is incompatible with %s", domain.ErrInvalid, runtime)
				}
			}
			defaults[string(runtime)] = modelID
		}
		encodedDefaults, err := marshal(defaults)
		if err != nil {
			return err
		}
		result := tx.Model(&settingsRecord{}).Where("user_id = ? AND version = ?", ownerID, expectedVersion).Updates(map[string]any{
			"personality": settings.Personality, "personality_instructions": strings.TrimSpace(settings.PersonalityInstructions),
			"runtime_model_defaults": encodedDefaults, "default_runtime_engine": string(settings.DefaultRuntimeEngine),
			"language": settings.Language, "timezone": settings.Timezone, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return nil
	})
	if err != nil {
		return domain.Settings{}, fmt.Errorf("update Personal Settings: %w", err)
	}
	return repository.GetSettings(ctx, ownerID)
}

func (repository *Repository) ListModelProviderConnections(ctx context.Context, ownerID string) ([]domain.ModelProviderConnection, error) {
	var rows []modelProviderConnectionRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Model Provider Connections: %w", err)
	}
	items := make([]domain.ModelProviderConnection, 0, len(rows))
	for _, row := range rows {
		item, err := repository.providerConnectionDomain(ctx, row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateModelProviderConnection(ctx context.Context, ownerID string, connection domain.ModelProviderConnection, ciphertext []byte, models []domain.ProviderModel) (domain.ModelProviderConnection, error) {
	protocols, _ := marshal(connection.Protocols)
	row := modelProviderConnectionRecord{ID: uuid.NewString(), OwnerID: ownerID, Name: strings.TrimSpace(connection.Name), ProviderType: connection.ProviderType, Endpoint: strings.TrimSpace(connection.Endpoint), Protocols: protocols, APIKeyCiphertext: ciphertext, VerificationStatus: connection.VerificationStatus, CustomEndpoint: connection.CustomEndpoint, Version: 1}
	if connection.VerificationError != "" {
		row.VerificationError = &connection.VerificationError
	}
	if len(models) > 0 {
		now := time.Now().UTC()
		row.LastSyncedAt = &now
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&modelProviderCredentialVersionRecord{ConnectionID: row.ID, ConnectionVersion: 1, APIKeyCiphertext: ciphertext}).Error; err != nil {
			return err
		}
		return replaceProviderModelRows(tx, ownerID, row.ID, models)
	})
	if err != nil {
		return domain.ModelProviderConnection{}, fmt.Errorf("create Model Provider Connection: %w", err)
	}
	return repository.providerConnectionDomain(ctx, row)
}

func (repository *Repository) UpdateModelProviderConnection(ctx context.Context, ownerID, connectionID string, connection domain.ModelProviderConnection, ciphertext []byte, models []domain.ProviderModel, expectedVersion int64) (domain.ModelProviderConnection, error) {
	protocols, _ := marshal(connection.Protocols)
	updates := map[string]any{"name": strings.TrimSpace(connection.Name), "endpoint": strings.TrimSpace(connection.Endpoint), "protocols": protocols, "verification_status": connection.VerificationStatus, "verification_error": nil, "custom_endpoint": connection.CustomEndpoint, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	if connection.VerificationError != "" {
		updates["verification_error"] = connection.VerificationError
	}
	if ciphertext != nil {
		updates["api_key_ciphertext"] = ciphertext
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		credential := ciphertext
		if credential == nil {
			var existing modelProviderConnectionRecord
			if err := tx.Select("api_key_ciphertext").Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Take(&existing).Error; err != nil {
				return mapNotFound(err)
			}
			credential = existing.APIKeyCiphertext
		}
		result := tx.Model(&modelProviderConnectionRecord{}).Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, connectionID, expectedVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if err := tx.Create(&modelProviderCredentialVersionRecord{ConnectionID: connectionID, ConnectionVersion: expectedVersion + 1, APIKeyCiphertext: credential}).Error; err != nil {
			return err
		}
		if models != nil {
			return replaceProviderModelRows(tx, ownerID, connectionID, models)
		}
		return nil
	})
	if err != nil {
		return domain.ModelProviderConnection{}, fmt.Errorf("update Model Provider Connection: %w", err)
	}
	return repository.getProviderConnection(ctx, ownerID, connectionID)
}

func (repository *Repository) DeleteModelProviderConnection(ctx context.Context, ownerID, connectionID string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var modelIDs []string
		if err := tx.Model(&providerModelRecord{}).Where("owner_user_id = ? AND connection_id = ?", ownerID, connectionID).Pluck("id", &modelIDs).Error; err != nil {
			return err
		}
		if len(modelIDs) > 0 {
			var count int64
			if err := tx.Model(&sessionRecord{}).Where("owner_user_id = ? AND current_provider_model_id IN ?", ownerID, modelIDs).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Model(&workflowRecord{}).Where("owner_user_id = ? AND provider_model_id IN ?", ownerID, modelIDs).Count(&count).Error; err != nil {
					return err
				}
			}
			var settings settingsRecord
			if err := tx.Where("user_id = ?", ownerID).Take(&settings).Error; err != nil {
				return err
			}
			defaults := map[string]string{}
			_ = json.Unmarshal(settings.RuntimeModelDefaults, &defaults)
			used := make(map[string]struct{}, len(modelIDs))
			for _, id := range modelIDs {
				used[id] = struct{}{}
			}
			for _, id := range defaults {
				if _, ok := used[id]; ok {
					count++
				}
			}
			if count > 0 {
				return fmt.Errorf("%w: change Runtime, Session, and Workflow model references before deleting this connection", domain.ErrConflict)
			}
		}
		result := tx.Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Delete(&modelProviderConnectionRecord{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (repository *Repository) GetModelProviderAPIKey(ctx context.Context, ownerID, connectionID string) ([]byte, error) {
	var row modelProviderConnectionRecord
	if err := repository.db.WithContext(ctx).Select("api_key_ciphertext").Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return row.APIKeyCiphertext, nil
}

func (repository *Repository) ReplaceProviderModels(ctx context.Context, ownerID, connectionID string, models []domain.ProviderModel, verificationStatus, syncError string) (domain.ModelProviderConnection, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var connection modelProviderConnectionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Take(&connection).Error; err != nil {
			return mapNotFound(err)
		}
		if models != nil {
			if err := replaceProviderModelRows(tx, ownerID, connectionID, models); err != nil {
				return err
			}
		}
		updates := map[string]any{"verification_status": verificationStatus, "verification_error": nil, "last_sync_error": nil, "last_synced_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
		if syncError != "" {
			updates["last_sync_error"] = syncError
			delete(updates, "last_synced_at")
		}
		result := tx.Model(&modelProviderConnectionRecord{}).Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		return tx.Create(&modelProviderCredentialVersionRecord{ConnectionID: connectionID, ConnectionVersion: connection.Version + 1, APIKeyCiphertext: connection.APIKeyCiphertext}).Error
	})
	if err != nil {
		return domain.ModelProviderConnection{}, err
	}
	return repository.getProviderConnection(ctx, ownerID, connectionID)
}

func (repository *Repository) CreateProviderModel(ctx context.Context, ownerID, connectionID string, model domain.ProviderModel) (domain.ProviderModel, error) {
	compatibility, _ := marshal(model.Compatibility)
	row := providerModelRecord{ID: uuid.NewString(), ConnectionID: connectionID, OwnerID: ownerID, ModelID: strings.TrimSpace(model.ModelID), DisplayName: strings.TrimSpace(model.DisplayName), Available: true, ManuallyAdded: true, Compatibility: compatibility}
	if row.DisplayName == "" {
		row.DisplayName = row.ModelID
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ProviderModel{}, fmt.Errorf("create Provider Model: %w", err)
	}
	return providerModelDomain(row)
}

func replaceProviderModelRows(tx *gorm.DB, ownerID, connectionID string, models []domain.ProviderModel) error {
	if err := tx.Model(&providerModelRecord{}).Where("owner_user_id = ? AND connection_id = ? AND NOT manually_added", ownerID, connectionID).Update("available", false).Error; err != nil {
		return err
	}
	for _, model := range models {
		compatibility, _ := marshal(model.Compatibility)
		row := providerModelRecord{ID: uuid.NewString(), ConnectionID: connectionID, OwnerID: ownerID, ModelID: model.ModelID, DisplayName: model.DisplayName, Available: true, ManuallyAdded: model.ManuallyAdded, Compatibility: compatibility}
		if row.DisplayName == "" {
			row.DisplayName = row.ModelID
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "connection_id"}, {Name: "model_id"}}, DoUpdates: clause.AssignmentColumns([]string{"display_name", "available", "compatibility", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (repository *Repository) getProviderConnection(ctx context.Context, ownerID, connectionID string) (domain.ModelProviderConnection, error) {
	var row modelProviderConnectionRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, connectionID).Take(&row).Error; err != nil {
		return domain.ModelProviderConnection{}, mapNotFound(err)
	}
	return repository.providerConnectionDomain(ctx, row)
}

func (repository *Repository) providerConnectionDomain(ctx context.Context, row modelProviderConnectionRecord) (domain.ModelProviderConnection, error) {
	item := domain.ModelProviderConnection{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, ProviderType: row.ProviderType, Endpoint: row.Endpoint, HasAPIKey: len(row.APIKeyCiphertext) > 0, VerificationStatus: row.VerificationStatus, CustomEndpoint: row.CustomEndpoint, LastSyncedAt: row.LastSyncedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
	if row.VerificationError != nil {
		item.VerificationError = *row.VerificationError
	}
	if row.LastSyncError != nil {
		item.LastSyncError = *row.LastSyncError
	}
	if err := json.Unmarshal(row.Protocols, &item.Protocols); err != nil {
		return domain.ModelProviderConnection{}, err
	}
	var models []providerModelRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND connection_id = ?", row.OwnerID, row.ID).Order("available DESC, display_name, model_id").Find(&models).Error; err != nil {
		return domain.ModelProviderConnection{}, err
	}
	for _, model := range models {
		parsed, err := providerModelDomain(model)
		if err != nil {
			return domain.ModelProviderConnection{}, err
		}
		item.Models = append(item.Models, parsed)
	}
	return item, nil
}

func providerModelDomain(row providerModelRecord) (domain.ProviderModel, error) {
	item := domain.ProviderModel{ID: row.ID, ConnectionID: row.ConnectionID, ModelID: row.ModelID, DisplayName: row.DisplayName, Available: row.Available, ManuallyAdded: row.ManuallyAdded}
	if err := json.Unmarshal(row.Compatibility, &item.Compatibility); err != nil {
		return domain.ProviderModel{}, err
	}
	return item, nil
}

func (repository *Repository) ListMCPServers(ctx context.Context, ownerID string) ([]domain.MCPServer, error) {
	var rows []mcpRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list MCP Servers: %w", err)
	}
	items := make([]domain.MCPServer, 0, len(rows))
	for _, row := range rows {
		item, err := mcpDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateMCPServer(ctx context.Context, ownerID string, server domain.MCPServer, secretCiphertext []byte) (domain.MCPServer, error) {
	if err := server.Validate(); err != nil {
		return domain.MCPServer{}, err
	}
	configuration, err := encodeMCPConfiguration(server)
	if err != nil {
		return domain.MCPServer{}, err
	}
	row := mcpRecord{ID: uuid.NewString(), OwnerID: ownerID, Name: strings.TrimSpace(server.Name), Transport: server.Transport, Configuration: configuration, SecretCiphertext: secretCiphertext, Version: 1}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.MCPServer{}, fmt.Errorf("create MCP Server: %w", err)
	}
	return mcpDomain(row)
}

func (repository *Repository) UpdateMCPServer(ctx context.Context, ownerID, serverID string, server domain.MCPServer, secretCiphertext []byte, expectedVersion int64) (domain.MCPServer, error) {
	if err := server.Validate(); err != nil {
		return domain.MCPServer{}, err
	}
	configuration, err := encodeMCPConfiguration(server)
	if err != nil {
		return domain.MCPServer{}, err
	}
	updates := map[string]any{
		"name": strings.TrimSpace(server.Name), "transport": server.Transport, "configuration": configuration,
		"test_requested_at": nil, "tested_at": nil, "test_error": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
	}
	if secretCiphertext != nil {
		updates["secret_ciphertext"] = secretCiphertext
	}
	result := repository.db.WithContext(ctx).Model(&mcpRecord{}).Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, serverID, expectedVersion).Updates(updates)
	if result.Error != nil {
		return domain.MCPServer{}, fmt.Errorf("update MCP Server: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.MCPServer{}, domain.ErrConflict
	}
	return repository.getMCP(ctx, ownerID, serverID)
}

func (repository *Repository) RequestMCPTest(ctx context.Context, ownerID, serverID string) (domain.MCPServer, error) {
	result := repository.db.WithContext(ctx).Model(&mcpRecord{}).
		Where("owner_user_id = ? AND id = ? AND test_requested_at IS NULL", ownerID, serverID).
		Updates(map[string]any{"test_requested_at": gorm.Expr("now()"), "tested_at": nil, "test_error": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return domain.MCPServer{}, fmt.Errorf("request MCP test: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := repository.db.WithContext(ctx).Model(&mcpRecord{}).Where("owner_user_id = ? AND id = ?", ownerID, serverID).Count(&count).Error; err != nil {
			return domain.MCPServer{}, err
		}
		if count == 0 {
			return domain.MCPServer{}, domain.ErrNotFound
		}
	}
	return repository.getMCP(ctx, ownerID, serverID)
}

func (repository *Repository) SetMCPTestResult(ctx context.Context, ownerID, serverID, testError string) (domain.MCPServer, error) {
	updates := map[string]any{"test_requested_at": nil, "tested_at": gorm.Expr("now()"), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	if testError == "" {
		updates["test_error"] = nil
	} else {
		updates["test_error"] = testError
	}
	result := repository.db.WithContext(ctx).Model(&mcpRecord{}).Where("owner_user_id = ? AND id = ?", ownerID, serverID).Updates(updates)
	if result.Error != nil {
		return domain.MCPServer{}, fmt.Errorf("record MCP test result: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.MCPServer{}, domain.ErrNotFound
	}
	return repository.getMCP(ctx, ownerID, serverID)
}

func (repository *Repository) DeleteMCPServer(ctx context.Context, ownerID, serverID string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var experts []expertRecord
		if err := tx.Where("owner_user_id = ?", ownerID).Find(&experts).Error; err != nil {
			return err
		}
		for _, expert := range experts {
			var ids []string
			if err := json.Unmarshal(expert.MCPServerIDs, &ids); err != nil {
				return err
			}
			filtered := removeID(ids, serverID)
			if len(filtered) != len(ids) {
				encoded, _ := marshal(filtered)
				if err := tx.Model(&expertRecord{}).Where("id = ?", expert.ID).Updates(map[string]any{"mcp_server_ids": encoded, "version": gorm.Expr("version + 1"), "updated_at": gorm.Expr("now()")}).Error; err != nil {
					return err
				}
			}
		}
		result := tx.Where("owner_user_id = ? AND id = ?", ownerID, serverID).Delete(&mcpRecord{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (repository *Repository) getMCP(ctx context.Context, ownerID, serverID string) (domain.MCPServer, error) {
	var row mcpRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, serverID).Take(&row).Error; err != nil {
		return domain.MCPServer{}, mapNotFound(err)
	}
	return mcpDomain(row)
}

func (repository *Repository) GetMCPSecret(ctx context.Context, ownerID, serverID string) ([]byte, error) {
	var row mcpRecord
	if err := repository.db.WithContext(ctx).Select("secret_ciphertext").Where("owner_user_id = ? AND id = ?", ownerID, serverID).Take(&row).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return append([]byte(nil), row.SecretCiphertext...), nil
}

func encodeMCPConfiguration(server domain.MCPServer) ([]byte, error) {
	return marshal(map[string]any{"url": server.URL, "runner": server.Runner, "package": server.Package, "package_version": server.PackageVersion, "arguments": server.Arguments, "environment": server.Environment})
}

func mcpDomain(row mcpRecord) (domain.MCPServer, error) {
	var configuration struct {
		URL            *string                      `json:"url"`
		Runner         *string                      `json:"runner"`
		Package        *string                      `json:"package"`
		PackageVersion *string                      `json:"package_version"`
		Arguments      []string                     `json:"arguments"`
		Environment    []domain.EnvironmentVariable `json:"environment"`
	}
	if err := json.Unmarshal(row.Configuration, &configuration); err != nil {
		return domain.MCPServer{}, fmt.Errorf("decode MCP configuration: %w", err)
	}
	item := domain.MCPServer{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Transport: row.Transport, URL: configuration.URL, Runner: configuration.Runner, Package: configuration.Package, PackageVersion: configuration.PackageVersion, Arguments: configuration.Arguments, Environment: configuration.Environment, TestRequestedAt: row.TestRequestedAt, TestedAt: row.TestedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
	if row.TestError != nil {
		item.TestError = *row.TestError
	}
	return item, nil
}

func (repository *Repository) ListSkills(ctx context.Context, ownerID string) ([]domain.Skill, error) {
	var rows []skillRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Order("updated_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Skills: %w", err)
	}
	items := make([]domain.Skill, 0, len(rows))
	for _, row := range rows {
		items = append(items, skillDomain(row))
	}
	return items, nil
}

func (repository *Repository) CreateSkill(ctx context.Context, ownerID string, skill domain.Skill) (domain.Skill, error) {
	row := skillRecord{ID: uuid.NewString(), OwnerID: ownerID, Name: strings.TrimSpace(skill.Name), Source: skill.Source, GitURL: skill.GitURL, GitRef: skill.GitRef, ObjectKey: skill.ObjectKey, SHA256: skill.SHA256, Version: 1}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.Skill{}, fmt.Errorf("create Skill: %w", err)
	}
	return skillDomain(row), nil
}

func (repository *Repository) UpdateSkill(ctx context.Context, ownerID, skillID string, gitRef *string, objectKey, sha256 string, expectedVersion int64) (domain.Skill, error) {
	updates := map[string]any{"object_key": objectKey, "sha256": sha256, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	if gitRef != nil {
		updates["git_ref"] = gitRef
	}
	result := repository.db.WithContext(ctx).Model(&skillRecord{}).Where("owner_user_id = ? AND id = ? AND version = ?", ownerID, skillID, expectedVersion).Updates(updates)
	if result.Error != nil {
		return domain.Skill{}, fmt.Errorf("update Skill: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.Skill{}, domain.ErrConflict
	}
	var row skillRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND id = ?", ownerID, skillID).Take(&row).Error; err != nil {
		return domain.Skill{}, mapNotFound(err)
	}
	return skillDomain(row), nil
}

func (repository *Repository) DeleteSkill(ctx context.Context, ownerID, skillID string) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var experts []expertRecord
		if err := tx.Where("owner_user_id = ?", ownerID).Find(&experts).Error; err != nil {
			return err
		}
		for _, expert := range experts {
			var ids []string
			if err := json.Unmarshal(expert.SkillIDs, &ids); err != nil {
				return err
			}
			filtered := removeID(ids, skillID)
			if len(filtered) != len(ids) {
				encoded, _ := marshal(filtered)
				if err := tx.Model(&expertRecord{}).Where("id = ?", expert.ID).Updates(map[string]any{"skill_ids": encoded, "version": gorm.Expr("version + 1"), "updated_at": gorm.Expr("now()")}).Error; err != nil {
					return err
				}
			}
		}
		result := tx.Where("owner_user_id = ? AND id = ?", ownerID, skillID).Delete(&skillRecord{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func skillDomain(row skillRecord) domain.Skill {
	return domain.Skill{ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Source: row.Source, GitURL: row.GitURL, GitRef: row.GitRef, ObjectKey: row.ObjectKey, SHA256: row.SHA256, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func removeID(values []string, target string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
