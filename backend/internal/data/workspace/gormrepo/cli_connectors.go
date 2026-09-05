package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
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
	var conformances []struct {
		DefinitionID  string `gorm:"column:definition_id"`
		RuntimeDigest string `gorm:"column:runtime_repo_digest"`
	}
	if err := repository.db.WithContext(ctx).Table("cli_connector_conformance").Select("definition_id, runtime_repo_digest").Where("passed = ?", true).Order("runtime_repo_digest").Scan(&conformances).Error; err != nil {
		return nil, fmt.Errorf("list CLI Connector conformance: %w", err)
	}
	runtimeDigests := make(map[string][]string)
	for _, row := range conformances {
		runtimeDigests[row.DefinitionID] = append(runtimeDigests[row.DefinitionID], row.RuntimeDigest)
	}
	items := make([]cliconnector.Definition, 0, len(rows))
	for _, row := range rows {
		item, err := cliDefinitionDomain(row)
		if err != nil {
			return nil, err
		}
		item.RuntimeDigests = runtimeDigests[item.ID]
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

func (repository *Repository) PublishCLIConnectorDefinition(ctx context.Context, id string, expectedVersion int64) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Version != expectedVersion || (row.State != string(cliconnector.StateDraft) && row.State != string(cliconnector.StateFailed)) {
			return domain.ErrConflict
		}
		definition, err := cliDefinitionDomain(row)
		if err != nil {
			return err
		}
		if err := definition.Validate(); err != nil {
			return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
		}
		result := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(map[string]any{"state": string(cliconnector.StateBuilding), "failure_reason": nil, "bundle_object_key": nil, "bundle_sha256": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		return tx.Where("id = ?", id).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Definition{}, err
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) DisableCLIConnectorDefinition(ctx context.Context, id string, expectedVersion int64) (cliconnector.Definition, error) {
	var row cliConnectorDefinitionRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&cliConnectorDefinitionRecord{}).Where("id = ? AND version = ? AND state = ?", id, expectedVersion, cliconnector.StateAvailable).Updates(map[string]any{"state": string(cliconnector.StateDisabled), "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrConflict
		}
		if err := tx.Model(&cliConnectorEnablementRecord{}).Where("definition_id = ? AND state <> 'disabled'", id).Updates(map[string]any{"state": "disabled", "action_url": nil, "action_expires_at": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Take(&row).Error
	})
	if err != nil {
		return cliconnector.Definition{}, err
	}
	return cliDefinitionDomain(row)
}

func (repository *Repository) EnableCLIConnector(ctx context.Context, ownerID, definitionID, actionURL string, expiry time.Time) (cliconnector.Enablement, error) {
	var row cliConnectorEnablementRecord
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var definition cliConnectorDefinitionRecord
		if err := tx.Where("id = ? AND state = 'available'", definitionID).Take(&definition).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: CLI Connector is unavailable", domain.ErrInvalid)
		} else if err != nil {
			return err
		}
		row = newCLIConnectorEnablement(ownerID, definitionID, definition.AuthenticationDriver, actionURL, expiry)
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

func newCLIConnectorEnablement(ownerID, definitionID, authenticationDriver, actionURL string, expiry time.Time) cliConnectorEnablementRecord {
	row := cliConnectorEnablementRecord{ID: uuid.NewString(), OwnerID: ownerID, DefinitionID: definitionID, State: "enabled", Version: 1}
	if authenticationDriver != "none" {
		row.State = "waiting_for_user"
		row.ActionURL = &actionURL
		row.ActionExpiresAt = &expiry
	}
	return row
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
	if row.BundleObjectKey != nil {
		item.BundleObjectKey = *row.BundleObjectKey
	}
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
	if err := repository.db.WithContext(ctx).Model(&cliCommandApprovalRecord{}).Where("owner_user_id = ? AND state IN ? AND expires_at <= ?", ownerID, []string{"pending", "approved"}, now).Updates(map[string]any{"state": "expired", "version": gorm.Expr("version + 1")}).Error; err != nil {
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
	value := domain.CommandApproval{ID: row.ID, OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID, CommandDigest: row.CommandDigest, NonceHash: row.NonceHash, ConnectorName: row.ConnectorName, Operation: row.Operation, Target: row.Target, RedactedArguments: row.RedactedArguments, State: domain.ApprovalState(row.State), ExpiresAt: row.ExpiresAt, DecidedAt: row.DecidedAt, ConsumedAt: row.ConsumedAt, Version: row.Version}
	if row.Identity != nil {
		value.Identity = domain.ExecutionIdentity(*row.Identity)
	}
	return value
}

func (repository *Repository) Await(ctx context.Context, request cliconnector.ApprovalRequest) (cliconnector.ApprovalGrant, error) {
	now := time.Now().UTC()
	timeout := request.ExpiresAt.Sub(now)
	approval, err := domain.NewCommandApproval(uuid.NewString(), request.OwnerID, request.StageID, request.CommandDigest, request.Nonce, now, timeout)
	if err != nil || (request.ExecutionKind != "session" && request.ExecutionKind != "run") || request.ExecutionID == "" || request.ConnectorName == "" || request.Operation == "" || request.Target == "" || !slices.Contains(request.AllowedIdentities, request.Identity) || request.CommandDigests[request.Identity] != request.CommandDigest {
		return cliconnector.ApprovalGrant{}, fmt.Errorf("%w: invalid CLI command approval request", domain.ErrInvalid)
	}
	for _, identity := range request.AllowedIdentities {
		if (identity != cliconnector.IdentityUser && identity != cliconnector.IdentityBot) || len(request.CommandDigests[identity]) != 64 {
			return cliconnector.ApprovalGrant{}, fmt.Errorf("%w: invalid CLI command approval identities", domain.ErrInvalid)
		}
	}
	approval.ExecutionKind, approval.ExecutionID = request.ExecutionKind, request.ExecutionID
	approval.ConnectorName, approval.Operation, approval.Target = request.ConnectorName, request.Operation, request.Target
	approval.RedactedArguments = request.RedactedArguments
	row := cliCommandApprovalRecord{
		ID: approval.ID, OwnerID: approval.OwnerID, ExecutionKind: approval.ExecutionKind, ExecutionID: approval.ExecutionID,
		StageID: approval.StageID, ConnectorName: approval.ConnectorName, Operation: approval.Operation,
		Target: approval.Target, RedactedArguments: approval.RedactedArguments, CommandDigest: approval.CommandDigest,
		NonceHash: approval.NonceHash, State: string(approval.State), ExpiresAt: approval.ExpiresAt, Version: 1,
	}
	if len(request.AllowedIdentities) == 1 {
		approval.Identity = domain.ExecutionIdentity(request.AllowedIdentities[0])
		identity := string(approval.Identity)
		row.Identity = &identity
	}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := transitionApprovalExecution(tx, request, false, approval.ID, "user_action.required"); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		return cliconnector.ApprovalGrant{}, fmt.Errorf("persist CLI command approval: %w", err)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(time.Until(request.ExpiresAt))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
			return cliconnector.ApprovalGrant{}, ctx.Err()
		case <-timer.C:
			if err := repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalExpired); err != nil {
				return cliconnector.ApprovalGrant{}, err
			}
			return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalExpired
		case <-ticker.C:
			var current cliCommandApprovalRecord
			if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", row.ID, request.OwnerID).Take(&current).Error; err != nil {
				_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
				return cliconnector.ApprovalGrant{}, err
			}
			switch domain.ApprovalState(current.State) {
			case domain.ApprovalApproved:
				if current.Identity == nil || !slices.Contains(request.AllowedIdentities, cliconnector.Identity(*current.Identity)) {
					_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
					return cliconnector.ApprovalGrant{}, domain.ErrConflict
				}
				identity := cliconnector.Identity(*current.Identity)
				if err := repository.bindApprovedCommand(ctx, row.ID, request.OwnerID, request.CommandDigests[identity], identity); err != nil {
					_ = repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalClosed)
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{Nonce: request.Nonce, Identity: identity, ExpiresAt: request.ExpiresAt}, nil
			case domain.ApprovalRejected:
				if err := repository.closeCommandApproval(ctx, row.ID, request, domain.ApprovalRejected); err != nil {
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalRejected
			case domain.ApprovalExpired, domain.ApprovalClosed:
				if err := repository.closeCommandApproval(context.WithoutCancel(ctx), row.ID, request, domain.ApprovalState(current.State)); err != nil {
					return cliconnector.ApprovalGrant{}, err
				}
				return cliconnector.ApprovalGrant{}, cliconnector.ErrApprovalExpired
			}
		}
	}
}

func (repository *Repository) bindApprovedCommand(ctx context.Context, approvalID, ownerID, digest string, identity cliconnector.Identity) error {
	if len(digest) != 64 {
		return domain.ErrInvalid
	}
	result := repository.db.WithContext(ctx).Model(&cliCommandApprovalRecord{}).
		Where("id = ? AND owner_user_id = ? AND state = 'approved' AND identity = ?", approvalID, ownerID, identity).
		Update("command_digest", digest)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (repository *Repository) Consume(ctx context.Context, ownerID, digest, nonce string) error {
	now := time.Now().UTC()
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row cliCommandApprovalRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_user_id = ? AND command_digest = ? AND nonce_hash = ?", ownerID, digest, domain.ApprovalNonceHash(nonce)).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		approval := commandApprovalDomain(row)
		if err := approval.Consume(ownerID, digest, nonce, now); err != nil {
			return err
		}
		result := tx.Model(&cliCommandApprovalRecord{}).Where("id = ? AND version = ? AND state = 'approved'", row.ID, row.Version).Updates(map[string]any{
			"state": string(domain.ApprovalConsumed), "consumed_at": approval.ConsumedAt, "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return domain.ErrConflict
		}
		request := cliconnector.ApprovalRequest{OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID}
		return transitionApprovalExecution(tx, request, true, row.ID, "user_action.resolved")
	})
}

func (repository *Repository) Close(ctx context.Context, ownerID, nonce string) error {
	var row cliCommandApprovalRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND nonce_hash = ?", ownerID, domain.ApprovalNonceHash(nonce)).Take(&row).Error; err != nil {
		return mapNotFound(err)
	}
	request := cliconnector.ApprovalRequest{OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID}
	return repository.closeCommandApproval(ctx, row.ID, request, domain.ApprovalClosed)
}

func (repository *Repository) closeCommandApproval(ctx context.Context, approvalID string, request cliconnector.ApprovalRequest, state domain.ApprovalState) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row cliCommandApprovalRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", approvalID, request.OwnerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == string(domain.ApprovalConsumed) {
			return nil
		}
		shouldClose := state == domain.ApprovalExpired && (row.State == string(domain.ApprovalPending) || row.State == string(domain.ApprovalApproved))
		shouldClose = shouldClose || state == domain.ApprovalClosed && (row.State == string(domain.ApprovalPending) || row.State == string(domain.ApprovalApproved))
		if shouldClose {
			if err := tx.Model(&cliCommandApprovalRecord{}).Where("id = ?", row.ID).Updates(map[string]any{"state": string(state), "version": gorm.Expr("version + 1")}).Error; err != nil {
				return err
			}
		}
		return transitionApprovalExecution(tx, request, true, row.ID, "user_action.resolved")
	})
}

func transitionApprovalExecution(tx *gorm.DB, request cliconnector.ApprovalRequest, resume bool, approvalID, eventType string) error {
	from, to := "running", "waiting_for_user"
	if resume {
		from, to = to, from
	}
	if request.ExecutionKind == "session" {
		messageID, err := strconv.ParseInt(request.ExecutionID, 10, 64)
		if err != nil || messageID <= 0 {
			return domain.ErrInvalid
		}
		if !resume {
			from, to = "generating", "waiting_for_user"
		} else {
			from, to = "waiting_for_user", "generating"
		}
		result := tx.Model(&messageRecord{}).Where("id = ? AND state = ? AND session_id IN (SELECT id FROM sessions WHERE owner_user_id = ?)", messageID, from, request.OwnerID).Updates(map[string]any{"state": to, "progress_stage": map[bool]string{false: "waiting_for_user", true: "using_tool"}[resume]})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			if resume && result.RowsAffected == 0 {
				return nil
			}
			return domain.ErrConflict
		}
		return nil
	}
	result := tx.Model(&runRecord{}).Where("id = ? AND owner_user_id = ? AND state = ?", request.ExecutionID, request.OwnerID, from).Updates(map[string]any{"state": to, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		if resume && result.RowsAffected == 0 {
			return nil
		}
		return domain.ErrConflict
	}
	payload, err := json.Marshal(map[string]string{"approval_id": approvalID})
	if err != nil {
		return err
	}
	var sequence int64
	if err := tx.Table("run_events").Select("COALESCE(MAX(sequence), 0)").Where("run_id = ?", request.ExecutionID).Scan(&sequence).Error; err != nil {
		return err
	}
	return tx.Table("run_events").Create(map[string]any{"run_id": request.ExecutionID, "sequence": sequence + 1, "event_type": eventType, "payload": payload, "occurred_at": time.Now().UTC()}).Error
}
