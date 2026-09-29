package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/account/application"
	"agent-platform/backend/internal/biz/account/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

var _ application.Repository = (*Repository)(nil)

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

type userModel struct {
	ID                     string     `gorm:"column:id"`
	OIDCSubject            string     `gorm:"column:oidc_subject"`
	Username               string     `gorm:"column:username"`
	Email                  string     `gorm:"column:email"`
	DisplayName            string     `gorm:"column:display_name"`
	Administrator          bool       `gorm:"column:administrator"`
	BootstrapAdministrator bool       `gorm:"column:bootstrap_administrator"`
	ResourcePublisher      bool       `gorm:"column:resource_publisher"`
	DisabledAt             *time.Time `gorm:"column:disabled_at"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
	Version                int64      `gorm:"column:version"`
}

func (userModel) TableName() string { return "users" }

type identityGroupModel struct {
	ID                         string     `gorm:"column:id"`
	ExternalID                 string     `gorm:"column:external_id"`
	Name                       string     `gorm:"column:name"`
	Path                       string     `gorm:"column:path"`
	Department                 bool       `gorm:"column:department"`
	DailyCreditLimitHundredths *int64     `gorm:"column:daily_credit_limit_hundredths"`
	LastSyncedAt               time.Time  `gorm:"column:last_synced_at"`
	DeletedAt                  *time.Time `gorm:"column:deleted_at"`
	CreatedAt                  time.Time  `gorm:"column:created_at"`
	UpdatedAt                  time.Time  `gorm:"column:updated_at"`
	Version                    int64      `gorm:"column:version"`
	MemberCount                int64      `gorm:"column:member_count;->"`
}

func (identityGroupModel) TableName() string { return "identity_groups" }

type governanceAuditModel struct {
	ID          int64     `gorm:"column:id"`
	ActorUserID string    `gorm:"column:actor_user_id"`
	Action      string    `gorm:"column:action"`
	TargetType  string    `gorm:"column:target_type"`
	TargetID    string    `gorm:"column:target_id"`
	Reason      string    `gorm:"column:reason"`
	Detail      []byte    `gorm:"column:detail;type:jsonb"`
	OccurredAt  time.Time `gorm:"column:occurred_at"`
}

func (governanceAuditModel) TableName() string { return "governance_audit_events" }

func (repository *Repository) FindPrincipal(ctx context.Context, subject string) (domain.Principal, error) {
	var model userModel
	if err := repository.db.WithContext(ctx).Where("oidc_subject = ?", subject).Take(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Principal{}, domain.ErrNotFound
		}
		return domain.Principal{}, fmt.Errorf("find User identity: %w", err)
	}
	users := []domain.User{toDomain(model)}
	if err := repository.loadUserGroups(ctx, users); err != nil {
		return domain.Principal{}, err
	}
	return domain.Principal{
		UserID: model.ID, Username: model.Username, Email: model.Email, DisplayName: model.DisplayName,
		Administrator: model.Administrator, BootstrapAdministrator: model.BootstrapAdministrator,
		ResourcePublisher: model.ResourcePublisher, Groups: users[0].Groups, Disabled: model.DisabledAt != nil,
	}, nil
}

func (repository *Repository) EnsureAdministrator(ctx context.Context, user domain.User) (domain.User, error) {
	var row userModel
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bootstrap_administrator = true").Take(&row).Error; err == nil {
			if row.OIDCSubject != user.OIDCSubject {
				return fmt.Errorf("a different bootstrap Administrator already exists")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		row = newUserModel(user)
		row.Administrator, row.BootstrapAdministrator = true, true
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO personal_settings (user_id) VALUES (?)", row.ID).Error
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("ensure bootstrap Administrator: %w", err)
	}
	return toDomain(row), nil
}

func (repository *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	var models []userModel
	if err := repository.db.WithContext(ctx).Order("administrator DESC, created_at, id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list Users: %w", err)
	}
	users := make([]domain.User, 0, len(models))
	for _, model := range models {
		users = append(users, toDomain(model))
	}
	if err := repository.loadUserGroups(ctx, users); err != nil {
		return nil, err
	}
	return users, nil
}

func (repository *Repository) CreateUser(ctx context.Context, user domain.User) (domain.User, error) {
	model := newUserModel(user)
	model.Administrator = false
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return fmt.Errorf("create User: %w", err)
		}
		if err := tx.Exec("INSERT INTO personal_settings (user_id) VALUES (?)", model.ID).Error; err != nil {
			return fmt.Errorf("create Personal Settings: %w", err)
		}
		return nil
	})
	return toDomain(model), err
}

func newUserModel(user domain.User) userModel {
	return userModel{
		ID: uuid.NewString(), OIDCSubject: user.OIDCSubject, Username: user.Username,
		Email: user.Email, DisplayName: user.DisplayName, Administrator: user.Administrator, Version: 1,
	}
}

func (repository *Repository) SetEnabled(ctx context.Context, actorUserID, userID string, enabled bool, expectedVersion int64, reason string) (domain.User, error) {
	var disabledAt any = gorm.Expr("now()")
	if enabled {
		disabledAt = nil
	}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&userModel{}).
			Where("id = ? AND version = ? AND administrator = false", userID, expectedVersion).
			Updates(map[string]any{"disabled_at": disabledAt, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		enabledMetric := int64(0)
		if enabled {
			enabledMetric = 1
		}
		detail, _ := json.Marshal(map[string]int64{"enabled": enabledMetric})
		return tx.Create(&governanceAuditModel{ActorUserID: actorUserID, Action: "user.enabled.updated", TargetType: "user", TargetID: userID, Reason: reason, Detail: detail}).Error
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("update User status: %w", err)
	}
	var model userModel
	if err := repository.db.WithContext(ctx).Where("id = ?", userID).Take(&model).Error; err != nil {
		return domain.User{}, fmt.Errorf("reload User: %w", err)
	}
	return toDomain(model), nil
}

func (repository *Repository) SetRoles(ctx context.Context, actorUserID, userID string, administrator, resourcePublisher bool, expectedVersion int64, reason string) (domain.User, error) {
	var updated userModel
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).Take(&target).Error; err != nil {
			return mapNotFound(err)
		}
		if target.Version != expectedVersion {
			return domain.ErrConflict
		}
		if target.BootstrapAdministrator && !administrator {
			return fmt.Errorf("%w: bootstrap Administrator cannot be demoted", domain.ErrConflict)
		}
		if target.Administrator && !administrator {
			var administrators int64
			if err := tx.Model(&userModel{}).Where("administrator = true AND disabled_at IS NULL").Count(&administrators).Error; err != nil {
				return err
			}
			if administrators <= 1 {
				return fmt.Errorf("%w: at least one enabled Administrator is required", domain.ErrConflict)
			}
		}
		if target.Administrator == administrator && target.ResourcePublisher == resourcePublisher {
			updated = target
			return nil
		}
		target.Administrator, target.ResourcePublisher = administrator, resourcePublisher
		target.Version++
		target.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&target).Error; err != nil {
			return err
		}
		administratorMetric, publisherMetric := int64(0), int64(0)
		if administrator {
			administratorMetric = 1
		}
		if resourcePublisher {
			publisherMetric = 1
		}
		detail, _ := json.Marshal(map[string]int64{"administrator": administratorMetric, "resource_publisher": publisherMetric})
		if err := tx.Create(&governanceAuditModel{ActorUserID: actorUserID, Action: "user.roles.updated", TargetType: "user", TargetID: userID, Reason: reason, Detail: detail}).Error; err != nil {
			return err
		}
		updated = target
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return toDomain(updated), nil
}

func (repository *Repository) ReplaceIdentityGroups(ctx context.Context, actorUserID string, snapshots []domain.IdentityGroupSnapshot, now time.Time) ([]domain.IdentityGroup, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var users []userModel
		if err := tx.Find(&users).Error; err != nil {
			return err
		}
		usersBySubject := make(map[string]string, len(users))
		for _, user := range users {
			usersBySubject[user.OIDCSubject] = user.ID
		}
		externalIDs := make([]string, 0, len(snapshots))
		membershipCount := int64(0)
		for _, snapshot := range snapshots {
			externalIDs = append(externalIDs, snapshot.ExternalID)
			var row identityGroupModel
			err := tx.Where("external_id = ?", snapshot.ExternalID).Take(&row).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				row = identityGroupModel{ID: uuid.NewString(), ExternalID: snapshot.ExternalID, Name: strings.TrimSpace(snapshot.Name), Path: strings.TrimSpace(snapshot.Path), Department: snapshot.Department, LastSyncedAt: now, Version: 1}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			case err != nil:
				return err
			default:
				updates := map[string]any{"name": strings.TrimSpace(snapshot.Name), "path": strings.TrimSpace(snapshot.Path), "department": snapshot.Department, "last_synced_at": now, "deleted_at": nil, "updated_at": now, "version": gorm.Expr("version + 1")}
				if !snapshot.Department {
					updates["daily_credit_limit_hundredths"] = nil
				}
				if err := tx.Model(&identityGroupModel{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
					return err
				}
			}
			if err := tx.Exec("DELETE FROM identity_group_memberships WHERE group_id = ?", row.ID).Error; err != nil {
				return err
			}
			seenMembers := map[string]struct{}{}
			for _, subject := range snapshot.MemberSubjects {
				userID, exists := usersBySubject[subject]
				if !exists {
					continue
				}
				if _, duplicate := seenMembers[userID]; duplicate {
					continue
				}
				seenMembers[userID] = struct{}{}
				if err := tx.Exec("INSERT INTO identity_group_memberships(group_id,user_id,last_synced_at) VALUES(?,?,?)", row.ID, userID, now).Error; err != nil {
					return err
				}
				membershipCount++
			}
		}
		inactive := tx.Model(&identityGroupModel{}).Where("deleted_at IS NULL")
		if len(externalIDs) > 0 {
			inactive = inactive.Where("external_id NOT IN ?", externalIDs)
		}
		if err := inactive.Updates(map[string]any{"deleted_at": now, "updated_at": now, "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]int64{"groups": int64(len(snapshots)), "memberships": membershipCount})
		return tx.Create(&governanceAuditModel{ActorUserID: actorUserID, Action: "identity.groups.synced", TargetType: "identity_source", TargetID: "keycloak", Reason: "read-only identity source synchronization", Detail: detail}).Error
	})
	if err != nil {
		return nil, fmt.Errorf("replace identity groups: %w", err)
	}
	return repository.ListIdentityGroups(ctx)
}

func (repository *Repository) ListIdentityGroups(ctx context.Context) ([]domain.IdentityGroup, error) {
	var rows []identityGroupModel
	selectClause := `identity_groups.*, (SELECT COUNT(*) FROM identity_group_memberships membership WHERE membership.group_id = identity_groups.id) AS member_count`
	if err := repository.db.WithContext(ctx).Table("identity_groups").Select(selectClause).Where("deleted_at IS NULL").Order("department DESC, path, id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list identity groups: %w", err)
	}
	items := make([]domain.IdentityGroup, 0, len(rows))
	for _, row := range rows {
		items = append(items, identityGroupDomain(row))
	}
	return items, nil
}

func (repository *Repository) UpdateIdentityGroupBudget(ctx context.Context, actorUserID, groupID string, dailyLimit *int64, expectedVersion int64, reason string) (domain.IdentityGroup, error) {
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&identityGroupModel{}).Where("id = ? AND deleted_at IS NULL AND department = true AND version = ?", groupID, expectedVersion).Updates(map[string]any{"daily_credit_limit_hundredths": dailyLimit, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		metrics := map[string]int64{"limited": 0}
		if dailyLimit != nil {
			metrics["limited"] = 1
			metrics["daily_credit_limit_hundredths"] = *dailyLimit
		}
		detail, _ := json.Marshal(metrics)
		return tx.Create(&governanceAuditModel{ActorUserID: actorUserID, Action: "identity_group.budget.updated", TargetType: "identity_group", TargetID: groupID, Reason: reason, Detail: detail}).Error
	})
	if err != nil {
		return domain.IdentityGroup{}, fmt.Errorf("update identity group budget: %w", err)
	}
	var row identityGroupModel
	selectClause := `identity_groups.*, (SELECT COUNT(*) FROM identity_group_memberships membership WHERE membership.group_id = identity_groups.id) AS member_count`
	if err := repository.db.WithContext(ctx).Table("identity_groups").Select(selectClause).Where("id = ?", groupID).Take(&row).Error; err != nil {
		return domain.IdentityGroup{}, err
	}
	return identityGroupDomain(row), nil
}

func (repository *Repository) ListGovernanceAuditEvents(ctx context.Context, limit int) ([]domain.GovernanceAuditEvent, error) {
	var rows []governanceAuditModel
	if err := repository.db.WithContext(ctx).Order("occurred_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list governance audit events: %w", err)
	}
	items := make([]domain.GovernanceAuditEvent, 0, len(rows))
	for _, row := range rows {
		detail := map[string]int64{}
		_ = json.Unmarshal(row.Detail, &detail)
		items = append(items, domain.GovernanceAuditEvent{ID: row.ID, ActorUserID: row.ActorUserID, Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID, Reason: row.Reason, Detail: detail, OccurredAt: row.OccurredAt})
	}
	return items, nil
}

func (repository *Repository) loadUserGroups(ctx context.Context, users []domain.User) error {
	if len(users) == 0 {
		return nil
	}
	ids := make([]string, 0, len(users))
	byID := make(map[string]int, len(users))
	for index, user := range users {
		ids = append(ids, user.ID)
		byID[user.ID] = index
	}
	var rows []struct {
		UserID string `gorm:"column:user_id"`
		identityGroupModel
	}
	if err := repository.db.WithContext(ctx).Table("identity_group_memberships membership").Select("membership.user_id, identity_groups.*").Joins("JOIN identity_groups ON identity_groups.id = membership.group_id AND identity_groups.deleted_at IS NULL").Where("membership.user_id IN ?", ids).Order("identity_groups.path").Scan(&rows).Error; err != nil {
		return fmt.Errorf("load User identity groups: %w", err)
	}
	for _, row := range rows {
		index := byID[row.UserID]
		users[index].Groups = append(users[index].Groups, identityGroupDomain(row.identityGroupModel))
	}
	return nil
}

func identityGroupDomain(row identityGroupModel) domain.IdentityGroup {
	return domain.IdentityGroup{ID: row.ID, ExternalID: row.ExternalID, Name: row.Name, Path: row.Path, Department: row.Department, DailyCreditLimitHundredths: row.DailyCreditLimitHundredths, MemberCount: row.MemberCount, LastSyncedAt: row.LastSyncedAt, DeletedAt: row.DeletedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Version: row.Version}
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func toDomain(model userModel) domain.User {
	return domain.User{
		ID: model.ID, OIDCSubject: model.OIDCSubject, Username: model.Username, Email: model.Email,
		DisplayName: model.DisplayName, Administrator: model.Administrator, BootstrapAdministrator: model.BootstrapAdministrator,
		ResourcePublisher: model.ResourcePublisher, Enabled: model.DisabledAt == nil,
		CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt, Version: model.Version,
	}
}
