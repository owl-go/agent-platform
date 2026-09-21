package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConnectorPackageRepository interface {
	CreateConnectorRevision(context.Context, domain.ConnectorRevision) (domain.ConnectorRevision, error)
	InstallConnector(context.Context, domain.ConnectorInstallation) (domain.ConnectorInstallation, error)
	ListConnectorInstallations(context.Context, string) ([]domain.ConnectorInstallation, error)
	ActivateConnectorRevision(context.Context, string, string, string, time.Time) (domain.ConnectorInstallation, error)
	RollbackConnectorRevision(context.Context, string, string, string, string, time.Time) (domain.ConnectorInstallation, error)
	SetConnectorInstallationState(context.Context, string, string, domain.ConnectorInstallationState, int64) (domain.ConnectorInstallation, error)
	DisconnectConnectorAuthorization(context.Context, string, string) (domain.ConnectorAuthorization, error)
	CreateConnectorAuthorization(context.Context, domain.ConnectorAuthorization) (domain.ConnectorAuthorization, error)
	RecordConnectorAudit(context.Context, domain.ConnectorAuditRecord) error
}

var _ ConnectorPackageRepository = (*Repository)(nil)

func (repository *Repository) CreateConnectorRevision(ctx context.Context, input domain.ConnectorRevision) (domain.ConnectorRevision, error) {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.PackageSource == "" || input.Version == "" || input.PackageSHA256 == "" || input.ObjectKey == "" {
		return domain.ConnectorRevision{}, fmt.Errorf("%w: connector revision is incomplete", domain.ErrInvalid)
	}
	row := connectorRevisionRecord{ID: input.ID, PackageSource: input.PackageSource, Version: input.Version, Mode: string(input.Mode), PackageSHA256: input.PackageSHA256, RuntimePolicy: input.RuntimePolicy, ObjectKey: input.ObjectKey, CreatedAt: input.CreatedAt}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	var existing connectorRevisionRecord
	err := repository.db.WithContext(ctx).Where("package_source = ? AND version = ?", input.PackageSource, input.Version).Order("created_at DESC").Take(&existing).Error
	if err == nil {
		if existing.PackageSHA256 != input.PackageSHA256 {
			return domain.ConnectorRevision{}, fmt.Errorf("%w: Connector version already has a different checksum", domain.ErrConflict)
		}
		return connectorRevisionDomain(existing), nil
	}
	if err != gorm.ErrRecordNotFound {
		return domain.ConnectorRevision{}, fmt.Errorf("read Connector Revision: %w", err)
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return domain.ConnectorRevision{}, fmt.Errorf("create Connector Revision: %w", err)
	}
	return connectorRevisionDomain(row), nil
}

func (repository *Repository) InstallConnector(ctx context.Context, input domain.ConnectorInstallation) (domain.ConnectorInstallation, error) {
	return repository.installConnector(ctx, input, nil)
}

func (repository *Repository) InstallConnectorWithAudit(ctx context.Context, input domain.ConnectorInstallation, audit domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	return repository.installConnector(ctx, input, &audit)
}

func (repository *Repository) installConnector(ctx context.Context, input domain.ConnectorInstallation, audit *domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.OwnerID == "" || input.PackageSource == "" || input.ActiveRevisionID == "" {
		return domain.ConnectorInstallation{}, fmt.Errorf("%w: connector installation is incomplete", domain.ErrInvalid)
	}
	if input.State == "" {
		input.State = domain.ConnectorInstallationPending
	}
	if input.Version == 0 {
		input.Version = 1
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = time.Now().UTC()
	}
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var revision connectorRevisionRecord
		if err := tx.Where("id = ? AND package_source = ?", input.ActiveRevisionID, input.PackageSource).Take(&revision).Error; err != nil {
			return mapNotFound(err)
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND package_source = ?", input.OwnerID, input.PackageSource).Take(&row).Error
		if err == nil {
			if row.State == string(domain.ConnectorInstallationUninstalled) {
				return domain.ErrConflict
			}
			updates := map[string]any{"active_revision_id": input.ActiveRevisionID, "state": string(domain.ConnectorInstallationActive), "updated_at": input.UpdatedAt, "version": gorm.Expr("version + 1")}
			var oldRevision connectorRevisionRecord
			if err := tx.Where("id = ?", row.ActiveRevisionID).Take(&oldRevision).Error; err != nil {
				return err
			}
			if oldRevision.Mode != revision.Mode {
				updates["authorization_id"] = nil
				if row.AuthorizationID != nil {
					if err := tx.Model(&connectorAuthorizationRecord{}).Where("id = ?", *row.AuthorizationID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
						return err
					}
				}
			}
			result := tx.Model(&connectorInstallationRecord{}).Where("id = ? AND version = ?", row.ID, row.Version).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrConflict
			}
			if err := tx.Where("id = ?", row.ID).Take(&row).Error; err != nil {
				return err
			}
			if audit != nil {
				if audit.InstallationID == "" {
					audit.InstallationID = row.ID
				}
				if audit.RevisionID == "" {
					audit.RevisionID = row.ActiveRevisionID
				}
				if audit.Mode == "" {
					audit.Mode = domain.ConnectorMode(revision.Mode)
				}
				return createConnectorAuditTx(tx, *audit)
			}
			return nil
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		row = connectorInstallationRecord{ID: input.ID, OwnerID: input.OwnerID, PackageSource: input.PackageSource, ActiveRevisionID: input.ActiveRevisionID, State: string(input.State), Version: input.Version, UpdatedAt: input.UpdatedAt}
		if input.AuthorizationID != "" {
			row.AuthorizationID = &input.AuthorizationID
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if audit != nil {
			if audit.InstallationID == "" {
				audit.InstallationID = row.ID
			}
			if audit.RevisionID == "" {
				audit.RevisionID = row.ActiveRevisionID
			}
			if audit.Mode == "" {
				audit.Mode = domain.ConnectorMode(revision.Mode)
			}
			return createConnectorAuditTx(tx, *audit)
		}
		return nil
	})
	if err != nil {
		return domain.ConnectorInstallation{}, fmt.Errorf("install Connector: %w", err)
	}
	return connectorInstallationDomain(row), nil
}

func (repository *Repository) ListConnectorInstallations(ctx context.Context, ownerID string) ([]domain.ConnectorInstallation, error) {
	var rows []connectorInstallationRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND state <> ?", ownerID, domain.ConnectorInstallationUninstalled).Order("package_source, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Connector Installations: %w", err)
	}
	items := make([]domain.ConnectorInstallation, 0, len(rows))
	for _, row := range rows {
		items = append(items, connectorInstallationDomain(row))
	}
	return items, nil
}

func (repository *Repository) SetConnectorInstallationState(ctx context.Context, ownerID, installationID string, state domain.ConnectorInstallationState, expectedVersion int64) (domain.ConnectorInstallation, error) {
	return repository.setConnectorInstallationState(ctx, ownerID, installationID, state, expectedVersion, nil)
}

func (repository *Repository) SetConnectorInstallationStateWithAudit(ctx context.Context, ownerID, installationID string, state domain.ConnectorInstallationState, expectedVersion int64, audit domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	return repository.setConnectorInstallationState(ctx, ownerID, installationID, state, expectedVersion, &audit)
}

func (repository *Repository) setConnectorInstallationState(ctx context.Context, ownerID, installationID string, state domain.ConnectorInstallationState, expectedVersion int64, audit *domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	if state != domain.ConnectorInstallationDisabled && state != domain.ConnectorInstallationUninstalled {
		return domain.ConnectorInstallation{}, fmt.Errorf("%w: unsupported Connector Installation state", domain.ErrInvalid)
	}
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND version = ? AND state <> ?", installationID, ownerID, expectedVersion, domain.ConnectorInstallationUninstalled).Take(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrConflict
			}
			return err
		}
		if err := tx.Model(&connectorInstallationRecord{}).Where("id = ? AND version = ?", installationID, row.Version).Updates(map[string]any{"state": string(state), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		if state == domain.ConnectorInstallationUninstalled {
			if err := tx.Model(&connectorAuthorizationRecord{}).Where("installation_id = ? AND owner_user_id = ?", installationID, ownerID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("id = ?", installationID).Take(&row).Error; err != nil {
			return err
		}
		if audit != nil {
			if audit.InstallationID == "" {
				audit.InstallationID = row.ID
			}
			if audit.RevisionID == "" {
				audit.RevisionID = row.ActiveRevisionID
			}
			return createConnectorAuditTx(tx, *audit)
		}
		return nil
	})
	if err != nil {
		return domain.ConnectorInstallation{}, err
	}
	return connectorInstallationDomain(row), nil
}

func (repository *Repository) ActivateConnectorRevision(ctx context.Context, ownerID, installationID, revisionID string, now time.Time) (domain.ConnectorInstallation, error) {
	return repository.changeConnectorRevision(ctx, ownerID, installationID, revisionID, "", now)
}

func (repository *Repository) RollbackConnectorRevision(ctx context.Context, ownerID, installationID, revisionID, reason string, now time.Time) (domain.ConnectorInstallation, error) {
	return repository.changeConnectorRevision(ctx, ownerID, installationID, revisionID, reason, now)
}

func (repository *Repository) changeConnectorRevision(ctx context.Context, ownerID, installationID, revisionID, reason string, now time.Time) (domain.ConnectorInstallation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", installationID, ownerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		var revision connectorRevisionRecord
		if err := tx.Where("id = ? AND package_source = ?", revisionID, row.PackageSource).Take(&revision).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == string(domain.ConnectorInstallationUninstalled) {
			return domain.ErrConflict
		}
		updates := map[string]any{"active_revision_id": revisionID, "state": string(domain.ConnectorInstallationActive), "updated_at": now, "version": gorm.Expr("version + 1")}
		var oldRevision connectorRevisionRecord
		if err := tx.Where("id = ?", row.ActiveRevisionID).Take(&oldRevision).Error; err != nil {
			return err
		}
		if oldRevision.Mode != revision.Mode {
			updates["authorization_id"] = nil
			if row.AuthorizationID != nil {
				if err := tx.Model(&connectorAuthorizationRecord{}).
					Where("id = ? AND owner_user_id = ?", *row.AuthorizationID, ownerID).
					Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&connectorInstallationRecord{}).Where("id = ? AND version = ?", installationID, row.Version).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", installationID).Take(&row).Error
	})
	if err != nil {
		return domain.ConnectorInstallation{}, err
	}
	_ = reason // reason is intentionally retained by the caller's audit record, not persisted on the installation.
	return connectorInstallationDomain(row), nil
}

func (repository *Repository) DisconnectConnectorAuthorization(ctx context.Context, ownerID, authorizationID string) (domain.ConnectorAuthorization, error) {
	var row connectorAuthorizationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", authorizationID, ownerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State != string(domain.ConnectorAuthorizationDisconnected) {
			result := tx.Model(&connectorAuthorizationRecord{}).Where("id = ? AND version = ?", authorizationID, row.Version).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return domain.ErrConflict
			}
		}
		if err := tx.Model(&connectorInstallationRecord{}).
			Where("id = ? AND owner_user_id = ? AND authorization_id = ?", row.InstallationID, ownerID, authorizationID).
			Updates(map[string]any{"authorization_id": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", authorizationID).Take(&row).Error
	})
	if err != nil {
		return domain.ConnectorAuthorization{}, err
	}
	return connectorAuthorizationDomain(row), nil
}

func (repository *Repository) DisconnectConnectorInstallationAuthorization(ctx context.Context, ownerID, installationID string, expectedVersion int64) (domain.ConnectorInstallation, error) {
	return repository.disconnectConnectorInstallationAuthorization(ctx, ownerID, installationID, expectedVersion, nil)
}

func (repository *Repository) DisconnectConnectorInstallationAuthorizationWithAudit(ctx context.Context, ownerID, installationID string, expectedVersion int64, audit domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	return repository.disconnectConnectorInstallationAuthorization(ctx, ownerID, installationID, expectedVersion, &audit)
}

func (repository *Repository) disconnectConnectorInstallationAuthorization(ctx context.Context, ownerID, installationID string, expectedVersion int64, audit *domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND version = ? AND state <> ?", installationID, ownerID, expectedVersion, domain.ConnectorInstallationUninstalled).Take(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrConflict
			}
			return err
		}
		if row.AuthorizationID != nil {
			if err := tx.Model(&connectorAuthorizationRecord{}).Where("id = ? AND owner_user_id = ?", *row.AuthorizationID, ownerID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&connectorInstallationRecord{}).Where("id = ? AND version = ?", installationID, row.Version).Updates(map[string]any{"authorization_id": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", installationID).Take(&row).Error; err != nil {
			return err
		}
		if audit != nil {
			if audit.InstallationID == "" {
				audit.InstallationID = row.ID
			}
			if audit.RevisionID == "" {
				audit.RevisionID = row.ActiveRevisionID
			}
			return createConnectorAuditTx(tx, *audit)
		}
		return nil
	})
	if err != nil {
		return domain.ConnectorInstallation{}, err
	}
	return connectorInstallationDomain(row), nil
}

func (repository *Repository) CreateConnectorAuthorization(ctx context.Context, input domain.ConnectorAuthorization) (domain.ConnectorAuthorization, error) {
	return repository.createConnectorAuthorization(ctx, input, nil)
}

func (repository *Repository) CreateConnectorAuthorizationWithAudit(ctx context.Context, input domain.ConnectorAuthorization, audit domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error) {
	return repository.createConnectorAuthorization(ctx, input, &audit)
}

func (repository *Repository) createConnectorAuthorization(ctx context.Context, input domain.ConnectorAuthorization, audit *domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error) {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.OwnerID == "" || input.InstallationID == "" || input.IdentityRef == "" || len(input.CredentialCiphertext) == 0 {
		return domain.ConnectorAuthorization{}, fmt.Errorf("%w: encrypted Connector authorization is incomplete", domain.ErrInvalid)
	}
	if input.State == "" {
		input.State = domain.ConnectorAuthorizationActive
	}
	now := time.Now().UTC()
	if input.State != domain.ConnectorAuthorizationActive || input.ExpiresAt != nil && !now.Before(*input.ExpiresAt) {
		return domain.ConnectorAuthorization{}, fmt.Errorf("%w: Connector authorization must be active and unexpired", domain.ErrInvalid)
	}
	if input.Version == 0 {
		input.Version = 1
	}
	scopes, err := json.Marshal(input.Scopes)
	if err != nil {
		return domain.ConnectorAuthorization{}, fmt.Errorf("encode Connector Authorization scopes: %w", err)
	}
	row := connectorAuthorizationRecord{ID: input.ID, OwnerID: input.OwnerID, InstallationID: input.InstallationID, IdentityRef: input.IdentityRef, Scopes: scopes, CredentialCiphertext: input.CredentialCiphertext, State: string(input.State), ExpiresAt: input.ExpiresAt, Version: input.Version, UpdatedAt: now}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation connectorInstallationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ? AND state <> ?", input.InstallationID, input.OwnerID, domain.ConnectorInstallationUninstalled).
			Take(&installation).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if installation.AuthorizationID != nil && *installation.AuthorizationID != row.ID {
			if err := tx.Model(&connectorAuthorizationRecord{}).
				Where("id = ? AND owner_user_id = ?", *installation.AuthorizationID, input.OwnerID).
				Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "updated_at": row.UpdatedAt, "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&connectorInstallationRecord{}).
			Where("id = ? AND owner_user_id = ?", input.InstallationID, input.OwnerID).
			Updates(map[string]any{"authorization_id": row.ID, "updated_at": row.UpdatedAt, "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		if audit != nil {
			return createConnectorAuditTx(tx, *audit)
		}
		return nil
	}); err != nil {
		return domain.ConnectorAuthorization{}, fmt.Errorf("create Connector Authorization: %w", err)
	}
	return connectorAuthorizationDomain(row), nil
}

func createConnectorAuditTx(tx *gorm.DB, input domain.ConnectorAuditRecord) error {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.Operation == "" || input.Outcome == "" {
		return fmt.Errorf("%w: audit operation and outcome are required", domain.ErrInvalid)
	}
	return tx.Create(&connectorAuditRecord{ID: input.ID, OwnerID: input.OwnerID, InstallationID: input.InstallationID, RevisionID: input.RevisionID, Mode: string(input.Mode), Operation: input.Operation, Risk: input.Risk, IdentityRef: input.IdentityRef, ApprovalReference: input.ApprovalReference, Outcome: input.Outcome, ErrorType: input.ErrorType, RequestID: input.RequestID, PolicyRevision: input.PolicyRevision, CreatedAt: input.CreatedAt}).Error
}

func (repository *Repository) RecordConnectorAudit(ctx context.Context, input domain.ConnectorAuditRecord) error {
	if input.ID == "" {
		input.ID = uuid.NewString()
	}
	if input.Operation == "" || input.Outcome == "" {
		return fmt.Errorf("%w: audit operation and outcome are required", domain.ErrInvalid)
	}
	return repository.db.WithContext(ctx).Create(&connectorAuditRecord{ID: input.ID, OwnerID: input.OwnerID, InstallationID: input.InstallationID, RevisionID: input.RevisionID, Mode: string(input.Mode), Operation: input.Operation, Risk: input.Risk, IdentityRef: input.IdentityRef, ApprovalReference: input.ApprovalReference, Outcome: input.Outcome, ErrorType: input.ErrorType, RequestID: input.RequestID, PolicyRevision: input.PolicyRevision, CreatedAt: input.CreatedAt}).Error
}

// ValidateCLIConnectorInvocation is the fail-closed lifecycle check used immediately before a CLI process starts.
func (repository *Repository) ValidateCLIConnectorInvocation(ctx context.Context, ownerID, definitionID string) error {
	var definition cliConnectorDefinitionRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND state = ? AND deleted_at IS NULL", definitionID, cliconnector.StateAvailable).Take(&definition).Error; err != nil {
		return mapNotFound(err)
	}
	if definition.AuthenticationDriver == "none" {
		return nil
	}
	var enablement cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND definition_id = ? AND state = ?", ownerID, definitionID, "enabled").Take(&enablement).Error; err != nil {
		return fmt.Errorf("%w: Connector is not enabled", domain.ErrConflict)
	}
	var count int64
	if err := repository.db.WithContext(ctx).Model(&cliConnectorAuthorizationRecord{}).Where("enablement_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", enablement.ID, "active").Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	return nil
}

// ValidateMCPInvocation is the fail-closed lifecycle check used immediately before MCP configuration is materialized.
func (repository *Repository) ValidateMCPInvocation(ctx context.Context, ownerID, serverID string) error {
	db := repository.db.WithContext(ctx)
	var server mcpRecord
	err := mcpCatalogQuery(db).
		Where("owner_user_id IN (?) AND id = ? AND test_requested_at IS NULL AND tested_at IS NOT NULL AND test_error IS NULL", accessibleResourceOwnerIDs(db, ownerID), serverID).
		Take(&server).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	var installation connectorInstallationRecord
	if err := db.Where("id = ? AND owner_user_id = ? AND state = ?", serverID, ownerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
		return mapNotFound(err)
	}
	var revision connectorRevisionRecord
	if err := db.Where("id = ? AND mode = ?", installation.ActiveRevisionID, domain.ConnectorModeMCP).Take(&revision).Error; err != nil {
		return mapNotFound(err)
	}
	var policy struct {
		AuthMode string `json:"auth_mode"`
	}
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.AuthMode == "" {
		return fmt.Errorf("%w: Connector authorization policy is unavailable", domain.ErrConflict)
	}
	if policy.AuthMode == "none" {
		return nil
	}
	if installation.AuthorizationID == nil {
		return fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	var count int64
	if err := db.Model(&connectorAuthorizationRecord{}).
		Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", *installation.AuthorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).
		Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	return nil
}

func connectorRevisionDomain(row connectorRevisionRecord) domain.ConnectorRevision {
	return domain.ConnectorRevision{ID: row.ID, PackageSource: row.PackageSource, Version: row.Version, Mode: domain.ConnectorMode(row.Mode), PackageSHA256: row.PackageSHA256, RuntimePolicy: row.RuntimePolicy, ObjectKey: row.ObjectKey, CreatedAt: row.CreatedAt}
}

func connectorInstallationDomain(row connectorInstallationRecord) domain.ConnectorInstallation {
	item := domain.ConnectorInstallation{ID: row.ID, OwnerID: row.OwnerID, PackageSource: row.PackageSource, ActiveRevisionID: row.ActiveRevisionID, State: domain.ConnectorInstallationState(row.State), Version: row.Version, UpdatedAt: row.UpdatedAt}
	if row.AuthorizationID != nil {
		item.AuthorizationID = *row.AuthorizationID
	}
	return item
}

func connectorAuthorizationDomain(row connectorAuthorizationRecord) domain.ConnectorAuthorization {
	var scopes []string
	_ = json.Unmarshal(row.Scopes, &scopes)
	return domain.ConnectorAuthorization{ID: row.ID, OwnerID: row.OwnerID, InstallationID: row.InstallationID, IdentityRef: row.IdentityRef, Scopes: scopes, CredentialCiphertext: row.CredentialCiphertext, State: domain.ConnectorAuthorizationState(row.State), ExpiresAt: row.ExpiresAt, Version: row.Version, UpdatedAt: row.UpdatedAt}
}
