package gormrepo

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
	"database/sql/driver"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sync"
	"time"
)

func renewableFeishuQuery(db *gorm.DB) *gorm.DB {
	return db.Table("connector_authorizations auth").
		Joins("JOIN connector_installations installation ON installation.id=auth.installation_id AND installation.owner_user_id=auth.owner_user_id AND installation.authorization_id=auth.id").
		Joins("JOIN users owner ON owner.id=auth.owner_user_id").
		Joins("JOIN connector_revisions revision ON revision.id=installation.active_revision_id AND revision.package_source=installation.package_source").
		Joins("JOIN connector_package_publications publication ON publication.package_source=installation.package_source").
		Where("owner.disabled_at IS NULL AND installation.state='active' AND installation.package_source='feishu' AND publication.state='available' AND revision.mode='cli' AND revision.runtime_policy->'cli'->>'authentication_driver'='feishu' AND COALESCE((revision.runtime_policy->>'legacy_projection')::boolean,false)=false AND auth.state IN ('active','expired') AND auth.credential_format IN ('json','access_token')")
}

func (r *Repository) ListDueFeishuAuthorizations(ctx context.Context, owner string, before time.Time) ([]domain.ConnectorAuthorization, error) {
	q := renewableFeishuQuery(r.db.WithContext(ctx)).Select("auth.*").Where("auth.expires_at IS NOT NULL AND auth.expires_at<=?", before)
	if owner != "" {
		q = q.Where("auth.owner_user_id=?", owner)
	}
	var rows []connectorAuthorizationRecord
	// Newly refreshed grants leave the due set; failures use an application cooldown.
	if err := q.Order("auth.updated_at,auth.id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.ConnectorAuthorization, 0, len(rows))
	for _, row := range rows {
		items = append(items, connectorAuthorizationDomain(row))
	}
	return items, nil
}
func (r *Repository) GetRenewableFeishuAuthorization(ctx context.Context, owner, installation, id string) (domain.ConnectorAuthorization, error) {
	var row connectorAuthorizationRecord
	err := renewableFeishuQuery(r.db.WithContext(ctx)).Select("auth.*").Where("auth.id=? AND auth.installation_id=? AND auth.owner_user_id=?", id, installation, owner).Take(&row).Error
	return connectorAuthorizationDomain(row), mapNotFound(err)
}

// Pin the advisory lock to one connection; provider HTTP never holds a data
// transaction. Manual refresh and background renewal share this lock.
func (r *Repository) LockConnectorAuthorizationRefresh(ctx context.Context, owner, installation, id string) (func(), bool, error) {
	sqlDB, err := r.db.DB()
	if err != nil {
		return nil, false, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, false, err
	}
	key := "connector-authorization-refresh:" + owner + ":" + installation + ":" + id
	var locked bool
	if err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", key).Scan(&locked); err != nil || !locked {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
		return nil, false, err
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			var unlocked bool
			if conn.QueryRowContext(clean, "SELECT pg_advisory_unlock(hashtextextended($1,0))", key).Scan(&unlocked) != nil || !unlocked {
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
			_ = conn.Close()
		})
	}
	return release, true, nil
}

func (r *Repository) SaveRenewedFeishuAuthorization(ctx context.Context, grant domain.ConnectorAuthorization, version int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var installation connectorInstallationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=? AND authorization_id=? AND state='active'", grant.InstallationID, grant.OwnerID, grant.ID).Take(&installation).Error; err != nil {
			return mapNotFound(err)
		}
		if _, err := New(tx, nil).GetRenewableFeishuAuthorization(ctx, grant.OwnerID, grant.InstallationID, grant.ID); err != nil {
			return err
		}
		_, err := New(tx, nil).RefreshConnectorAuthorization(ctx, grant, version, domain.ConnectorAuditRecord{OwnerID: grant.OwnerID, InstallationID: grant.InstallationID, Operation: "refresh_authorization", IdentityRef: grant.IdentityRef, Outcome: "succeeded", CreatedAt: time.Now().UTC()})
		return err
	})
}

func (r *Repository) PendingChannelOwner(ctx context.Context) (string, error) {
	var row struct{ OwnerUserID string }
	err := r.db.WithContext(ctx).Table("message_channel_inbox inbox").Select("channel.owner_user_id").Joins("JOIN workflow_message_channels channel ON channel.id=inbox.channel_id").Where("inbox.state='received' AND channel.enabled AND channel.deleted_at IS NULL AND channel.config_version=inbox.config_version").Order("inbox.received_at,inbox.id").Limit(1).Scan(&row).Error
	return row.OwnerUserID, err
}
