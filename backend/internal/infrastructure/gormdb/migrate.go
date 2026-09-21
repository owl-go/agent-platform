package gormdb

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const migrationLockID int64 = 770091734641

type migrationRecord struct {
	Name      string    `gorm:"column:name;primaryKey"`
	Checksum  string    `gorm:"column:checksum"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

func (migrationRecord) TableName() string { return "schema_migrations" }

func Migrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(?)`, migrationLockID).Error; err != nil {
			return fmt.Errorf("lock database migrations: %w", err)
		}
		if err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				name text PRIMARY KEY,
				checksum text NOT NULL,
				applied_at timestamptz NOT NULL DEFAULT now()
			)`).Error; err != nil {
			return fmt.Errorf("create migration ledger: %w", err)
		}

		entries, err := fs.ReadDir(migrationFiles, "migrations")
		if err != nil {
			return fmt.Errorf("list database migrations: %w", err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			contents, err := migrationFiles.ReadFile("migrations/" + entry.Name())
			if err != nil {
				return fmt.Errorf("read migration %s: %w", entry.Name(), err)
			}
			sum := sha256.Sum256(contents)
			checksum := hex.EncodeToString(sum[:])

			var applied migrationRecord
			err = tx.Where("name = ?", entry.Name()).Take(&applied).Error
			switch {
			case err == nil && applied.Checksum != checksum:
				return fmt.Errorf("database migration %s checksum changed", entry.Name())
			case err == nil:
				continue
			case err != gorm.ErrRecordNotFound:
				return fmt.Errorf("read migration ledger for %s: %w", entry.Name(), err)
			}

			if err := tx.Exec(string(contents)).Error; err != nil {
				return fmt.Errorf("apply database migration %s: %w", entry.Name(), err)
			}
			if err := tx.Create(&migrationRecord{Name: entry.Name(), Checksum: checksum}).Error; err != nil {
				return fmt.Errorf("record database migration %s: %w", entry.Name(), err)
			}
		}
		return nil
	})
}
