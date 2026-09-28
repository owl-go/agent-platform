package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConnectorPackageRepository interface {
	CreateConnectorRevision(context.Context, domain.ConnectorRevision) (domain.ConnectorRevision, error)
	GetConnectorRevision(context.Context, string) (domain.ConnectorRevision, error)
	ListConnectorRevisions(context.Context) ([]domain.ConnectorRevision, error)
	PublishConnectorRevision(context.Context, string, string, int64) (domain.ConnectorPublication, error)
	SetConnectorPublicationState(context.Context, string, string, domain.ConnectorPublicationState, int64) (domain.ConnectorPublication, error)
	ListConnectorPublications(context.Context, bool) ([]domain.ConnectorPublication, error)
	ListConnectorPublicationHealth(context.Context) ([]domain.ConnectorPublicationHealth, error)
	HasConnectorBundleRuntimeConformance(context.Context, string, string) (bool, error)
	InstallConnector(context.Context, domain.ConnectorInstallation) (domain.ConnectorInstallation, error)
	ListConnectorInstallations(context.Context, string) ([]domain.ConnectorInstallation, error)
	ListConnectorAuthorizations(context.Context, string, string) ([]domain.ConnectorAuthorization, error)
	SelectConnectorAuthorization(context.Context, string, string, string, int64) (domain.ConnectorInstallation, error)
	RefreshConnectorAuthorization(context.Context, domain.ConnectorAuthorization, int64, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	ActivateConnectorRevision(context.Context, string, string, string, time.Time) (domain.ConnectorInstallation, error)
	ActivateConnectorRevisionWithVersion(context.Context, string, string, string, int64, time.Time) (domain.ConnectorInstallation, error)
	RollbackConnectorRevision(context.Context, string, string, string, string, time.Time) (domain.ConnectorInstallation, error)
	SetConnectorInstallationState(context.Context, string, string, domain.ConnectorInstallationState, int64) (domain.ConnectorInstallation, error)
	DisconnectConnectorAuthorization(context.Context, string, string) (domain.ConnectorAuthorization, error)
	DisconnectConnectorAuthorizationWithAudit(context.Context, string, string, domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error)
	CreateConnectorAuthorization(context.Context, domain.ConnectorAuthorization) (domain.ConnectorAuthorization, error)
	RecordConnectorAudit(context.Context, domain.ConnectorAuditRecord) error
}

var _ ConnectorPackageRepository = (*Repository)(nil)

func (repository *Repository) HasConnectorBundleRuntimeConformance(ctx context.Context, bundleSHA256, runtimeDigest string) (bool, error) {
	if len(bundleSHA256) != 64 || !strings.HasPrefix(runtimeDigest, "sha256:") {
		return false, fmt.Errorf("%w: Connector conformance identity is invalid", domain.ErrInvalid)
	}
	var count int64
	err := repository.db.WithContext(ctx).Table("cli_connector_conformance").
		Where("bundle_sha256 = ? AND runtime_repo_digest = ? AND passed = true", bundleSHA256, runtimeDigest).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("read Connector Conformance evidence: %w", err)
	}
	return count > 0, nil
}

func (repository *Repository) GetConnectorRevision(ctx context.Context, revisionID string) (domain.ConnectorRevision, error) {
	if revisionID == "" {
		return domain.ConnectorRevision{}, fmt.Errorf("%w: Connector Revision ID is required", domain.ErrInvalid)
	}
	var row connectorRevisionRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", revisionID).Take(&row).Error; err != nil {
		return domain.ConnectorRevision{}, mapNotFound(err)
	}
	return connectorRevisionDomain(row), nil
}

func (repository *Repository) ListConnectorRevisions(ctx context.Context) ([]domain.ConnectorRevision, error) {
	var rows []connectorRevisionRecord
	if err := repository.db.WithContext(ctx).Order("package_source, created_at DESC, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Connector Revisions: %w", err)
	}
	items := make([]domain.ConnectorRevision, 0, len(rows))
	for _, row := range rows {
		items = append(items, connectorRevisionDomain(row))
	}
	return items, nil
}

func (repository *Repository) PublishConnectorRevision(ctx context.Context, administratorID, revisionID string, expectedVersion int64) (domain.ConnectorPublication, error) {
	if administratorID == "" || revisionID == "" || expectedVersion < 0 {
		return domain.ConnectorPublication{}, fmt.Errorf("%w: connector publication input is incomplete", domain.ErrInvalid)
	}
	var row connectorPublicationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var revision connectorRevisionRecord
		if err := tx.Where("id = ?", revisionID).Take(&revision).Error; err != nil {
			return mapNotFound(err)
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("package_source = ?", revision.PackageSource).Take(&row).Error
		if err == gorm.ErrRecordNotFound {
			if expectedVersion != 0 {
				return domain.ErrConflict
			}
			row = connectorPublicationRecord{PackageSource: revision.PackageSource, ActiveRevisionID: revision.ID, State: string(domain.ConnectorPublicationAvailable), AdministratorID: administratorID, Version: 1, UpdatedAt: time.Now().UTC()}
			return tx.Create(&row).Error
		}
		if err != nil {
			return err
		}
		if row.Version != expectedVersion {
			return domain.ErrConflict
		}
		result := tx.Model(&connectorPublicationRecord{}).
			Where("package_source = ? AND version = ?", row.PackageSource, row.Version).
			Updates(map[string]any{"active_revision_id": revision.ID, "state": string(domain.ConnectorPublicationAvailable), "administrator_user_id": administratorID, "version": gorm.Expr("version + 1"), "updated_at": gorm.Expr("now()")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.Where("package_source = ?", row.PackageSource).Take(&row).Error
	})
	if err != nil {
		return domain.ConnectorPublication{}, fmt.Errorf("publish Connector Revision: %w", err)
	}
	return connectorPublicationDomain(row), nil
}

func (repository *Repository) SetConnectorPublicationState(ctx context.Context, administratorID, packageSource string, state domain.ConnectorPublicationState, expectedVersion int64) (domain.ConnectorPublication, error) {
	if administratorID == "" || packageSource == "" || expectedVersion <= 0 || state != domain.ConnectorPublicationAvailable && state != domain.ConnectorPublicationDisabled {
		return domain.ConnectorPublication{}, fmt.Errorf("%w: connector publication state input is invalid", domain.ErrInvalid)
	}
	var row connectorPublicationRecord
	result := repository.db.WithContext(ctx).Model(&connectorPublicationRecord{}).
		Where("package_source = ? AND version = ?", packageSource, expectedVersion).
		Updates(map[string]any{"state": string(state), "administrator_user_id": administratorID, "version": gorm.Expr("version + 1"), "updated_at": gorm.Expr("now()")})
	if result.Error != nil {
		return domain.ConnectorPublication{}, fmt.Errorf("set Connector Publication state: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return domain.ConnectorPublication{}, domain.ErrConflict
	}
	if err := repository.db.WithContext(ctx).Where("package_source = ?", packageSource).Take(&row).Error; err != nil {
		return domain.ConnectorPublication{}, fmt.Errorf("read Connector Publication: %w", err)
	}
	return connectorPublicationDomain(row), nil
}

func (repository *Repository) ListConnectorPublications(ctx context.Context, includeDisabled bool) ([]domain.ConnectorPublication, error) {
	query := repository.db.WithContext(ctx).Order("package_source")
	if !includeDisabled {
		query = query.Where("state = ?", domain.ConnectorPublicationAvailable)
	}
	var rows []connectorPublicationRecord
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Connector Publications: %w", err)
	}
	items := make([]domain.ConnectorPublication, 0, len(rows))
	for _, row := range rows {
		items = append(items, connectorPublicationDomain(row))
	}
	return items, nil
}

func (repository *Repository) ListConnectorPublicationHealth(ctx context.Context) ([]domain.ConnectorPublicationHealth, error) {
	publications, err := repository.ListConnectorPublications(ctx, true)
	if err != nil {
		return nil, err
	}
	items := make([]domain.ConnectorPublicationHealth, 0, len(publications))
	for _, publication := range publications {
		revision, readErr := repository.GetConnectorRevision(ctx, publication.ActiveRevisionID)
		if readErr != nil {
			return nil, readErr
		}
		var installationCount, activeInstallationCount, activeAuthorizationCount int64
		if err := repository.db.WithContext(ctx).Model(&connectorInstallationRecord{}).Where("package_source = ? AND state <> ?", publication.PackageSource, domain.ConnectorInstallationUninstalled).Count(&installationCount).Error; err != nil {
			return nil, err
		}
		if err := repository.db.WithContext(ctx).Model(&connectorInstallationRecord{}).Where("package_source = ? AND state = ?", publication.PackageSource, domain.ConnectorInstallationActive).Count(&activeInstallationCount).Error; err != nil {
			return nil, err
		}
		if err := repository.db.WithContext(ctx).Table("connector_authorizations AS auth").Joins("JOIN connector_installations AS installation ON installation.id = auth.installation_id").Where("installation.package_source = ? AND auth.state = ?", publication.PackageSource, domain.ConnectorAuthorizationActive).Count(&activeAuthorizationCount).Error; err != nil {
			return nil, err
		}
		items = append(items, domain.ConnectorPublicationHealth{Publication: publication, Revision: revision, InstallationCount: installationCount, ActiveInstallationCount: activeInstallationCount, ActiveAuthorizationCount: activeAuthorizationCount})
	}
	return items, nil
}

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
					if err := tx.Model(&connectorAuthorizationRecord{}).Where("id = ?", *row.AuthorizationID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "credential_ciphertext": []byte{}, "refresh_credential_ciphertext": nil, "refresh_credential_aad": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
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
	query := repository.db.WithContext(ctx).Table("connector_installations AS installation").Joins("JOIN connector_revisions AS revision ON revision.id = installation.active_revision_id").Where("installation.owner_user_id = ? AND installation.state <> ? AND COALESCE(revision.runtime_policy->>'legacy_projection', 'false') <> 'true'", ownerID, domain.ConnectorInstallationUninstalled).Select("installation.*").Order("installation.package_source, installation.id")
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Connector Installations: %w", err)
	}
	items := make([]domain.ConnectorInstallation, 0, len(rows))
	for _, row := range rows {
		item := connectorInstallationDomain(row)
		var revision connectorRevisionRecord
		if err := repository.db.WithContext(ctx).Where("id = ?", row.ActiveRevisionID).Take(&revision).Error; err != nil {
			return nil, fmt.Errorf("read Connector Revision: %w", err)
		}
		var policy struct {
			AuthMode string `json:"auth_mode"`
		}
		if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil {
			return nil, fmt.Errorf("decode Connector authorization policy: %w", err)
		}
		item.Authorized = policy.AuthMode == "none"
		if !item.Authorized && item.AuthorizationID != "" {
			var count int64
			if err := repository.db.WithContext(ctx).Model(&connectorAuthorizationRecord{}).Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", item.AuthorizationID, item.ID, ownerID, domain.ConnectorAuthorizationActive).Count(&count).Error; err != nil {
				return nil, fmt.Errorf("read Connector authorization state: %w", err)
			}
			item.Authorized = count == 1
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) ListConnectorAuthorizations(ctx context.Context, ownerID, installationID string) ([]domain.ConnectorAuthorization, error) {
	if ownerID == "" || installationID == "" {
		return nil, fmt.Errorf("%w: connector authorization owner and installation are required", domain.ErrInvalid)
	}
	var installation connectorInstallationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND state <> ?", installationID, ownerID, domain.ConnectorInstallationUninstalled).Take(&installation).Error; err != nil {
		return nil, mapNotFound(err)
	}
	var rows []connectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("installation_id = ? AND owner_user_id = ?", installationID, ownerID).Order("updated_at DESC, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list Connector Authorizations: %w", err)
	}
	items := make([]domain.ConnectorAuthorization, 0, len(rows))
	for _, row := range rows {
		items = append(items, connectorAuthorizationDomain(row))
	}
	return items, nil
}

func (repository *Repository) SelectConnectorAuthorization(ctx context.Context, ownerID, installationID, authorizationID string, expectedVersion int64) (domain.ConnectorInstallation, error) {
	return repository.selectConnectorAuthorization(ctx, ownerID, installationID, authorizationID, expectedVersion, nil)
}

func (repository *Repository) SelectConnectorAuthorizationWithAudit(ctx context.Context, ownerID, installationID, authorizationID string, expectedVersion int64, audit domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	return repository.selectConnectorAuthorization(ctx, ownerID, installationID, authorizationID, expectedVersion, &audit)
}

func (repository *Repository) selectConnectorAuthorization(ctx context.Context, ownerID, installationID, authorizationID string, expectedVersion int64, audit *domain.ConnectorAuditRecord) (domain.ConnectorInstallation, error) {
	if ownerID == "" || installationID == "" || authorizationID == "" || expectedVersion <= 0 {
		return domain.ConnectorInstallation{}, fmt.Errorf("%w: connector authorization selection is incomplete", domain.ErrInvalid)
	}
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ? AND version = ? AND state = ?", installationID, ownerID, expectedVersion, domain.ConnectorInstallationActive).Take(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return domain.ErrConflict
			}
			return err
		}
		var authorization connectorAuthorizationRecord
		if err := tx.Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", authorizationID, installationID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error; err != nil {
			return mapNotFound(err)
		}
		result := tx.Model(&connectorInstallationRecord{}).Where("id = ? AND version = ?", row.ID, row.Version).Updates(map[string]any{"authorization_id": authorization.ID, "version": gorm.Expr("version + 1"), "updated_at": gorm.Expr("now()")})
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
			if audit.IdentityRef == "" {
				audit.IdentityRef = authorization.IdentityRef
			}
			return createConnectorAuditTx(tx, *audit)
		}
		return nil
	})
	if err != nil {
		return domain.ConnectorInstallation{}, fmt.Errorf("select Connector Authorization: %w", err)
	}
	return connectorInstallationDomain(row), nil
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
			if err := tx.Model(&connectorAuthorizationRecord{}).Where("installation_id = ? AND owner_user_id = ?", installationID, ownerID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "credential_ciphertext": []byte{}, "refresh_credential_ciphertext": nil, "refresh_credential_aad": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
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
	return repository.changeConnectorRevision(ctx, ownerID, installationID, revisionID, "", 0, now)
}

func (repository *Repository) ActivateConnectorRevisionWithVersion(ctx context.Context, ownerID, installationID, revisionID string, expectedVersion int64, now time.Time) (domain.ConnectorInstallation, error) {
	if expectedVersion <= 0 {
		return domain.ConnectorInstallation{}, fmt.Errorf("%w: expected Connector Installation version is required", domain.ErrInvalid)
	}
	return repository.changeConnectorRevision(ctx, ownerID, installationID, revisionID, "", expectedVersion, now)
}

func (repository *Repository) RollbackConnectorRevision(ctx context.Context, ownerID, installationID, revisionID, reason string, now time.Time) (domain.ConnectorInstallation, error) {
	return repository.changeConnectorRevision(ctx, ownerID, installationID, revisionID, reason, 0, now)
}

func (repository *Repository) changeConnectorRevision(ctx context.Context, ownerID, installationID, revisionID, reason string, expectedVersion int64, now time.Time) (domain.ConnectorInstallation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var row connectorInstallationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", installationID, ownerID)
		if expectedVersion > 0 {
			query = query.Where("version = ?", expectedVersion)
		}
		if err := query.Take(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound && expectedVersion > 0 {
				return domain.ErrConflict
			}
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
					Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "credential_ciphertext": []byte{}, "refresh_credential_ciphertext": nil, "refresh_credential_aad": "", "updated_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
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
	return repository.disconnectConnectorAuthorization(ctx, ownerID, authorizationID, nil)
}

func (repository *Repository) DisconnectConnectorAuthorizationWithAudit(ctx context.Context, ownerID, authorizationID string, audit domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error) {
	return repository.disconnectConnectorAuthorization(ctx, ownerID, authorizationID, &audit)
}

func (repository *Repository) disconnectConnectorAuthorization(ctx context.Context, ownerID, authorizationID string, audit *domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error) {
	var row connectorAuthorizationRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", authorizationID, ownerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State != string(domain.ConnectorAuthorizationDisconnected) {
			result := tx.Model(&connectorAuthorizationRecord{}).Where("id = ? AND version = ?", authorizationID, row.Version).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "credential_ciphertext": []byte{}, "refresh_credential_ciphertext": nil, "refresh_credential_aad": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
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
		if audit != nil {
			if audit.InstallationID == "" {
				audit.InstallationID = row.InstallationID
			}
			if audit.IdentityRef == "" {
				audit.IdentityRef = row.IdentityRef
			}
			if audit.RevisionID == "" {
				var installation connectorInstallationRecord
				if err := tx.Where("id = ?", row.InstallationID).Take(&installation).Error; err != nil {
					return err
				}
				audit.RevisionID = installation.ActiveRevisionID
			}
			if err := createConnectorAuditTx(tx, *audit); err != nil {
				return err
			}
		}
		return tx.Where("id = ?", authorizationID).Take(&row).Error
	})
	if err != nil {
		return domain.ConnectorAuthorization{}, err
	}
	return connectorAuthorizationDomain(row), nil
}

func (repository *Repository) RefreshConnectorAuthorization(ctx context.Context, input domain.ConnectorAuthorization, expectedVersion int64, audit domain.ConnectorAuditRecord) (domain.ConnectorAuthorization, error) {
	if input.ID == "" || input.OwnerID == "" || input.InstallationID == "" || input.IdentityRef == "" || input.ExternalIdentityID == "" || len(input.CredentialCiphertext) == 0 || input.CredentialAAD == "" || input.CredentialFormat != "json" || input.ExpiresAt == nil || !time.Now().UTC().Before(*input.ExpiresAt) || expectedVersion <= 0 {
		return domain.ConnectorAuthorization{}, fmt.Errorf("%w: refreshed Connector authorization is incomplete", domain.ErrInvalid)
	}
	scopes, err := json.Marshal(input.Scopes)
	if err != nil {
		return domain.ConnectorAuthorization{}, fmt.Errorf("encode refreshed Connector Authorization scopes: %w", err)
	}
	var row connectorAuthorizationRecord
	err = repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation connectorInstallationRecord
		if err := tx.Where("id = ? AND owner_user_id = ? AND state = ?", input.InstallationID, input.OwnerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
			return mapNotFound(err)
		}
		if audit.InstallationID == "" {
			audit.InstallationID = installation.ID
		}
		if audit.RevisionID == "" {
			audit.RevisionID = installation.ActiveRevisionID
		}
		result := tx.Model(&connectorAuthorizationRecord{}).
			Where("id = ? AND installation_id = ? AND owner_user_id = ? AND version = ? AND state IN ?", input.ID, input.InstallationID, input.OwnerID, expectedVersion, []string{string(domain.ConnectorAuthorizationActive), string(domain.ConnectorAuthorizationExpired)}).
			Updates(map[string]any{
				"identity_ref": input.IdentityRef, "external_identity_id": input.ExternalIdentityID,
				"external_display_name": input.ExternalDisplayName, "scopes": scopes,
				"credential_ciphertext": input.CredentialCiphertext, "credential_aad": input.CredentialAAD,
				"refresh_credential_ciphertext": nil, "refresh_credential_aad": "",
				"credential_format": input.CredentialFormat, "state": string(domain.ConnectorAuthorizationActive),
				"expires_at": input.ExpiresAt, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if err := createConnectorAuditTx(tx, audit); err != nil {
			return err
		}
		return tx.Where("id = ?", input.ID).Take(&row).Error
	})
	if err != nil {
		return domain.ConnectorAuthorization{}, fmt.Errorf("refresh Connector Authorization: %w", err)
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
			if err := tx.Model(&connectorAuthorizationRecord{}).Where("id = ? AND owner_user_id = ?", *row.AuthorizationID, ownerID).Updates(map[string]any{"state": string(domain.ConnectorAuthorizationDisconnected), "credential_ciphertext": []byte{}, "refresh_credential_ciphertext": nil, "refresh_credential_aad": "", "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
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
	if input.CredentialFormat == "" {
		input.CredentialFormat = "json"
	}
	if input.CredentialFormat != "json" && input.CredentialFormat != "access_token" {
		return domain.ConnectorAuthorization{}, fmt.Errorf("%w: Connector authorization credential format is unsupported", domain.ErrInvalid)
	}
	row := connectorAuthorizationRecord{ID: input.ID, OwnerID: input.OwnerID, InstallationID: input.InstallationID, IdentityRef: input.IdentityRef, ExternalIdentityID: input.ExternalIdentityID, ExternalDisplayName: input.ExternalDisplayName, Scopes: scopes, CredentialCiphertext: input.CredentialCiphertext, CredentialAAD: input.CredentialAAD, CredentialFormat: input.CredentialFormat, RefreshCredentialCiphertext: input.RefreshCredentialCiphertext, RefreshCredentialAAD: input.RefreshCredentialAAD, State: string(input.State), ExpiresAt: input.ExpiresAt, Version: input.Version, UpdatedAt: now}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation connectorInstallationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND owner_user_id = ? AND state <> ?", input.InstallationID, input.OwnerID, domain.ConnectorInstallationUninstalled).
			Take(&installation).Error; err != nil {
			return mapNotFound(err)
		}
		if row.ExternalIdentityID != "" {
			var existing connectorAuthorizationRecord
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND installation_id = ? AND identity_ref = ? AND external_identity_id = ?", row.OwnerID, row.InstallationID, row.IdentityRef, row.ExternalIdentityID).Take(&existing).Error
			switch {
			case err == nil:
				row.ID = existing.ID
				result := tx.Model(&connectorAuthorizationRecord{}).Where("id = ? AND version = ?", existing.ID, existing.Version).Updates(map[string]any{
					"external_display_name": row.ExternalDisplayName, "scopes": row.Scopes,
					"credential_ciphertext": row.CredentialCiphertext, "credential_aad": row.CredentialAAD,
					"credential_format": row.CredentialFormat, "refresh_credential_ciphertext": row.RefreshCredentialCiphertext,
					"refresh_credential_aad": row.RefreshCredentialAAD, "state": row.State, "expires_at": row.ExpiresAt,
					"updated_at": row.UpdatedAt, "version": gorm.Expr("version + 1"),
				})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return domain.ErrConflict
				}
				if err := tx.Where("id = ?", row.ID).Take(&row).Error; err != nil {
					return err
				}
			case err == gorm.ErrRecordNotFound:
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			default:
				return err
			}
		} else if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Model(&connectorInstallationRecord{}).
			Where("id = ? AND owner_user_id = ?", input.InstallationID, input.OwnerID).
			Updates(map[string]any{"authorization_id": row.ID, "updated_at": row.UpdatedAt, "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		if audit != nil {
			if audit.InstallationID == "" {
				audit.InstallationID = installation.ID
			}
			if audit.RevisionID == "" {
				audit.RevisionID = installation.ActiveRevisionID
			}
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
		if err != gorm.ErrRecordNotFound {
			return err
		}
		return repository.ValidateConnectorPackageCLIInvocation(ctx, ownerID, definitionID, "", "")
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

func (repository *Repository) ValidateConnectorPackageCLIInvocation(ctx context.Context, ownerID, installationID, revisionID, authorizationID string) error {
	var installation connectorInstallationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ? AND state = ?", installationID, ownerID, domain.ConnectorInstallationActive).Take(&installation).Error; err != nil {
		return fmt.Errorf("%w: Connector installation is unavailable", domain.ErrConflict)
	}
	if revisionID == "" {
		revisionID = installation.ActiveRevisionID
	}
	var revision connectorRevisionRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND package_source = ? AND mode = ?", revisionID, installation.PackageSource, domain.ConnectorModeCLI).Take(&revision).Error; err != nil {
		return fmt.Errorf("%w: Connector revision is unavailable", domain.ErrConflict)
	}
	var publication connectorPublicationRecord
	if err := repository.db.WithContext(ctx).Where("package_source = ?", installation.PackageSource).Take(&publication).Error; err == nil && publication.State != string(domain.ConnectorPublicationAvailable) {
		return fmt.Errorf("%w: Connector publication is disabled", domain.ErrConflict)
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	var policy connectorCLIPolicy
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.LegacyProjection || policy.CLI == nil || policy.BundleObjectKey == "" || policy.BundleSHA256 == "" {
		return fmt.Errorf("%w: Connector CLI bundle is unavailable", domain.ErrConflict)
	}
	if policy.AuthMode == "none" {
		return nil
	}
	if authorizationID == "" && installation.AuthorizationID != nil {
		authorizationID = *installation.AuthorizationID
	}
	if authorizationID == "" {
		return fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	var authorization connectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND installation_id = ? AND owner_user_id = ? AND state = ? AND (expires_at IS NULL OR expires_at > now())", authorizationID, installation.ID, ownerID, domain.ConnectorAuthorizationActive).Take(&authorization).Error; err != nil {
		return fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	return nil
}

func (repository *Repository) ResolveConnectorPackageAuthorization(ctx context.Context, ownerID, installationID, revisionID, authorizationID, identity string) (domain.ConnectorAuthorizationMaterial, error) {
	if err := repository.ValidateConnectorPackageCLIInvocation(ctx, ownerID, installationID, revisionID, authorizationID); err != nil {
		return domain.ConnectorAuthorizationMaterial{}, err
	}
	var installation connectorInstallationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", installationID, ownerID).Take(&installation).Error; err != nil {
		return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	if authorizationID == "" && installation.AuthorizationID != nil {
		authorizationID = *installation.AuthorizationID
	}
	if authorizationID == "" {
		return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	var authorization connectorAuthorizationRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND installation_id = ? AND owner_user_id = ?", authorizationID, installationID, ownerID).Take(&authorization).Error; err != nil {
		return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector authorization is unavailable", domain.ErrConflict)
	}
	if authorization.IdentityRef != "" && authorization.IdentityRef != identity && authorization.IdentityRef != "user" {
		return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector identity is not authorized", domain.ErrConflict)
	}
	material := domain.ConnectorAuthorizationMaterial{CredentialCiphertext: append([]byte(nil), authorization.CredentialCiphertext...), CredentialAAD: authorization.CredentialAAD, CredentialFormat: authorization.CredentialFormat}
	if err := json.Unmarshal(authorization.Scopes, &material.Scopes); err != nil {
		return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector authorization scopes are invalid: %v", domain.ErrConflict, err)
	}
	if material.CredentialAAD == "" {
		material.CredentialAAD = "connector-authorization:" + ownerID
	}
	if installation.PackageSource == "feishu" || material.CredentialFormat == "access_token" {
		application, err := repository.GetConnectorProviderApplication(ctx, ownerID, installationID)
		if err == nil {
			material.AppIDCiphertext = append([]byte(nil), application.AppIDCiphertext...)
			material.AppSecretCiphertext = append([]byte(nil), application.AppSecretCiphertext...)
		} else if material.CredentialFormat == "access_token" {
			return domain.ConnectorAuthorizationMaterial{}, fmt.Errorf("%w: Connector provider application is unavailable: %w", domain.ErrConflict, err)
		} else if !errors.Is(err, domain.ErrNotFound) {
			return domain.ConnectorAuthorizationMaterial{}, err
		}
	}
	return material, nil
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
		LegacyProjection bool   `json:"legacy_projection"`
		AuthMode         string `json:"auth_mode"`
	}
	if err := json.Unmarshal(revision.RuntimePolicy, &policy); err != nil || policy.LegacyProjection || policy.AuthMode == "" {
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

func connectorPublicationDomain(row connectorPublicationRecord) domain.ConnectorPublication {
	return domain.ConnectorPublication{PackageSource: row.PackageSource, ActiveRevisionID: row.ActiveRevisionID, State: domain.ConnectorPublicationState(row.State), AdministratorID: row.AdministratorID, Version: row.Version, UpdatedAt: row.UpdatedAt}
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
	return domain.ConnectorAuthorization{ID: row.ID, OwnerID: row.OwnerID, InstallationID: row.InstallationID, IdentityRef: row.IdentityRef, ExternalIdentityID: row.ExternalIdentityID, ExternalDisplayName: row.ExternalDisplayName, Scopes: scopes, CredentialCiphertext: row.CredentialCiphertext, CredentialAAD: row.CredentialAAD, CredentialFormat: row.CredentialFormat, RefreshCredentialCiphertext: row.RefreshCredentialCiphertext, RefreshCredentialAAD: row.RefreshCredentialAAD, State: domain.ConnectorAuthorizationState(row.State), ExpiresAt: row.ExpiresAt, Version: row.Version, UpdatedAt: row.UpdatedAt}
}
