package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type connectorProviderApplicationRecord struct {
	OwnerID                             string    `gorm:"column:owner_user_id"`
	InstallationID                      string    `gorm:"column:installation_id"`
	ProviderApplicationIDCiphertext     []byte    `gorm:"column:provider_application_id_ciphertext"`
	ProviderApplicationSecretCiphertext []byte    `gorm:"column:provider_application_secret_ciphertext"`
	ProviderName                        string    `gorm:"column:provider_name"`
	DeveloperConsoleURL                 string    `gorm:"column:developer_console_url"`
	CreatedAt                           time.Time `gorm:"column:created_at"`
	UpdatedAt                           time.Time `gorm:"column:updated_at"`
	Version                             int64     `gorm:"column:version"`
}

func (connectorProviderApplicationRecord) TableName() string {
	return "connector_provider_applications"
}

type connectorSetupFlowRecord struct {
	ID                   string    `gorm:"column:id"`
	OwnerID              string    `gorm:"column:owner_user_id"`
	InstallationID       string    `gorm:"column:installation_id"`
	DeviceCodeCiphertext []byte    `gorm:"column:device_code_ciphertext"`
	ActionURL            string    `gorm:"column:action_url"`
	ExpiresAt            time.Time `gorm:"column:expires_at"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (connectorSetupFlowRecord) TableName() string { return "connector_setup_flows" }

type connectorAuthorizationFlowRecord struct {
	ID                   string    `gorm:"column:id"`
	OwnerID              string    `gorm:"column:owner_user_id"`
	InstallationID       string    `gorm:"column:installation_id"`
	Identity             string    `gorm:"column:identity"`
	Scopes               []byte    `gorm:"column:scopes;type:jsonb"`
	DeviceCodeCiphertext []byte    `gorm:"column:device_code_ciphertext"`
	ActionURL            string    `gorm:"column:action_url"`
	ExpiresAt            time.Time `gorm:"column:expires_at"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (connectorAuthorizationFlowRecord) TableName() string { return "connector_authorization_flows" }

func (repository *Repository) BeginConnectorSetup(ctx context.Context, input domain.ConnectorSetup) (domain.ConnectorSetup, error) {
	if input.OwnerID == "" || input.InstallationID == "" || input.ActionURL == "" || input.ExpiresAt == nil || !input.ExpiresAt.After(time.Now().UTC()) || len(input.DeviceCodeCiphertext) == 0 {
		return domain.ConnectorSetup{}, domain.ErrInvalid
	}
	row := connectorSetupFlowRecord{ID: uuid.NewString(), OwnerID: input.OwnerID, InstallationID: input.InstallationID, DeviceCodeCiphertext: input.DeviceCodeCiphertext, ActionURL: input.ActionURL, ExpiresAt: *input.ExpiresAt}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation connectorInstallationRecord
		if err := tx.Where("id = ? AND owner_user_id = ? AND state = ?", input.InstallationID, input.OwnerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
			return mapNotFound(err)
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "installation_id"}}, DoUpdates: clause.Assignments(map[string]any{"id": row.ID, "device_code_ciphertext": row.DeviceCodeCiphertext, "action_url": row.ActionURL, "expires_at": row.ExpiresAt, "updated_at": gorm.Expr("now()")})}).Create(&row).Error
	})
	if err != nil {
		return domain.ConnectorSetup{}, err
	}
	return repository.GetConnectorSetup(ctx, input.OwnerID, row.ID)
}

func (repository *Repository) GetConnectorSetup(ctx context.Context, ownerID, flowID string) (domain.ConnectorSetup, error) {
	var row connectorSetupFlowRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", flowID, ownerID).Take(&row).Error; err != nil {
		return domain.ConnectorSetup{}, mapNotFound(err)
	}
	if !time.Now().UTC().Before(row.ExpiresAt) {
		return domain.ConnectorSetup{}, domain.ErrConflict
	}
	return domain.ConnectorSetup{ID: row.ID, OwnerID: row.OwnerID, InstallationID: row.InstallationID, State: "waiting_for_user", ActionURL: row.ActionURL, ExpiresAt: &row.ExpiresAt, DeviceCodeCiphertext: append([]byte(nil), row.DeviceCodeCiphertext...)}, nil
}

func (repository *Repository) GetConnectorProviderApplication(ctx context.Context, ownerID, installationID string) (domain.ConnectorProviderApplication, error) {
	var row connectorProviderApplicationRecord
	err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Take(&row).Error
	if err == nil {
		return connectorProviderApplicationDomain(row), nil
	}
	if err != gorm.ErrRecordNotFound {
		return domain.ConnectorProviderApplication{}, err
	}
	var legacy feishuCLIApplicationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Take(&legacy).Error; err != nil {
		return domain.ConnectorProviderApplication{}, mapNotFound(err)
	}
	return domain.ConnectorProviderApplication{OwnerID: ownerID, InstallationID: installationID, AppIDCiphertext: append([]byte(nil), legacy.ProviderApplicationIDCiphertext...), AppSecretCiphertext: append([]byte(nil), legacy.ProviderApplicationSecretCiphertext...), ProviderName: legacy.ProviderName, DeveloperConsoleURL: legacy.DeveloperConsoleURL}, nil
}

func (repository *Repository) CompleteConnectorSetup(ctx context.Context, ownerID, flowID string, application domain.ConnectorProviderApplication) (domain.ConnectorSetup, error) {
	var flow connectorSetupFlowRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND expires_at > now()", flowID, ownerID).Take(&flow).Error; err != nil {
			return mapNotFound(err)
		}
		row := connectorProviderApplicationRecord{OwnerID: ownerID, InstallationID: flow.InstallationID, ProviderApplicationIDCiphertext: application.AppIDCiphertext, ProviderApplicationSecretCiphertext: application.AppSecretCiphertext, ProviderName: application.ProviderName, DeveloperConsoleURL: application.DeveloperConsoleURL, Version: 1}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}}, DoUpdates: clause.Assignments(map[string]any{"installation_id": row.InstallationID, "provider_application_id_ciphertext": row.ProviderApplicationIDCiphertext, "provider_application_secret_ciphertext": row.ProviderApplicationSecretCiphertext, "provider_name": row.ProviderName, "developer_console_url": row.DeveloperConsoleURL, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("connector_provider_applications.version + 1")})}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Delete(&connectorSetupFlowRecord{}, "id = ?", flow.ID).Error
	})
	if err != nil {
		return domain.ConnectorSetup{}, err
	}
	return domain.ConnectorSetup{ID: flow.ID, OwnerID: ownerID, InstallationID: flow.InstallationID, State: "completed", ProviderName: application.ProviderName, DeveloperConsoleURL: application.DeveloperConsoleURL}, nil
}

func (repository *Repository) BeginConnectorAuthorizationFlow(ctx context.Context, input domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	if input.OwnerID == "" || input.InstallationID == "" || input.Identity != "user" || input.ActionURL == "" || !input.ExpiresAt.After(time.Now().UTC()) || len(input.DeviceCodeCiphertext) == 0 {
		return domain.ConnectorAuthorizationAttempt{}, domain.ErrInvalid
	}
	scopes, _ := json.Marshal(input.Scopes)
	row := connectorAuthorizationFlowRecord{ID: uuid.NewString(), OwnerID: input.OwnerID, InstallationID: input.InstallationID, Identity: input.Identity, Scopes: scopes, DeviceCodeCiphertext: input.DeviceCodeCiphertext, ActionURL: input.ActionURL, ExpiresAt: input.ExpiresAt}
	if err := repository.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "installation_id"}, {Name: "identity"}}, DoUpdates: clause.Assignments(map[string]any{"id": row.ID, "scopes": scopes, "device_code_ciphertext": row.DeviceCodeCiphertext, "action_url": row.ActionURL, "expires_at": row.ExpiresAt, "updated_at": gorm.Expr("now()")})}).Create(&row).Error; err != nil {
		return domain.ConnectorAuthorizationAttempt{}, err
	}
	return repository.GetConnectorAuthorizationFlow(ctx, input.OwnerID, row.ID)
}

func (repository *Repository) GetConnectorAuthorizationFlow(ctx context.Context, ownerID, flowID string) (domain.ConnectorAuthorizationAttempt, error) {
	var row connectorAuthorizationFlowRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND expires_at > now()", flowID, ownerID).Take(&row).Error; err != nil {
		return domain.ConnectorAuthorizationAttempt{}, mapNotFound(err)
	}
	var scopes []string
	if err := json.Unmarshal(row.Scopes, &scopes); err != nil {
		return domain.ConnectorAuthorizationAttempt{}, fmt.Errorf("decode Connector authorization scopes: %w", err)
	}
	return domain.ConnectorAuthorizationAttempt{ID: row.ID, OwnerID: row.OwnerID, InstallationID: row.InstallationID, Identity: row.Identity, Scopes: scopes, ActionURL: row.ActionURL, ExpiresAt: row.ExpiresAt, DeviceCodeCiphertext: append([]byte(nil), row.DeviceCodeCiphertext...)}, nil
}

func (repository *Repository) DeleteConnectorAuthorizationFlow(ctx context.Context, ownerID, flowID string) error {
	return repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", flowID, ownerID).Delete(&connectorAuthorizationFlowRecord{}).Error
}

func connectorProviderApplicationDomain(row connectorProviderApplicationRecord) domain.ConnectorProviderApplication {
	return domain.ConnectorProviderApplication{OwnerID: row.OwnerID, InstallationID: row.InstallationID, AppIDCiphertext: append([]byte(nil), row.ProviderApplicationIDCiphertext...), AppSecretCiphertext: append([]byte(nil), row.ProviderApplicationSecretCiphertext...), ProviderName: row.ProviderName, DeveloperConsoleURL: row.DeveloperConsoleURL}
}

// UpdateConnectorAuthorizationFlow binds the callback to the current, unexpired
// owner-scoped challenge; replayed or superseded callbacks fail the compare-and-swap.
func (repository *Repository) UpdateConnectorAuthorizationFlow(ctx context.Context, ownerID, flowID string, previous, next []byte, actionURL string) error {
	if ownerID == "" || flowID == "" || len(previous) == 0 || len(next) == 0 || actionURL == "" {
		return domain.ErrInvalid
	}
	result := repository.db.WithContext(ctx).Model(&connectorAuthorizationFlowRecord{}).Where("id = ? AND owner_user_id = ? AND expires_at > now() AND device_code_ciphertext = ?", flowID, ownerID, previous).Updates(map[string]any{"device_code_ciphertext": next, "action_url": actionURL, "updated_at": gorm.Expr("now()")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

// ConsumeConnectorAuthorizationFlow atomically claims a single-use OAuth code.
func (repository *Repository) ConsumeConnectorAuthorizationFlow(ctx context.Context, ownerID, flowID string, expected []byte) error {
	if ownerID == "" || flowID == "" || len(expected) == 0 {
		return domain.ErrInvalid
	}
	result := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND device_code_ciphertext = ? AND expires_at > now()", flowID, ownerID, expected).Delete(&connectorAuthorizationFlowRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}
