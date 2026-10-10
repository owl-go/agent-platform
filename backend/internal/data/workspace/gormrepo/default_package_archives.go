package gormrepo

import (
	"fmt"

	"agent-platform/backend/internal/expertpackage"
	"gorm.io/gorm"
)

// Retain the source package in the same transaction as its managed definition.
// Export can then preserve non-executable display metadata without rebuilding it.
func retainDefaultPackage(tx *gorm.DB, owner, kind, key string, pkg expertpackage.Package) error {
	var seed defaultResourceSeedRecord
	if err := tx.Where("kind=? AND resource_key=?", kind, key).Take(&seed).Error; err != nil {
		return err
	}
	if !seed.Managed {
		return nil
	}
	var version int64
	table := "experts"
	if kind == "expert_team" {
		table = "expert_teams"
	}
	if err := tx.Table(table).Select("version").Where("id=? AND owner_user_id=? AND system_managed=true", seed.ResourceID, owner).Scan(&version).Error; err != nil {
		return err
	}
	if version <= 0 {
		return fmt.Errorf("managed default package has no matching resource")
	}
	archive, err := pkg.Archive()
	if err != nil {
		return err
	}
	if err := tx.Exec("INSERT INTO expert_package_imports(owner_user_id,package_id,package_version,content_sha256,kind,resource_id,resource_version,archive) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(owner_user_id,package_id,package_version) DO NOTHING", owner, pkg.Manifest.ID, pkg.Manifest.Version, pkg.SHA256, kind, seed.ResourceID, version, archive).Error; err != nil {
		return err
	}
	var saved struct {
		ContentSHA256   string
		Kind            string
		ResourceID      string
		ResourceVersion int64
	}
	if err := tx.Table("expert_package_imports").Where("owner_user_id=? AND package_id=? AND package_version=?", owner, pkg.Manifest.ID, pkg.Manifest.Version).Take(&saved).Error; err != nil {
		return err
	}
	if saved.ContentSHA256 != pkg.SHA256 || saved.Kind != kind || saved.ResourceID != seed.ResourceID || saved.ResourceVersion != version {
		return fmt.Errorf("default package conflicts with an existing source revision")
	}
	return nil
}
