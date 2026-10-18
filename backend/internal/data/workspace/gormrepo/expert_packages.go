package gormrepo

import (
	"context"
	"fmt"
	"regexp"

	"agent-platform/backend/internal/biz/workspace/domain"
	"gorm.io/gorm"
)

func (repository *Repository) ImportExpertPackage(ctx context.Context, ownerID string, input domain.ExpertPackageDefinition) (domain.ExpertPackageImport, error) {
	var result domain.ExpertPackageImport
	if input.PackageID == "" || input.PackageVersion == "" || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(input.SHA256) || (input.Expert == nil) == (input.Team == nil) {
		return result, domain.ErrInvalid
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.Team != nil {
			if err := requireTeamAdministrator(tx, ownerID); err != nil {
				return err
			}
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "expert-package:"+ownerID+":"+input.PackageID).Error; err != nil {
			return err
		}
		var prior struct {
			ContentSHA256   string
			Kind            string
			ResourceID      string
			ResourceVersion int64
		}
		query := tx.Table("expert_package_imports").Where("owner_user_id=? AND package_id=? AND package_version=?", ownerID, input.PackageID, input.PackageVersion).Take(&prior)
		scoped := New(tx, repository.credits)
		if query.Error == nil {
			if input.TargetResourceID != "" && prior.ResourceID != input.TargetResourceID {
				return domain.ErrConflict
			}
			if prior.ContentSHA256 != input.SHA256 {
				return fmt.Errorf("%w: package version has different content", domain.ErrConflict)
			}
			if prior.Kind == "expert" {
				item, err := scoped.GetExpert(ctx, ownerID, prior.ResourceID)
				if err != nil {
					return err
				}
				result.Expert = &item
			} else {
				item, err := scoped.GetExpertTeam(ctx, ownerID, prior.ResourceID)
				if err != nil {
					return err
				}
				result.Team = &item
			}
			result.Replayed = true
			return nil
		}
		if query.Error != gorm.ErrRecordNotFound {
			return query.Error
		}
		var head struct {
			PackageVersion  string
			ResourceID      string
			Kind            string
			ResourceVersion int64
		}
		headQuery := tx.Table("expert_package_imports").Where("owner_user_id=? AND package_id=?", ownerID, input.PackageID).Order("resource_version DESC").Take(&head)
		if headQuery.Error != nil && headQuery.Error != gorm.ErrRecordNotFound {
			return headQuery.Error
		}
		if headQuery.Error == nil {
			if input.TargetResourceID != "" && input.TargetResourceID != head.ResourceID {
				return domain.ErrConflict
			}
			if domain.ComparePackageVersions(input.PackageVersion, head.PackageVersion) <= 0 {
				return fmt.Errorf("%w: package upgrade requires a newer semantic version", domain.ErrConflict)
			}
		} else if input.ExpectedVersion != 0 || input.TargetResourceID != "" {
			return fmt.Errorf("%w: selected upgrade target does not match the package", domain.ErrConflict)
		}
		kind, id, version := "expert", "", int64(0)
		if input.Expert != nil {
			var item domain.Expert
			var err error
			if headQuery.Error == nil {
				if head.Kind != "expert" || input.ExpectedVersion <= 0 {
					return fmt.Errorf("%w: package upgrade needs the current resource version", domain.ErrConflict)
				}
				item, err = scoped.UpdateExpert(ctx, ownerID, head.ResourceID, *input.Expert, input.ExpectedVersion)
			} else {
				item, err = scoped.CreateExpert(ctx, ownerID, *input.Expert)
			}
			if err != nil {
				return err
			}
			result.Expert = &item
			id, version = item.ID, item.Version
		} else {
			var item domain.ExpertTeam
			var err error
			if headQuery.Error == nil {
				if head.Kind != "expert_team" || input.ExpectedVersion <= 0 {
					return fmt.Errorf("%w: package upgrade needs the current resource version", domain.ErrConflict)
				}
				item, err = scoped.UpdateExpertTeam(ctx, ownerID, head.ResourceID, *input.Team, input.ExpectedVersion)
			} else {
				item, err = scoped.CreateExpertTeam(ctx, ownerID, *input.Team)
			}
			if err != nil {
				return err
			}
			result.Team = &item
			id, version = item.ID, item.Version
			kind = "expert_team"
		}
		return tx.Exec("INSERT INTO expert_package_imports(owner_user_id,package_id,package_version,content_sha256,kind,resource_id,resource_version,archive) VALUES(?,?,?,?,?,?,?,?)", ownerID, input.PackageID, input.PackageVersion, input.SHA256, kind, id, version, input.Archive).Error
	})
	return result, err
}

// ImportedExpertPackage returns transport bytes only for the exact resource
// revision. Resource visibility is checked independently of any object key.
func (repository *Repository) ImportedExpertPackage(ctx context.Context, ownerID, kind, id string, version int64) ([]byte, error) {
	if kind == "expert" {
		if _, err := repository.GetExpert(ctx, ownerID, id); err != nil {
			return nil, err
		}
	} else if kind == "expert_team" {
		if _, err := repository.GetExpertTeam(ctx, ownerID, id); err != nil {
			return nil, err
		}
	} else {
		return nil, domain.ErrInvalid
	}
	var row struct{ Archive []byte }
	err := repository.db.WithContext(ctx).Table("expert_package_imports").Select("archive").Where("kind=? AND resource_id=? AND resource_version=?", kind, id, version).Take(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return row.Archive, err
}
