package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) ListCLIConnectorDefinitions(ctx context.Context, includeUnpublished bool) ([]cliconnector.Definition, error) {
	query := repository.db.WithContext(ctx).Order("name, id")
	if !includeUnpublished {
		query = query.Where("state = ?", cliconnector.StateAvailable)
	}
	var rows []cliConnectorDefinitionRecord
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list CLI Connector Definitions: %w", err)
	}
	var conformances []struct {
		DefinitionID  string `gorm:"column:definition_id"`
		RuntimeDigest string `gorm:"column:runtime_repo_digest"`
	}
	if err := repository.db.WithContext(ctx).Table("cli_connector_conformance c").
		Select("c.definition_id, c.runtime_repo_digest").
		Joins("JOIN cli_connector_definitions d ON d.id = c.definition_id AND d.bundle_sha256 = c.bundle_sha256 AND d.deleted_at IS NULL").
		Where("c.passed = ?", true).Order("c.runtime_repo_digest").Scan(&conformances).Error; err != nil {
		return nil, fmt.Errorf("list CLI Connector conformance: %w", err)
	}
	runtimeDigests := make(map[string][]string)
	for _, row := range conformances {
		runtimeDigests[row.DefinitionID] = append(runtimeDigests[row.DefinitionID], row.RuntimeDigest)
	}
	items := make([]cliconnector.Definition, 0, len(rows))
	for _, row := range rows {
		item, err := cliDefinitionDomain(row)
		if err != nil {
			return nil, err
		}
		item.RuntimeDigests = runtimeDigests[item.ID]
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) ListCLIConnectorHealth(ctx context.Context, now time.Time) ([]cliconnector.Health, error) {
	var rows []struct {
		DefinitionID                string `gorm:"column:definition_id"`
		DefinitionName              string `gorm:"column:definition_name"`
		DefinitionState             string `gorm:"column:definition_state"`
		EnablementCount             int64  `gorm:"column:enablement_count"`
		EnabledCount                int64  `gorm:"column:enabled_count"`
		WaitingForUserCount         int64  `gorm:"column:waiting_for_user_count"`
		ActiveAuthorizationCount    int64  `gorm:"column:active_authorization_count"`
		AttentionAuthorizationCount int64  `gorm:"column:attention_authorization_count"`
	}
	err := repository.db.WithContext(ctx).Raw(`
		SELECT d.id AS definition_id,
		       d.name AS definition_name,
		       d.state AS definition_state,
		       (SELECT COUNT(*) FROM cli_connector_enablements e WHERE e.definition_id = d.id) AS enablement_count,
		       (SELECT COUNT(*) FROM cli_connector_enablements e WHERE e.definition_id = d.id AND e.state = 'enabled') AS enabled_count,
		       (SELECT COUNT(*) FROM cli_connector_enablements e WHERE e.definition_id = d.id AND e.state = 'waiting_for_user')
		         + (SELECT COUNT(*) FROM cli_connector_authorization_attempts aa JOIN cli_connector_enablements e ON e.id = aa.enablement_id WHERE e.definition_id = d.id AND aa.expires_at > ?) AS waiting_for_user_count,
		       (SELECT COUNT(*) FROM cli_connector_authorizations a JOIN cli_connector_enablements e ON e.id = a.enablement_id WHERE e.definition_id = d.id AND a.state = 'active' AND (a.expires_at IS NULL OR a.expires_at > ?)) AS active_authorization_count,
		       (SELECT COUNT(*) FROM cli_connector_authorizations a JOIN cli_connector_enablements e ON e.id = a.enablement_id WHERE e.definition_id = d.id AND (a.state = 'invalid' OR (a.state = 'active' AND a.expires_at IS NOT NULL AND a.expires_at <= ?))) AS attention_authorization_count
		FROM cli_connector_definitions d
		WHERE d.deleted_at IS NULL
		ORDER BY d.name, d.id`, now, now, now).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list CLI Connector health: %w", err)
	}
	items := make([]cliconnector.Health, 0, len(rows))
	for _, row := range rows {
		items = append(items, cliconnector.Health{
			DefinitionID: row.DefinitionID, DefinitionName: row.DefinitionName, DefinitionState: cliconnector.State(row.DefinitionState),
			EnablementCount: row.EnablementCount, EnabledCount: row.EnabledCount, WaitingForUserCount: row.WaitingForUserCount,
			ActiveAuthorizationCount: row.ActiveAuthorizationCount, AttentionAuthorizationCount: row.AttentionAuthorizationCount,
		})
	}
	return items, nil
}

func (repository *Repository) CreateCLIConnectorDefinition(ctx context.Context, administratorID string, input cliconnector.Definition) (cliconnector.Definition, error) {
	if err := input.ValidateDraft(); err != nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	capabilities, _ := json.Marshal(input.Capabilities)
	architectures, _ := json.Marshal(input.SupportedArchitectures)
	recommendedSkills, _ := json.Marshal(input.RecommendedSkills)
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}
	row := cliConnectorDefinitionRecord{ID: id, Name: input.Name, Icon: input.Icon, Description: input.Description, InstallationType: input.InstallationType, NPMPackage: input.Package, NPMVersion: input.Version, NPMIntegrity: input.Integrity, Executable: input.Executable, AuthenticationDriver: input.AuthenticationDriver, Capabilities: capabilities, SupportedArchitectures: architectures, RecommendedSkillIDs: []byte(`[]`), RecommendedSkills: recommendedSkills, State: string(cliconnector.StateDraft), CreatedByUserID: administratorID, Version: 1}
	if input.SourceObjectKey != "" {
		row.SourceObjectKey, row.SourceSHA256 = &input.SourceObjectKey, &input.SourceSHA256
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return cliconnector.Definition{}, fmt.Errorf("create CLI Connector Definition: %w", err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) UpdateCLIConnectorDefinition(ctx context.Context, id string, input cliconnector.Definition, expectedVersion int64) (cliconnector.Definition, error) {
	if err := input.ValidateDraft(); err != nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	capabilities, _ := json.Marshal(input.Capabilities)
	architectures, _ := json.Marshal(input.SupportedArchitectures)
	recommendedSkills, _ := json.Marshal(input.RecommendedSkills)
	updates := map[string]any{"name": input.Name, "icon": input.Icon, "description": input.Description, "installation_type": input.InstallationType, "npm_package": input.Package, "npm_version": input.Version, "npm_integrity": input.Integrity, "executable": input.Executable, "authentication_driver": input.AuthenticationDriver, "capabilities": capabilities, "supported_architectures": architectures, "recommended_skills": recommendedSkills, "state": "draft", "failure_reason": nil, "bundle_object_key": nil, "bundle_sha256": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}
	if input.SourceObjectKey != "" {
		updates["source_object_key"], updates["source_sha256"] = input.SourceObjectKey, input.SourceSHA256
	} else {
		updates["source_object_key"], updates["source_sha256"] = nil, nil
	}
	result := repository.db.WithContext(ctx).Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ? AND state IN ?", id, expectedVersion, []string{"draft", "failed", "available", "disabled"}).Updates(updates)
	if result.Error != nil {
		return cliconnector.Definition{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.Definition{}, domain.ErrConflict
	}
	var row cliConnectorDefinitionRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		return cliconnector.Definition{}, mapNotFound(err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) PublishCLIConnectorDefinition(ctx context.Context, id string, expectedVersion int64) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Version != expectedVersion || (row.State != string(cliconnector.StateDraft) && row.State != string(cliconnector.StateFailed)) {
			return domain.ErrConflict
		}
		definition, err := cliDefinitionDomain(row)
		if err != nil {
			return err
		}
		if err := definition.ValidateDraft(); err != nil {
			return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
		}
		result := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(map[string]any{"state": string(cliconnector.StateBuilding), "failure_reason": nil, "bundle_object_key": nil, "bundle_sha256": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.Where("id = ?", id).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Definition{}, err
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) DisableCLIConnectorDefinition(ctx context.Context, id string, expectedVersion int64) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ? AND state = ?", id, expectedVersion, cliconnector.StateAvailable).Updates(map[string]any{"state": string(cliconnector.StateDisabled), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if err := tx.Model(&cliConnectorEnablementRecord{}).Where("definition_id = ? AND state <> 'disabled'", id).Updates(map[string]any{"state": "disabled", "action_url": nil, "action_expires_at": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Definition{}, err
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) DeleteCLIConnectorDefinition(ctx context.Context, id string, expectedVersion int64) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var definition cliConnectorDefinitionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&definition).Error; err != nil {
			return mapNotFound(err)
		}
		if definition.Version != expectedVersion {
			return domain.ErrConflict
		}
		// Keep historical snapshots and the User's Feishu Application, but revoke access.
		if err := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ?", id).Updates(map[string]any{
			"state": "disabled", "deleted_at": time.Now().UTC(), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&cliConnectorEnablementRecord{}).Where("definition_id = ?", id).Updates(map[string]any{
			"state": "disabled", "action_url": nil, "action_expires_at": nil, "registration_device_code_ciphertext": nil,
			"updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		enablements := tx.Model(&cliConnectorEnablementRecord{}).Select("id").Where("definition_id = ?", id)
		if err := tx.Where("enablement_id IN (?)", enablements).Delete(&cliConnectorAuthorizationAttemptRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&cliConnectorAuthorizationRecord{}).Where("enablement_id IN (?)", enablements).Updates(map[string]any{
			"state": "disconnected", "token_ciphertext": nil, "refresh_token_ciphertext": nil,
			"updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&expertRecord{}).Where("jsonb_exists(cli_connector_definition_ids, ?)", id).Updates(map[string]any{
			"cli_connector_definition_ids": gorm.Expr("cli_connector_definition_ids - ?::text", id),
			"updated_at":                   gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		}).Error
	})
}

func (repository *Repository) GetAvailableCLIConnectorDefinition(ctx context.Context, definitionID string) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND state = ?", definitionID, cliconnector.StateAvailable).Take(&row).Error; err != nil {
		return cliconnector.Definition{}, mapNotFound(err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) GetCLIConnectorEnablement(ctx context.Context, ownerID, definitionID string) (cliconnector.Enablement, error) {
	var row cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error; err != nil {
		return cliconnector.Enablement{}, mapNotFound(err)
	}
	item := cliEnablementDomain(row)
	if err := repository.attachFeishuApplication(ctx, ownerID, &item); err != nil {
		return cliconnector.Enablement{}, err
	}
	return item, nil
}

func (repository *Repository) EnableCLIConnector(ctx context.Context, ownerID, definitionID string) (cliconnector.Enablement, error) {
	var row cliConnectorEnablementRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var definition cliConnectorDefinitionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND state = 'available'", definitionID).Take(&definition).Error; err != nil {
			return mapNotFound(err)
		}
		var application feishuCLIApplicationRecord
		if definition.AuthenticationDriver == "feishu" {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ?", ownerID).Take(&application).Error; err != nil {
				return mapNotFound(err)
			}
			var previous cliConnectorEnablementRecord
			if err := tx.Where("id = ?", application.EnablementID).Take(&previous).Error; err != nil {
				return err
			}
			if previous.DefinitionID != definitionID {
				var count int64
				if err := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ?", previous.DefinitionID).Count(&count).Error; err != nil {
					return err
				}
				if count != 0 {
					return domain.ErrConflict
				}
			}
		} else if definition.AuthenticationDriver != "none" {
			return domain.ErrInvalid
		}
		row = cliConnectorEnablementRecord{ID: uuid.NewString(), OwnerID: ownerID, DefinitionID: definitionID, State: "enabled", Version: 1}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "definition_id"}}, DoUpdates: clause.Assignments(map[string]any{
			"state": "enabled", "action_url": nil, "action_expires_at": nil, "registration_device_code_ciphertext": nil,
			"updated_at": gorm.Expr("now()"), "version": gorm.Expr("cli_connector_enablements.version + 1"),
		}), Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "cli_connector_enablements.state <> 'enabled'"}}}}).Create(&row).Error; err != nil {
			return err
		}
		row = cliConnectorEnablementRecord{}
		if err := tx.Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error; err != nil {
			return err
		}
		if application.ID != "" && application.EnablementID != row.ID {
			return tx.Model(&feishuCLIApplicationRecord{}).Where("id = ?", application.ID).Updates(map[string]any{"enablement_id": row.ID, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error
		}
		return nil
	})
	if err != nil {
		return cliconnector.Enablement{}, err
	}
	item := cliEnablementDomain(row)
	if err := repository.attachFeishuApplication(ctx, ownerID, &item); err != nil {
		return cliconnector.Enablement{}, err
	}
	return item, nil
}

func (repository *Repository) DisableCLIConnector(ctx context.Context, ownerID, definitionID string, expectedVersion int64) (cliconnector.Enablement, error) {
	if ownerID == "" || definitionID == "" || expectedVersion < 1 {
		return cliconnector.Enablement{}, domain.ErrInvalid
	}
	var row cliConnectorEnablementRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Version != expectedVersion {
			return domain.ErrConflict
		}
		if row.State == "disabled" {
			return nil
		}
		result := tx.Model(&cliConnectorEnablementRecord{}).Where("id = ? AND version = ?", row.ID, expectedVersion).Updates(map[string]any{
			"state": "disabled", "action_url": nil, "action_expires_at": nil, "registration_device_code_ciphertext": nil,
			"updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if err := tx.Where("enablement_id = ?", row.ID).Delete(&cliConnectorAuthorizationAttemptRecord{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Enablement{}, err
	}
	return cliEnablementDomain(row), nil
}

func (repository *Repository) BeginFeishuCLIConnectorEnablement(ctx context.Context, ownerID, definitionID, actionURL string, expiry time.Time, deviceCodeCiphertext []byte) (cliconnector.Enablement, error) {
	if ownerID == "" || actionURL == "" || !expiry.After(time.Now().UTC()) || len(deviceCodeCiphertext) == 0 {
		return cliconnector.Enablement{}, domain.ErrInvalid
	}
	var row cliConnectorEnablementRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var definition cliConnectorDefinitionRecord
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND state = 'available' AND authentication_driver = 'feishu'", definitionID).Take(&definition).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error; err == nil {
			if row.State == "enabled" || row.State == "waiting_for_user" && row.ActionExpiresAt != nil && row.ActionExpiresAt.After(time.Now().UTC()) {
				return nil
			}
			result := tx.Model(&cliConnectorEnablementRecord{}).Where("id = ? AND version = ?", row.ID, row.Version).Updates(map[string]any{
				"state": "waiting_for_user", "action_url": actionURL, "action_expires_at": expiry,
				"registration_device_code_ciphertext": deviceCodeCiphertext, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
			})
			if result.Error != nil || result.RowsAffected != 1 {
				if result.Error != nil {
					return result.Error
				}
				return domain.ErrConflict
			}
			return tx.Where("id = ?", row.ID).Take(&row).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = newCLIConnectorEnablement(ownerID, definitionID, "feishu", actionURL, expiry)
		row.RegistrationDeviceCodeCiphertext = append([]byte(nil), deviceCodeCiphertext...)
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "definition_id"}}, DoNothing: true}).Create(&row)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return tx.Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error
		}
		return nil
	})
	if err != nil {
		return cliconnector.Enablement{}, err
	}
	return cliEnablementDomain(row), nil
}

func (repository *Repository) GetFeishuCLIConnectorRegistration(ctx context.Context, ownerID, enablementID string) (cliconnector.EnablementRegistration, error) {
	var row cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND state = 'waiting_for_user'", enablementID, ownerID).Take(&row).Error; err != nil {
		return cliconnector.EnablementRegistration{}, mapNotFound(err)
	}
	if row.ActionExpiresAt == nil || !time.Now().UTC().Before(*row.ActionExpiresAt) || len(row.RegistrationDeviceCodeCiphertext) == 0 {
		return cliconnector.EnablementRegistration{}, domain.ErrConflict
	}
	return cliconnector.EnablementRegistration{Enablement: cliEnablementDomain(row), DeviceCodeCiphertext: append([]byte(nil), row.RegistrationDeviceCodeCiphertext...)}, nil
}

func (repository *Repository) CompleteFeishuCLIConnectorEnablement(ctx context.Context, ownerID, enablementID string, appIDCiphertext, appSecretCiphertext []byte, providerName, consoleURL string) (cliconnector.Enablement, error) {
	if len(appIDCiphertext) == 0 || len(appSecretCiphertext) == 0 || providerName == "" || consoleURL == "" {
		return cliconnector.Enablement{}, domain.ErrInvalid
	}
	var row cliConnectorEnablementRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND state = 'waiting_for_user'", enablementID, ownerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		application := feishuCLIApplicationRecord{
			ID: uuid.NewString(), OwnerID: ownerID, EnablementID: enablementID,
			ProviderApplicationIDCiphertext: append([]byte(nil), appIDCiphertext...), ProviderApplicationSecretCiphertext: append([]byte(nil), appSecretCiphertext...),
			ProviderName: providerName, DeveloperConsoleURL: consoleURL, GrantedScopes: []byte(`[]`), Version: 1,
		}
		if err := tx.Create(&application).Error; err != nil {
			return err
		}
		result := tx.Model(&cliConnectorEnablementRecord{}).Where("id = ? AND state = 'waiting_for_user'", row.ID).Updates(map[string]any{
			"state": "enabled", "action_url": nil, "action_expires_at": nil, "registration_device_code_ciphertext": nil,
			"updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return domain.ErrConflict
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Enablement{}, err
	}
	item := cliEnablementDomain(row)
	item.ProviderName, item.DeveloperConsoleURL = providerName, consoleURL
	return item, nil
}

func (repository *Repository) InvalidateCLIConnectorEnablement(ctx context.Context, ownerID, enablementID string) (cliconnector.Enablement, error) {
	result := repository.db.WithContext(ctx).Model(&cliConnectorEnablementRecord{}).
		Where("id = ? AND owner_user_id = ? AND state = 'waiting_for_user'", enablementID, ownerID).
		Updates(map[string]any{"state": "invalid", "action_url": nil, "action_expires_at": nil, "registration_device_code_ciphertext": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return cliconnector.Enablement{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.Enablement{}, domain.ErrConflict
	}
	var row cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", enablementID).Take(&row).Error; err != nil {
		return cliconnector.Enablement{}, err
	}
	return cliEnablementDomain(row), nil
}

func newCLIConnectorEnablement(ownerID, definitionID, authenticationDriver, actionURL string, expiry time.Time) cliConnectorEnablementRecord {
	row := cliConnectorEnablementRecord{ID: uuid.NewString(), OwnerID: ownerID, DefinitionID: definitionID, State: "enabled", Version: 1}
	if authenticationDriver != "none" {
		row.State = "waiting_for_user"
		row.ActionURL = &actionURL
		row.ActionExpiresAt = &expiry
	}
	return row
}

func (repository *Repository) ListCLIConnectorEnablements(ctx context.Context, ownerID string) ([]cliconnector.Enablement, error) {
	var rows []cliConnectorEnablementRecord
	definitions := repository.db.Model(&cliConnectorDefinitionRecord{}).Select("id")
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND definition_id IN (?)", ownerID, definitions).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]cliconnector.Enablement, 0, len(rows))
	for _, row := range rows {
		item := cliEnablementDomain(row)
		if err := repository.attachFeishuApplication(ctx, ownerID, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) attachFeishuApplication(ctx context.Context, ownerID string, item *cliconnector.Enablement) error {
	if item == nil || item.State != "enabled" {
		return nil
	}
	var application feishuCLIApplicationRecord
	err := repository.db.WithContext(ctx).Select("provider_name", "developer_console_url").Where("owner_user_id = ? AND enablement_id = ?", ownerID, item.ID).Take(&application).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	item.ProviderName, item.DeveloperConsoleURL = application.ProviderName, application.DeveloperConsoleURL
	return nil
}

func (repository *Repository) GetFeishuCLIApplicationCredentials(ctx context.Context, ownerID, enablementID string) (cliconnector.FeishuApplicationCredentials, error) {
	var row feishuCLIApplicationRecord
	if err := repository.db.WithContext(ctx).Select("provider_application_id_ciphertext", "provider_application_secret_ciphertext").Where("owner_user_id = ? AND enablement_id = ?", ownerID, enablementID).Take(&row).Error; err != nil {
		return cliconnector.FeishuApplicationCredentials{}, mapNotFound(err)
	}
	return cliconnector.FeishuApplicationCredentials{
		AppIDCiphertext: append([]byte(nil), row.ProviderApplicationIDCiphertext...), AppSecretCiphertext: append([]byte(nil), row.ProviderApplicationSecretCiphertext...),
	}, nil
}

func (repository *Repository) GetCLIConnectorAuthorizationPolicy(ctx context.Context, ownerID, enablementID string) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	err := repository.db.WithContext(ctx).Table("cli_connector_definitions AS definitions").
		Select("definitions.*").
		Joins("JOIN cli_connector_enablements AS enablements ON enablements.definition_id = definitions.id").
		Where("enablements.id = ? AND enablements.owner_user_id = ? AND enablements.state = 'enabled' AND definitions.state = 'available'", enablementID, ownerID).
		Take(&row).Error
	if err != nil {
		return cliconnector.Definition{}, mapNotFound(err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) BeginCLIConnectorAuthorization(ctx context.Context, ownerID, enablementID string, identity cliconnector.Identity, scopes []string, actionURL string, expiresAt time.Time, deviceCodeCiphertext []byte) (cliconnector.AuthorizationAttempt, error) {
	if identity != cliconnector.IdentityUser || actionURL == "" || !expiresAt.After(time.Now().UTC()) || len(deviceCodeCiphertext) == 0 {
		return cliconnector.AuthorizationAttempt{}, domain.ErrInvalid
	}
	encodedScopes, _ := json.Marshal(scopes)
	row := cliConnectorAuthorizationAttemptRecord{
		ID: uuid.NewString(), OwnerID: ownerID, EnablementID: enablementID, Identity: string(identity), Scopes: encodedScopes,
		DeviceCodeCiphertext: append([]byte(nil), deviceCodeCiphertext...), ActionURL: actionURL, ExpiresAt: expiresAt,
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enablement cliConnectorEnablementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND state = 'enabled'", enablementID, ownerID).Take(&enablement).Error; err != nil {
			return mapNotFound(err)
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "owner_user_id"}, {Name: "enablement_id"}, {Name: "identity"}},
			DoUpdates: clause.Assignments(map[string]any{"scopes": encodedScopes, "device_code_ciphertext": deviceCodeCiphertext, "action_url": actionURL, "expires_at": expiresAt, "updated_at": gorm.Expr("now()")}),
		}).Create(&row).Error
	})
	if err != nil {
		return cliconnector.AuthorizationAttempt{}, err
	}
	var stored cliConnectorAuthorizationAttemptRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND enablement_id = ? AND identity = ?", ownerID, enablementID, identity).Take(&stored).Error; err != nil {
		return cliconnector.AuthorizationAttempt{}, err
	}
	return cliAuthorizationAttemptDomain(stored)
}

func (repository *Repository) GetCLIConnectorAuthorizationAttempt(ctx context.Context, ownerID, attemptID string) (cliconnector.AuthorizationAttempt, error) {
	var row cliConnectorAuthorizationAttemptRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", attemptID, ownerID).Take(&row).Error; err != nil {
		return cliconnector.AuthorizationAttempt{}, mapNotFound(err)
	}
	if !time.Now().UTC().Before(row.ExpiresAt) {
		return cliconnector.AuthorizationAttempt{}, domain.ErrConflict
	}
	return cliAuthorizationAttemptDomain(row)
}

func (repository *Repository) CompleteCLIConnectorAuthorization(ctx context.Context, attempt cliconnector.AuthorizationAttempt, externalID, displayName string, scopes []string, tokenCiphertext, refreshTokenCiphertext []byte, expiresAt time.Time) (cliconnector.Authorization, error) {
	if attempt.Identity != cliconnector.IdentityUser || externalID == "" || displayName == "" || len(tokenCiphertext) == 0 || !expiresAt.After(time.Now().UTC()) {
		return cliconnector.Authorization{}, domain.ErrInvalid
	}
	encodedScopes, _ := json.Marshal(scopes)
	row := cliConnectorAuthorizationRecord{
		ID: uuid.NewString(), OwnerID: attempt.OwnerID, EnablementID: attempt.EnablementID, Identity: string(attempt.Identity),
		ExternalIdentityID: externalID, ExternalDisplayName: displayName, Scopes: encodedScopes, TokenCiphertext: append([]byte(nil), tokenCiphertext...),
		RefreshTokenCiphertext: append([]byte(nil), refreshTokenCiphertext...), ExpiresAt: &expiresAt, State: "active", Version: 1,
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var enablement cliConnectorEnablementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND state = 'enabled'", attempt.EnablementID, attempt.OwnerID).Take(&enablement).Error; err != nil {
			return mapNotFound(err)
		}
		result := tx.Where("id = ? AND owner_user_id = ?", attempt.ID, attempt.OwnerID).Delete(&cliConnectorAuthorizationAttemptRecord{})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return domain.ErrConflict
		}
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "enablement_id"}, {Name: "identity"}, {Name: "external_identity_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"external_display_name": displayName, "scopes": encodedScopes, "token_ciphertext": tokenCiphertext,
				"refresh_token_ciphertext": refreshTokenCiphertext, "expires_at": expiresAt, "state": "active", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("cli_connector_authorizations.version + 1"),
			}),
		}).Create(&row).Error
	})
	if err != nil {
		return cliconnector.Authorization{}, err
	}
	var stored cliConnectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND enablement_id = ? AND identity = ? AND external_identity_id = ?", attempt.OwnerID, attempt.EnablementID, attempt.Identity, externalID).Take(&stored).Error; err != nil {
		return cliconnector.Authorization{}, err
	}
	return cliAuthorizationDomain(stored)
}

func (repository *Repository) DeleteCLIConnectorAuthorizationAttempt(ctx context.Context, ownerID, attemptID string) error {
	return repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", attemptID, ownerID).Delete(&cliConnectorAuthorizationAttemptRecord{}).Error
}

func (repository *Repository) ListCLIConnectorAuthorizations(ctx context.Context, ownerID, enablementID string) ([]cliconnector.Authorization, error) {
	var rows []cliConnectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND enablement_id = ?", ownerID, enablementID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]cliconnector.Authorization, 0, len(rows))
	for _, row := range rows {
		item, err := cliAuthorizationDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) DisconnectCLIConnectorAuthorization(ctx context.Context, ownerID, authorizationID string, expectedVersion int64) (cliconnector.Authorization, error) {
	result := repository.db.WithContext(ctx).Model(&cliConnectorAuthorizationRecord{}).Where("id = ? AND owner_user_id = ? AND version = ? AND state <> 'disconnected'", authorizationID, ownerID, expectedVersion).Updates(map[string]any{
		"state": "disconnected", "token_ciphertext": nil, "refresh_token_ciphertext": nil, "expires_at": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
	})
	if result.Error != nil {
		return cliconnector.Authorization{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.Authorization{}, domain.ErrConflict
	}
	var row cliConnectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", authorizationID, ownerID).Take(&row).Error; err != nil {
		return cliconnector.Authorization{}, mapNotFound(err)
	}
	return cliAuthorizationDomain(row)
}

func (repository *Repository) ResolveCLIConnectorExecutionCredentials(ctx context.Context, ownerID, definitionID string, identity cliconnector.Identity, requiredScopes []string) (cliconnector.EncryptedExecutionCredentials, error) {
	var enablement cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND definition_id = ? AND state = 'enabled'", ownerID, definitionID).Take(&enablement).Error; err != nil {
		return cliconnector.EncryptedExecutionCredentials{}, mapNotFound(err)
	}
	var application feishuCLIApplicationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND enablement_id = ?", ownerID, enablement.ID).Take(&application).Error; err != nil {
		return cliconnector.EncryptedExecutionCredentials{}, mapNotFound(err)
	}
	result := cliconnector.EncryptedExecutionCredentials{
		AppIDCiphertext: append([]byte(nil), application.ProviderApplicationIDCiphertext...), AppSecretCiphertext: append([]byte(nil), application.ProviderApplicationSecretCiphertext...), EnablementID: enablement.ID,
	}
	if identity == cliconnector.IdentityBot {
		var granted []string
		if err := json.Unmarshal(application.GrantedScopes, &granted); err != nil || !containsAllScopes(granted, requiredScopes) {
			return cliconnector.EncryptedExecutionCredentials{}, domain.ErrInvalid
		}
		return result, nil
	}
	if identity != cliconnector.IdentityUser {
		return cliconnector.EncryptedExecutionCredentials{}, domain.ErrInvalid
	}
	var authorizations []cliConnectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND enablement_id = ? AND identity = 'user' AND state = 'active' AND expires_at > ?", ownerID, enablement.ID, time.Now().UTC()).Order("updated_at DESC").Limit(2).Find(&authorizations).Error; err != nil {
		return cliconnector.EncryptedExecutionCredentials{}, err
	}
	if len(authorizations) != 1 {
		return cliconnector.EncryptedExecutionCredentials{}, domain.ErrInvalid
	}
	var granted []string
	if err := json.Unmarshal(authorizations[0].Scopes, &granted); err != nil || !containsAllScopes(granted, requiredScopes) || len(authorizations[0].TokenCiphertext) == 0 {
		return cliconnector.EncryptedExecutionCredentials{}, domain.ErrInvalid
	}
	result.TokenCiphertext = append([]byte(nil), authorizations[0].TokenCiphertext...)
	result.ExternalIdentityID = authorizations[0].ExternalIdentityID
	return result, nil
}

func (repository *Repository) HasCLIConnectorRuntimeConformance(ctx context.Context, definitionID, bundleSHA256, runtimeDigest string) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Table("cli_connector_conformance").Where(
		"definition_id = ? AND bundle_sha256 = ? AND runtime_repo_digest = ? AND passed = ?",
		definitionID, bundleSHA256, runtimeDigest, true,
	).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check CLI Connector Runtime conformance: %w", err)
	}
	return count == 1, nil
}

func containsAllScopes(granted, required []string) bool {
	set := make(map[string]struct{}, len(granted))
	for _, scope := range granted {
		set[scope] = struct{}{}
	}
	for _, scope := range required {
		if _, ok := set[scope]; !ok {
			return false
		}
	}
	return true
}

func cliDefinitionDomain(row cliConnectorDefinitionRecord) (cliconnector.Definition, error) {
	var capabilities []cliconnector.Capability
	if err := json.Unmarshal(row.Capabilities, &capabilities); err != nil {
		return cliconnector.Definition{}, err
	}
	var architectures []string
	var recommendedSkills []cliconnector.RecommendedSkill
	if err := json.Unmarshal(row.SupportedArchitectures, &architectures); err != nil {
		return cliconnector.Definition{}, err
	}
	if err := json.Unmarshal(row.RecommendedSkills, &recommendedSkills); err != nil {
		return cliconnector.Definition{}, err
	}
	item := cliconnector.Definition{ID: row.ID, Name: row.Name, Icon: row.Icon, Description: row.Description, InstallationType: row.InstallationType, Package: row.NPMPackage, Version: row.NPMVersion, Integrity: row.NPMIntegrity, Executable: row.Executable, AuthenticationDriver: row.AuthenticationDriver, State: cliconnector.State(row.State), Capabilities: capabilities, SupportedArchitectures: architectures, RecommendedSkills: recommendedSkills, VersionNumber: row.Version, CreatedByUserID: row.CreatedByUserID}
	if row.SourceObjectKey != nil {
		item.SourceObjectKey = *row.SourceObjectKey
	}
	if row.SourceSHA256 != nil {
		item.SourceSHA256 = *row.SourceSHA256
	}
	if row.BundleObjectKey != nil {
		item.BundleObjectKey = *row.BundleObjectKey
	}
	if row.BundleSHA256 != nil {
		item.BundleSHA256 = *row.BundleSHA256
	}
	if row.FailureReason != nil {
		item.FailureReason = *row.FailureReason
	}
	return item, nil
}
func cliEnablementDomain(row cliConnectorEnablementRecord) cliconnector.Enablement {
	item := cliconnector.Enablement{ID: row.ID, OwnerID: row.OwnerID, DefinitionID: row.DefinitionID, State: row.State, ActionExpiresAt: row.ActionExpiresAt, Version: row.Version}
	if row.ActionURL != nil {
		item.ActionURL = *row.ActionURL
	}
	return item
}

func cliAuthorizationAttemptDomain(row cliConnectorAuthorizationAttemptRecord) (cliconnector.AuthorizationAttempt, error) {
	var scopes []string
	if err := json.Unmarshal(row.Scopes, &scopes); err != nil {
		return cliconnector.AuthorizationAttempt{}, err
	}
	return cliconnector.AuthorizationAttempt{
		ID: row.ID, OwnerID: row.OwnerID, EnablementID: row.EnablementID, Identity: cliconnector.Identity(row.Identity), Scopes: scopes,
		ActionURL: row.ActionURL, ExpiresAt: row.ExpiresAt, DeviceCodeCiphertext: append([]byte(nil), row.DeviceCodeCiphertext...),
	}, nil
}

func cliAuthorizationDomain(row cliConnectorAuthorizationRecord) (cliconnector.Authorization, error) {
	var scopes []string
	if err := json.Unmarshal(row.Scopes, &scopes); err != nil {
		return cliconnector.Authorization{}, err
	}
	return cliconnector.Authorization{
		ID: row.ID, OwnerID: row.OwnerID, EnablementID: row.EnablementID, Identity: cliconnector.Identity(row.Identity),
		ExternalIdentityID: row.ExternalIdentityID, ExternalDisplayName: row.ExternalDisplayName, Scopes: scopes,
		State: row.State, ExpiresAt: row.ExpiresAt, Version: row.Version,
	}, nil
}

func (repository *Repository) ListCommandApprovals(ctx context.Context, ownerID string, now time.Time) ([]domain.CommandApproval, error) {
	if err := repository.db.WithContext(ctx).Model(&cliCommandApprovalRecord{}).Where("owner_user_id = ? AND state IN ? AND expires_at <= ?", ownerID, []string{"pending", "approved"}, now).Updates(map[string]any{"state": "expired", "version": gorm.Expr("version + 1")}).Error; err != nil {
		return nil, err
	}
	var rows []cliCommandApprovalRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND state IN ?", ownerID, []string{"pending", "approved"}).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.CommandApproval, 0, len(rows))
	for _, row := range rows {
		items = append(items, commandApprovalDomain(row))
	}
	return items, nil
}

func (repository *Repository) DecideCommandApproval(ctx context.Context, ownerID, approvalID string, decision domain.ApprovalState, identity domain.ExecutionIdentity, expectedVersion int64, now time.Time) (domain.CommandApproval, error) {
	var row cliCommandApprovalRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ?", ownerID, approvalID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		approval := commandApprovalDomain(row)
		if row.Version != expectedVersion {
			return domain.ErrConflict
		}
		if err := approval.Decide(ownerID, decision, identity, now); err != nil {
			return err
		}
		updates := map[string]any{"state": string(approval.State), "decided_at": approval.DecidedAt, "version": gorm.Expr("version + 1")}
		if approval.Identity != "" {
			updates["identity"] = string(approval.Identity)
		}
		result := tx.Model(&cliCommandApprovalRecord{}).Where("id = ? AND version = ?", row.ID, expectedVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return domain.CommandApproval{}, err
	}
	return commandApprovalDomain(row), nil
}

func commandApprovalDomain(row cliCommandApprovalRecord) domain.CommandApproval {
	value := domain.CommandApproval{ID: row.ID, OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID, CommandDigest: row.CommandDigest, NonceHash: row.NonceHash, ConnectorName: row.ConnectorName, Operation: row.Operation, Target: row.Target, RedactedArguments: row.RedactedArguments, State: domain.ApprovalState(row.State), ExpiresAt: row.ExpiresAt, DecidedAt: row.DecidedAt, ConsumedAt: row.ConsumedAt, Version: row.Version}
	if row.Identity != nil {
		value.Identity = domain.ExecutionIdentity(*row.Identity)
	}
	return value
}

func (repository *Repository) Await(ctx context.Context, request cliconnector.ApprovalRequest) (cliconnector.ApprovalGrant, error) {
	now := time.Now().UTC()
	timeout := request.ExpiresAt.Sub(now)
	approval, err := domain.NewCommandApproval(uuid.NewString(), request.OwnerID, request.StageID, request.CommandDigest, request.Nonce, now, timeout)
	if err != nil || (request.ExecutionKind != "session" && request.ExecutionKind != "run") || request.ExecutionID == "" || request.ConnectorName == "" || request.Operation == "" || request.Target == "" || !slices.Contains(request.AllowedIdentities, request.Identity) || request.CommandDigests[request.Identity] != request.CommandDigest {
		return cliconnector.ApprovalGrant{}, fmt.Errorf("%w: invalid CLI command approval request", domain.ErrInvalid)
	}
	for _, identity := range request.AllowedIdentities {
		if (identity != cliconnector.IdentityUser && identity != cliconnector.IdentityBot) || len(request.CommandDigests[identity]) != 64 {
			return cliconnector.ApprovalGrant{}, fmt.Errorf("%w: invalid CLI command approval identities", domain.ErrInvalid)
		}
	}
	approval.ExecutionKind, approval.ExecutionID = request.ExecutionKind, request.ExecutionID
	approval.ConnectorName, approval.Operation, approval.Target = request.ConnectorName, request.Operation, request.Target
	approval.RedactedArguments = request.RedactedArguments
	row := cliCommandApprovalRecord{
		ID: approval.ID, OwnerID: approval.OwnerID, ExecutionKind: approval.ExecutionKind, ExecutionID: approval.ExecutionID,
		StageID: approval.StageID, ConnectorName: approval.ConnectorName, Operation: approval.Operation,
		Target: approval.Target, RedactedArguments: approval.RedactedArguments, CommandDigest: approval.CommandDigest,
		NonceHash: approval.NonceHash, State: string(approval.State), ExpiresAt: approval.ExpiresAt, Version: 1,
	}
	if len(request.AllowedIdentities) == 1 {
		approval.Identity = domain.ExecutionIdentity(request.AllowedIdentities[0])
		identity := string(approval.Identity)
		row.Identity = &identity
	}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := transitionApprovalExecution(tx, request, false, approval.ID, "user_action.required"); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		return cliconnector.ApprovalGrant{}, fmt.Errorf("persist CLI command approval: %w", err)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(time.Until(request.ExpiresAt))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
			return cliconnector.ApprovalGrant{}, ctx.Err()
		case <-timer.C:
			if err := repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalExpired); err != nil {
				return cliconnector.ApprovalGrant{}, err
			}
			return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalExpired
		case <-ticker.C:
			var current cliCommandApprovalRecord
			if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", row.ID, request.OwnerID).Take(&current).Error; err != nil {
				_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
				return cliconnector.ApprovalGrant{}, err
			}
			switch domain.ApprovalState(current.State) {
			case domain.ApprovalApproved:
				if current.Identity == nil || !slices.Contains(request.AllowedIdentities, cliconnector.Identity(*current.Identity)) {
					_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
					return cliconnector.ApprovalGrant{}, domain.ErrConflict
				}
				identity := cliconnector.Identity(*current.Identity)
				if err := repository.bindApprovedCommand(ctx, row.ID, request.OwnerID, request.CommandDigests[identity], identity); err != nil {
					_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{Nonce: request.Nonce, Identity: identity, ExpiresAt: request.ExpiresAt}, nil
			case domain.ApprovalRejected:
				if err := repository.closeCommandApproval(ctx, row.ID, request, domain.ApprovalRejected); err != nil {
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalRejected
			case domain.ApprovalExpired, domain.ApprovalClosed:
				if err := repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalState(current.State)); err != nil {
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalExpired
			}
		}
	}
}

func (repository *Repository) bindApprovedCommand(ctx context.Context, approvalID, ownerID, digest string, identity cliconnector.Identity) error {
	if len(digest) != 64 {
		return domain.ErrInvalid
	}
	result := repository.db.WithContext(ctx).Model(&cliCommandApprovalRecord{}).
		Where("id = ? AND owner_user_id = ? AND state = 'approved' AND identity = ?", approvalID, ownerID, identity).
		Update("command_digest", digest)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) Consume(ctx context.Context, ownerID, digest, nonce string) error {
	now := time.Now().UTC()
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row cliCommandApprovalRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND command_digest = ? AND nonce_hash = ?", ownerID, digest, domain.ApprovalNonceHash(nonce)).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		approval := commandApprovalDomain(row)
		if err := approval.Consume(ownerID, digest, nonce, now); err != nil {
			return err
		}
		result := tx.Model(&cliCommandApprovalRecord{}).Where("id = ? AND version = ? AND state = 'approved'", row.ID, row.Version).Updates(map[string]any{
			"state": string(domain.ApprovalConsumed), "consumed_at": approval.ConsumedAt, "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return domain.ErrConflict
		}
		request := cliconnector.ApprovalRequest{OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID}
		return transitionApprovalExecution(tx, request, true, row.ID, "user_action.resolved")
	})
}

func (repository *Repository) Close(ctx context.Context, ownerID, nonce string) error {
	var row cliCommandApprovalRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND nonce_hash = ?", ownerID, domain.ApprovalNonceHash(nonce)).Take(&row).Error; err != nil {
		return mapNotFound(err)
	}
	request := cliconnector.ApprovalRequest{OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID}
	return repository.closeCommandApproval(ctx, row.ID, request, domain.ApprovalClosed)
}

func (repository *Repository) closeCommandApproval(ctx context.Context, approvalID string, request cliconnector.ApprovalRequest, state domain.ApprovalState) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row cliCommandApprovalRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", approvalID, request.OwnerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == string(domain.ApprovalConsumed) {
			return nil
		}
		shouldClose := state == domain.ApprovalExpired && (row.State == string(domain.ApprovalPending) || row.State == string(domain.ApprovalApproved))
		shouldClose = shouldClose || state == domain.ApprovalClosed && (row.State == string(domain.ApprovalPending) || row.State == string(domain.ApprovalApproved))
		if shouldClose {
			if err := tx.Model(&cliCommandApprovalRecord{}).Where("id = ?", row.ID).Updates(map[string]any{"state": string(state), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		return transitionApprovalExecution(tx, request, true, row.ID, "user_action.resolved")
	})
}

func transitionApprovalExecution(tx *gorm.DB, request cliconnector.ApprovalRequest, resume bool, approvalID, eventType string) error {
	from, to := "running", "waiting_for_user"
	if resume {
		from, to = to, from
	}
	if request.ExecutionKind == "session" {
		messageID, err := strconv.ParseInt(request.ExecutionID, 10, 64)
		if err != nil || messageID <= 0 {
			return domain.ErrInvalid
		}
		if !resume {
			from, to = "generating", "waiting_for_user"
		} else {
			from, to = "waiting_for_user", "generating"
		}
		result := tx.Model(&messageRecord{}).Where("id = ? AND state = ? AND session_id IN (SELECT id FROM sessions WHERE owner_user_id = ?)", messageID, from, request.OwnerID).Updates(map[string]any{"state": to, "progress_stage": "using_tool"})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			if resume && result.RowsAffected == 0 {
				return nil
			}
			return domain.ErrConflict
		}
		return nil
	}
	result := tx.Model(&runRecord{}).Where("id = ? AND owner_user_id = ? AND state = ?", request.ExecutionID, request.OwnerID, from).Updates(map[string]any{"state": to, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		if resume && result.RowsAffected == 0 {
			return nil
		}
		return domain.ErrConflict
	}
	payload, err := json.Marshal(map[string]string{"approval_id": approvalID})
	if err != nil {
		return err
	}
	var sequence int64
	if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", request.ExecutionID).Scan(&sequence).Error; err != nil {
		return err
	}
	return tx.Table("run_events").Create(map[string]any{"run_id": request.ExecutionID, "sequence": sequence + 1, "event_type": eventType, "payload": payload, "occurred_at": time.Now().UTC()}).Error
}
