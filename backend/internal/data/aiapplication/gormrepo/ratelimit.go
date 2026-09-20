package gormrepo

import (
	"context"
	"time"
)

func (r *Repository) ConsumeExternalRate(ctx context.Context, scope, key string, now time.Time, limit int) (bool, error) {
	bucket := now.UTC().Truncate(time.Minute)
	// Keep the bounded bucket table from retaining stale visitor and IP keys.
	if err := r.db.WithContext(ctx).Where("bucket_start < ?", bucket.Add(-2*time.Hour)).Delete(&externalRateLimitRecord{}).Error; err != nil {
		return false, err
	}
	var allowed bool
	err := r.db.WithContext(ctx).Raw(`
		WITH consumed AS (
			INSERT INTO external_rate_limit_buckets (scope, scope_key, bucket_start, calls)
			VALUES (?, ?, ?, 1)
			ON CONFLICT (scope, scope_key, bucket_start) DO UPDATE
			SET calls = external_rate_limit_buckets.calls + 1
			WHERE external_rate_limit_buckets.calls < ?
			RETURNING scope
		)
		SELECT EXISTS (SELECT 1 FROM consumed)`, scope, key, bucket, limit).Row().Scan(&allowed)
	return allowed, err
}

type externalRateLimitRecord struct {
	Scope       string    `gorm:"column:scope;primaryKey"`
	ScopeKey    string    `gorm:"column:scope_key;primaryKey"`
	BucketStart time.Time `gorm:"column:bucket_start;primaryKey"`
	Calls       int       `gorm:"column:calls"`
}

func (externalRateLimitRecord) TableName() string { return "external_rate_limit_buckets" }
