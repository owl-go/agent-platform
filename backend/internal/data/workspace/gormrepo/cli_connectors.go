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

func (repository *Repository) ListCLIConnectorDefinitions(ctx context.Context, includeUnpublished bool) ([]cliconnector.Definition, error) {
	query := repository.db.WithContext(ctx).Order("name, id")
	if !includeUnpublished {
		query = query.Where("state = ?", cliconnector.StateAvailable)
	}
	var rows []cliConnectorDefinitionRecord
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list CLI Connector Definitions: %w", err)
	}
	items := make([]cliconnector.Definition, 0, len(rows))
	for _, row := range rows {
		item, err := cliDefinitionDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) CreateCLIConnectorDefinition(ctx context.Context, administratorID string, input cliconnector.Definition) (cliconnector.Definition, error) {
	if err := input.Validate(); err != nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	capabilities, _ := json.Marshal(input.Capabilities)
	architectures, _ := json.Marshal(input.SupportedArchitectures)
	recommendedSkills, _ := json.Marshal(input.RecommendedSkillIDs)
	row := cliConnectorDefinitionRecord{ID: uuid.NewString(), Name: input.Name, NPMPackage: input.Package, NPMVersion: input.Version, NPMIntegrity: input.Integrity, Executable: input.Executable, AuthenticationDriver: input.AuthenticationDriver, Capabilities: capabilities, SupportedArchitectures: architectures, RecommendedSkillIDs: recommendedSkills, State: string(cliconnector.StateDraft), CreatedByUserID: administratorID, Version: 1}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return cliconnector.Definition{}, fmt.Errorf("create CLI Connector Definition: %w", err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) UpdateCLIConnectorDefinition(ctx context.Context, id string, input cliconnector.Definition, expectedVersion int64) (cliconnector.Definition, error) {
	if err := input.Validate(); err != nil {
		return cliconnector.Definition{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	capabilities, _ := json.Marshal(input.Capabilities)
	architectures, _ := json.Marshal(input.SupportedArchitectures)
	recommendedSkills, _ := json.Marshal(input.RecommendedSkillIDs)
	result := repository.db.WithContext(ctx).Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ? AND state IN ?", id, expectedVersion, []string{"draft", "failed"}).Updates(map[string]any{"name": input.Name, "npm_package": input.Package, "npm_version": input.Version, "npm_integrity": input.Integrity, "executable": input.Executable, "authentication_driver": input.AuthenticationDriver, "capabilities": capabilities, "supported_architectures": architectures, "recommended_skill_ids": recommendedSkills, "state": "draft", "failure_reason": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return cliconnector.Definition{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.Definition{}, domain.ErrConflict
	}
	var row cliConnectorDefinitionRecord
	if err := repository.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		return cliconnector.Definition{}, mapNotFound(err)
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) DeleteCLIConnectorDefinition(ctx context.Context, id string) error {
	result := repository.db.WithContext(ctx).Where("id = ? AND state IN ?", id, []string{"draft", "failed"}).Delete(&cliConnectorDefinitionRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) EnableCLIConnector(ctx context.Context, ownerID, definitionID, actionURL string, expiry time.Time) (cliconnector.Enablement, error) {
	row := cliConnectorEnablementRecord{ID: uuid.NewString(), OwnerID: ownerID, DefinitionID: definitionID, State: "waiting_for_user", ActionURL: &actionURL, ActionExpiresAt: &expiry, Version: 1}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND state = 'available'", definitionID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("%w: CLI Connector is unavailable", domain.ErrInvalid)
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_user_id"}, {Name: "definition_id"}}, DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Where("owner_user_id = ? AND definition_id = ?", ownerID, definitionID).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Enablement{}, err
	}
	return cliEnablementDomain(row), nil
}

func (repository *Repository) ListCLIConnectorEnablements(ctx context.Context, ownerID string) ([]cliconnector.Enablement, error) {
	var rows []cliConnectorEnablementRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ?", ownerID).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]cliconnector.Enablement, 0, len(rows))
	for _, row := range rows {
		items = append(items, cliEnablementDomain(row))
	}
	return items, nil
}

func cliDefinitionDomain(row cliConnectorDefinitionRecord) (cliconnector.Definition, error) {
	var capabilities []cliconnector.Capability
	if err := json.Unmarshal(row.Capabilities, &capabilities); err != nil {
		return cliconnector.Definition{}, err
	}
	var architectures, recommendedSkills []string
	if err := json.Unmarshal(row.SupportedArchitectures, &architectures); err != nil {
		return cliconnector.Definition{}, err
	}
	if err := json.Unmarshal(row.RecommendedSkillIDs, &recommendedSkills); err != nil {
		return cliconnector.Definition{}, err
	}
	item := cliconnector.Definition{ID: row.ID, Name: row.Name, Package: row.NPMPackage, Version: row.NPMVersion, Integrity: row.NPMIntegrity, Executable: row.Executable, AuthenticationDriver: row.AuthenticationDriver, State: cliconnector.State(row.State), Capabilities: capabilities, SupportedArchitectures: architectures, RecommendedSkillIDs: recommendedSkills, VersionNumber: row.Version, CreatedByUserID: row.CreatedByUserID}
	if row.BundleSHA256 != nil {
		item.BundleSHA256 = *row.BundleSHA256
	}
	if row.FailureReason != nil {
		item.FailureReason = *row.FailureReason
	}
	return item, nil
}
func cliEnablementDomain(row cliConnectorEnablementRecord) cliconnector.Enablement {
	item := cliconnector.Enablement{ID: row.ID, OwnerID: row.OwnerID, DefinitionID: row.DefinitionID, State: row.State, ActionExpiresAt: row.ActionExpiresAt, Version: row.Version}
	if row.ActionURL != nil {
		item.ActionURL = *row.ActionURL
	}
	return item
}

func (repository *Repository) ListCommandApprovals(ctx context.Context, ownerID string, now time.Time) ([]domain.CommandApproval, error) {
	if err := repository.db.WithContext(ctx).Model(&cliCommandApprovalRecord{}).Where("owner_user_id = ? AND state = 'pending' AND expires_at <= ?", ownerID, now).Updates(map[string]any{"state": "expired", "version": gorm.Expr("version + 1")}).Error; err != nil {
		return nil, err
	}
	var rows []cliCommandApprovalRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND state IN ?", ownerID, []string{"pending", "approved"}).Order("created_at").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]domain.CommandApproval, 0, len(rows))
	for _, row := range rows {
		items = append(items, commandApprovalDomain(row))
	}
	return items, nil
}

func (repository *Repository) DecideCommandApproval(ctx context.Context, ownerID, approvalID string, decision domain.ApprovalState, identity domain.ExecutionIdentity, expectedVersion int64, now time.Time) (domain.CommandApproval, error) {
	var row cliCommandApprovalRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND id = ?", ownerID, approvalID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		approval := commandApprovalDomain(row)
		if row.Version != expectedVersion {
			return domain.ErrConflict
		}
		if err := approval.Decide(ownerID, decision, identity, now); err != nil {
			return err
		}
		updates := map[string]any{"state": string(approval.State), "decided_at": approval.DecidedAt, "version": gorm.Expr("version + 1")}
		if approval.Identity != "" {
			updates["identity"] = string(approval.Identity)
		}
		result := tx.Model(&cliCommandApprovalRecord{}).Where("id = ? AND version = ?", row.ID, expectedVersion).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.Where("id = ?", row.ID).Take(&row).Error
	})
	if err != nil {
		return domain.CommandApproval{}, err
	}
	return commandApprovalDomain(row), nil
}

func commandApprovalDomain(row cliCommandApprovalRecord) domain.CommandApproval {
	value := domain.CommandApproval{ID: row.ID, OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID, CommandDigest: row.CommandDigest, ConnectorName: row.ConnectorName, Operation: row.Operation, Target: row.Target, RedactedArguments: row.RedactedArguments, State: domain.ApprovalState(row.State), ExpiresAt: row.ExpiresAt, DecidedAt: row.DecidedAt, ConsumedAt: row.ConsumedAt, Version: row.Version}
	if row.Identity != nil {
		value.Identity = domain.ExecutionIdentity(*row.Identity)
	}
	return value
}
