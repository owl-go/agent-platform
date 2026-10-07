package gormrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/defaultresources"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/skillstore"
	"agent-platform/backend/internal/systemskills"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type defaultResourceSeedRecord struct {
	Kind           string    `gorm:"column:kind;primaryKey"`
	ResourceKey    string    `gorm:"column:resource_key;primaryKey"`
	ResourceID     string    `gorm:"column:resource_id"`
	CatalogVersion string    `gorm:"column:catalog_version"`
	ContentSHA256  string    `gorm:"column:content_sha256"`
	Managed        bool      `gorm:"column:managed"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (defaultResourceSeedRecord) TableName() string { return "default_resource_seeds" }

// EnsureDefaultResources installs only credential-free platform definitions.
// API and Worker serialize initialization using one transaction-scoped lock.
// Existing administrator resources, publications and every private resource
// remain under their existing owner's control.
func (repository *Repository) EnsureDefaultResources(ctx context.Context, objects objectstore.Provider) error {
	catalog, err := defaultresources.LoadContext(ctx)
	if err != nil {
		return err
	}
	return repository.ensureResources(ctx, objects, catalog)
}

func (repository *Repository) EnsureSystemSkills(ctx context.Context, objects objectstore.Provider) error {
	return repository.ensureResources(ctx, objects, defaultresources.Catalog{})
}

func (repository *Repository) ensureResources(ctx context.Context, objects objectstore.Provider, catalog defaultresources.Catalog) error {
	if objects == nil {
		return fmt.Errorf("default resource Object Store is required")
	}
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('agent-workspace.default-resources'))").Error; err != nil {
			return fmt.Errorf("lock default resources: %w", err)
		}
		var administrator struct {
			ID string `gorm:"column:id"`
		}
		if err := tx.Table("users").Select("id").Where("bootstrap_administrator = true AND administrator = true").Take(&administrator).Error; err != nil {
			return fmt.Errorf("find bootstrap Administrator for default resources: %w", err)
		}
		skills := map[string]string{}
		for _, definition := range systemskills.Definitions() {
			archive, digest, err := systemskills.Archive(definition.Key)
			if err != nil {
				return err
			}
			id, err := seedSkill(ctx, tx, objects, administrator.ID, definition.Key, digest, definition.Name, "sparkles", archive)
			if err != nil {
				return fmt.Errorf("initialize system Skill %s: %w", definition.Key, err)
			}
			skills[definition.Key] = id
		}
		for _, definition := range catalog.Skills {
			archive, err := catalog.Archive(definition.Archive)
			if err != nil {
				return err
			}
			id, err := seedSkill(ctx, tx, objects, administrator.ID, definition.Key, definition.Version, definition.Name, definition.Icon, archive)
			if err != nil {
				return fmt.Errorf("initialize default Skill %s: %w", definition.Key, err)
			}
			skills[definition.Key] = id
		}
		for _, definition := range catalog.Experts {
			if err := seedExpert(tx, administrator.ID, definition, skills); err != nil {
				return fmt.Errorf("initialize default Expert %s: %w", definition.Key, err)
			}
		}
		for _, definition := range catalog.Connectors {
			if err := seedConnector(ctx, tx, objects, administrator.ID, catalog, definition); err != nil {
				return fmt.Errorf("initialize default Connector %s: %w", definition.Source, err)
			}
		}
		return nil
	})
}

func readSeed(tx *gorm.DB, kind, key, version, digest string) (defaultResourceSeedRecord, error) {
	var seed defaultResourceSeedRecord
	err := tx.Where("kind = ? AND resource_key = ?", kind, key).Take(&seed).Error
	if err == nil && seed.Managed && seed.CatalogVersion == version && seed.ContentSHA256 != digest {
		return seed, fmt.Errorf("default %s %s changed without a catalog version update", kind, key)
	}
	return seed, err
}
func saveSeed(tx *gorm.DB, kind, key, id, version, digest string, managed bool) error {
	row := defaultResourceSeedRecord{Kind: kind, ResourceKey: key, ResourceID: id, CatalogVersion: version, ContentSHA256: digest, Managed: managed, UpdatedAt: time.Now().UTC()}
	return tx.Save(&row).Error
}
func contentDigest(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func storeDefaultObject(ctx context.Context, objects objectstore.Provider, key string, data []byte, digest, contentType string) error {
	object, err := objects.Stat(ctx, key)
	if errors.Is(err, objectstore.ErrNotFound) {
		object, err = objects.Put(ctx, key, bytes.NewReader(data), objectstore.PutOptions{Size: int64(len(data)), SHA256: digest, ContentType: contentType})
	}
	if err != nil {
		return fmt.Errorf("store default resource object: %w", err)
	}
	if object.SHA256 != digest || object.Size != int64(len(data)) {
		return fmt.Errorf("default resource object checksum or size mismatch")
	}
	return nil
}

func seedSkill(ctx context.Context, tx *gorm.DB, objects objectstore.Provider, owner, key, version, name, icon string, archive []byte) (string, error) {
	normalized, digest, _, err := skillstore.ValidateUpload(ctx, archive)
	if err != nil {
		return "", err
	}
	definitionDigest := contentDigest([]string{digest, name, icon})
	seed, seedErr := readSeed(tx, "skill", key, version, definitionDigest)
	if seedErr != nil && !errors.Is(seedErr, gorm.ErrRecordNotFound) {
		return "", seedErr
	}
	var row skillRecord
	err = tx.Where("system_key = ?", key).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if seedErr == nil {
			if seed.Managed {
				return "", fmt.Errorf("managed default Skill is missing")
			}
			if err = tx.Where("id = ? AND owner_user_id = ?", seed.ResourceID, owner).Take(&row).Error; err != nil {
				return "", fmt.Errorf("existing administrator Skill for default binding is missing: %w", err)
			}
			return row.ID, nil
		}
		err = tx.Where("owner_user_id = ? AND name_normalized = ?", owner, normalizeResourceName(name)).Take(&row).Error
		if err == nil {
			return row.ID, saveSeed(tx, "skill", key, row.ID, version, definitionDigest, false)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		row = skillRecord{ID: uuid.NewString(), OwnerID: owner, SystemKey: key, SystemManaged: true, Name: name, NameNormalized: normalizeResourceName(name), Icon: icon, Source: "upload", CreatedAt: time.Now().UTC(), Version: 1}
	} else if !row.SystemManaged || row.OwnerID != owner {
		return "", fmt.Errorf("default Skill has invalid ownership")
	}
	objectKey := "skills/default/" + digest + ".zip"
	if err = storeDefaultObject(ctx, objects, objectKey, normalized, digest, "application/zip"); err != nil {
		return "", err
	}
	if row.SHA256 == digest && row.ObjectKey == objectKey && row.Name == name && row.Icon == icon {
		return row.ID, saveSeed(tx, "skill", key, row.ID, version, definitionDigest, true)
	}
	if row.SHA256 != "" {
		row.Version++
	}
	row.Name = name
	row.NameNormalized = normalizeResourceName(name)
	row.Icon = icon
	row.SHA256 = digest
	row.ObjectKey = objectKey
	row.UpdatedAt = time.Now().UTC()
	if err = tx.Save(&row).Error; err != nil {
		return "", err
	}
	return row.ID, saveSeed(tx, "skill", key, row.ID, version, definitionDigest, true)
}

func seedExpert(tx *gorm.DB, owner string, definition defaultresources.Expert, skills map[string]string) error {
	ids := []string{}
	for _, key := range definition.SkillKeys {
		if skills[key] == "" {
			return fmt.Errorf("default Expert references missing Skill %s", key)
		}
		ids = append(ids, skills[key])
	}
	input := domain.ExpertInput{Name: definition.Name, Icon: definition.Icon, IconBackground: definition.IconBackground, Introduction: definition.Introduction, CoreCapability: definition.CoreCapability, OperatingProcedure: definition.OperatingProcedure, OutputStandard: definition.OutputStandard, Cautions: definition.Cautions, SkillIDs: ids}
	if err := input.Validate(); err != nil {
		return err
	}
	digest := contentDigest(definition)
	seed, seedErr := readSeed(tx, "expert", definition.Key, definition.Version, digest)
	if seedErr != nil && !errors.Is(seedErr, gorm.ErrRecordNotFound) {
		return seedErr
	}
	if seedErr == nil && !seed.Managed {
		return nil
	}
	var row expertRecord
	err := tx.Where("system_key = ?", definition.Key).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if seedErr == nil {
			return fmt.Errorf("managed default Expert is missing")
		}
		if err = tx.Where("owner_user_id = ? AND name_normalized = ?", owner, normalizeResourceName(definition.Name)).Take(&row).Error; err == nil {
			return saveSeed(tx, "expert", definition.Key, row.ID, definition.Version, digest, false)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = expertRecord{ID: uuid.NewString(), OwnerID: owner, SystemKey: definition.Key, SystemManaged: true, CreatedAt: time.Now().UTC(), Version: 1}
	} else {
		if !row.SystemManaged || row.OwnerID != owner {
			return fmt.Errorf("default Expert has invalid ownership")
		}
		if seedErr == nil && seed.CatalogVersion == definition.Version && seed.ContentSHA256 == digest {
			return nil
		}
		row.Version++
	}
	row.Name = definition.Name
	row.NameNormalized = normalizeResourceName(definition.Name)
	row.Icon = definition.Icon
	row.IconBackground = definition.IconBackground
	row.Introduction = definition.Introduction
	row.CoreCapability = definition.CoreCapability
	row.OperatingProcedure = definition.OperatingProcedure
	row.OutputStandard = definition.OutputStandard
	row.Cautions = definition.Cautions
	row.SkillIDs, _ = json.Marshal(ids)
	row.MCPServerIDs = []byte("[]")
	row.CLIConnectorDefinitionIDs = []byte("[]")
	row.ExpertiseTags = []byte("[]")
	row.TagProjectionStatus = "queued"
	now := time.Now().UTC()
	row.TagProjectionRequestedAt = &now
	row.UpdatedAt = now
	if err = tx.Save(&row).Error; err != nil {
		return err
	}
	return saveSeed(tx, "expert", definition.Key, row.ID, definition.Version, digest, true)
}

func seedConnector(ctx context.Context, tx *gorm.DB, objects objectstore.Provider, owner string, catalog defaultresources.Catalog, definition defaultresources.Connector) error {
	pkg, err := catalog.ParseConnector(definition)
	if err != nil {
		return err
	}
	seed, seedErr := readSeed(tx, "connector", definition.Source, definition.Version, pkg.SHA256)
	if seedErr != nil && !errors.Is(seedErr, gorm.ErrRecordNotFound) {
		return seedErr
	}
	if seedErr == nil && !seed.Managed {
		return nil
	}
	// Revisions may belong to a private installation even without a publication.
	// A source/version collision must never promote that private package.
	if errors.Is(seedErr, gorm.ErrRecordNotFound) {
		var existing connectorRevisionRecord
		findErr := tx.Where("package_source = ? AND version = ?", definition.Source, definition.Version).Take(&existing).Error
		if findErr == nil && existing.PackageSHA256 != pkg.SHA256 {
			return saveSeed(tx, "connector", definition.Source, existing.ID, definition.Version, pkg.SHA256, false)
		}
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
	}
	var publication connectorPublicationRecord
	publicationErr := tx.Where("package_source = ?", definition.Source).Take(&publication).Error
	if publicationErr != nil && !errors.Is(publicationErr, gorm.ErrRecordNotFound) {
		return publicationErr
	}
	if publicationErr == nil && (seedErr != nil || !seed.Managed || publication.ActiveRevisionID != seed.ResourceID) {
		return saveSeed(tx, "connector", definition.Source, publication.ActiveRevisionID, definition.Version, pkg.SHA256, false)
	}
	if err = storeDefaultObject(ctx, objects, pkg.ObjectKey(), pkg.NormalizedArchive, pkg.SHA256, "application/zip"); err != nil {
		return err
	}
	if len(pkg.CLIBundle) > 0 {
		if err = storeDefaultObject(ctx, objects, pkg.BundleObjectKey(), pkg.CLIBundle, pkg.CLIBundleSHA256, "application/gzip"); err != nil {
			return err
		}
	}
	policy, err := pkg.RuntimePolicy()
	if err != nil {
		return err
	}
	repo := New(tx, nil)
	revision, err := repo.CreateConnectorRevision(ctx, domain.ConnectorRevision{PackageSource: pkg.Metadata.Source, Version: pkg.Metadata.Version, Mode: domain.ConnectorMode(pkg.Metadata.Type), PackageSHA256: pkg.SHA256, ObjectKey: pkg.ObjectKey(), RuntimePolicy: policy})
	if err != nil {
		return err
	}
	state := string(domain.ConnectorPublicationAvailable)
	if pkg.CLI != nil {
		state = string(domain.ConnectorPublicationDisabled)
	}
	if publicationErr == nil {
		if publication.ActiveRevisionID == revision.ID {
			return saveSeed(tx, "connector", definition.Source, revision.ID, definition.Version, pkg.SHA256, true)
		}
		// Preserve an Administrator's explicit disabled state on package upgrades.
		if publication.State == string(domain.ConnectorPublicationDisabled) {
			state = publication.State
		}
		result := tx.Model(&connectorPublicationRecord{}).Where("package_source = ? AND version = ?", publication.PackageSource, publication.Version).Updates(map[string]any{"active_revision_id": revision.ID, "state": state, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
		err = result.Error
		if err == nil && result.RowsAffected != 1 {
			return fmt.Errorf("default Connector publication changed during initialization")
		}
	} else {
		err = tx.Create(&connectorPublicationRecord{PackageSource: definition.Source, ActiveRevisionID: revision.ID, State: state, AdministratorID: owner, Version: 1, UpdatedAt: time.Now().UTC()}).Error
	}
	if err != nil {
		return err
	}
	return saveSeed(tx, "connector", definition.Source, revision.ID, definition.Version, pkg.SHA256, true)
}
