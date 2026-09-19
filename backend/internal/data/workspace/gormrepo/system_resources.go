package gormrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/systemskills"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnsureSystemSkills installs the immutable platform Skills before the API or
// Worker becomes ready. The rows remain owned by the bootstrap Administrator,
// while system_managed prevents ordinary catalog mutation paths from changing them.
func (repository *Repository) EnsureSystemSkills(ctx context.Context, objects objectstore.Provider) error {
	if objects == nil {
		return fmt.Errorf("system Skill Object Store is required")
	}
	var administrator struct {
		ID string `gorm:"column:id"`
	}
	if err := repository.db.WithContext(ctx).Table("users").Select("id").Where("administrator = true").Take(&administrator).Error; err != nil {
		return fmt.Errorf("find bootstrap Administrator for system Skills: %w", err)
	}
	for _, definition := range systemskills.Definitions() {
		archive, digest, err := systemskills.Archive(definition.Key)
		if err != nil {
			return err
		}
		objectKey := "skills/system/" + strings.TrimPrefix(definition.Key, "system.") + ".zip"
		stored, statErr := objects.Stat(ctx, objectKey)
		if statErr != nil {
			if !errors.Is(statErr, objectstore.ErrNotFound) {
				return fmt.Errorf("check system Skill %s: %w", definition.Key, statErr)
			}
			stored, err = objects.Put(ctx, objectKey, bytes.NewReader(archive), objectstore.PutOptions{Size: int64(len(archive)), SHA256: digest, ContentType: "application/zip", Metadata: map[string]string{"system-key": definition.Key}})
			if err != nil {
				return fmt.Errorf("store system Skill %s: %w", definition.Key, err)
			}
		}
		if stored.SHA256 != digest {
			return fmt.Errorf("system Skill %s object checksum mismatch", definition.Key)
		}
		if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var row skillRecord
			err := tx.Where("system_key = ?", definition.Key).Take(&row).Error
			now := time.Now().UTC()
			if err == nil {
				if !row.SystemManaged || row.OwnerID != administrator.ID {
					return fmt.Errorf("system Skill %s has invalid ownership", definition.Key)
				}
				if row.SHA256 == digest && row.ObjectKey == objectKey && row.Name == definition.Name {
					return nil
				}
				return tx.Model(&skillRecord{}).Where("id = ?", row.ID).Updates(map[string]any{"owner_user_id": administrator.ID, "name": definition.Name, "name_normalized": normalizeResourceName(definition.Name), "source": "upload", "object_key": objectKey, "sha256": digest, "system_managed": true, "updated_at": now, "version": gorm.Expr("version + 1")}).Error
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			return tx.Create(&skillRecord{ID: uuid.NewString(), OwnerID: administrator.ID, SystemKey: definition.Key, SystemManaged: true, Name: definition.Name, NameNormalized: normalizeResourceName(definition.Name), Icon: "sparkles", Source: "upload", ObjectKey: objectKey, SHA256: digest, CreatedAt: now, UpdatedAt: now, Version: 1}).Error
		}); err != nil {
			return fmt.Errorf("persist system Skill %s: %w", definition.Key, err)
		}
	}
	return nil
}
