package gormrepo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *Repository) AwaitAction(ctx context.Context, request cliconnector.ActionRequest) error {
	if err := cliconnector.ValidateActionRequest(request); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	if request.OwnerID == "" || request.OperationID == "" || request.ConnectorID == "" || request.AuthorizationScheme == "" || request.EnablementID == "" || request.CapabilityID == "" ||
		(request.ExecutionKind != "session" && request.ExecutionKind != "run") || request.ExecutionID == "" || request.StageID == "" ||
		!request.ExpiresAt.After(time.Now().UTC()) || len(request.Actions) == 0 {
		return fmt.Errorf("%w: invalid Connector action requirement", domain.ErrInvalid)
	}
	permissions, _ := json.Marshal(request.Permissions)
	actions, _ := json.Marshal(request.Actions)
	phrase, _ := json.Marshal(request.OperationPhrase)
	row := connectorActionRequirementRecord{
		ID: uuid.NewString(), OwnerID: request.OwnerID, ExecutionKind: request.ExecutionKind, ExecutionID: request.ExecutionID,
		StageID: request.StageID, OperationID: request.OperationID, ConnectorID: request.ConnectorID, ConnectorName: request.ConnectorName,
		AuthorizationScheme: request.AuthorizationScheme,
		Identity:            string(request.Identity),
		EnablementID:        request.EnablementID, CapabilityID: request.CapabilityID, OperationPhrase: phrase,
		Reason: string(request.Reason), Permissions: permissions, Actions: actions, State: string(cliconnector.ActionPending),
		ExpiresAt: request.ExpiresAt, Version: 1,
	}
	if err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := transitionApprovalExecution(tx, cliconnector.ApprovalRequest{OwnerID: request.OwnerID, ExecutionKind: request.ExecutionKind, ExecutionID: request.ExecutionID, StageID: request.StageID}, false, row.ID, "connector.action.required"); err != nil {
			return err
		}
		return tx.Create(&row).Error
	}); err != nil {
		return fmt.Errorf("persist Connector action requirement: %w", err)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(time.Until(request.ExpiresAt))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = repository.finishConnectorAction(context.WithoutCancel(ctx), request.OwnerID, row.ID, cliconnector.ActionCancelled)
			return ctx.Err()
		case <-timer.C:
			_ = repository.finishConnectorAction(context.WithoutCancel(ctx), request.OwnerID, row.ID, cliconnector.ActionExpired)
			return cliconnector.ErrActionExpired
		case <-ticker.C:
			var current connectorActionRequirementRecord
			if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", row.ID, request.OwnerID).Take(&current).Error; err != nil {
				return err
			}
			switch cliconnector.ActionState(current.State) {
			case cliconnector.ActionResolved:
				return nil
			case cliconnector.ActionCancelled:
				return cliconnector.ErrActionRejected
			case cliconnector.ActionExpired:
				return cliconnector.ErrActionExpired
			}
		}
	}
}

func (repository *Repository) ListConnectorActions(ctx context.Context, ownerID string, now time.Time) ([]cliconnector.ActionRequirement, error) {
	var expired []connectorActionRequirementRecord
	if err := repository.db.WithContext(ctx).Select("id").Where("owner_user_id = ? AND state = 'pending' AND expires_at <= ?", ownerID, now).Find(&expired).Error; err != nil {
		return nil, err
	}
	for _, row := range expired {
		if err := repository.finishConnectorAction(ctx, ownerID, row.ID, cliconnector.ActionExpired); err != nil && !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
	}
	var rows []connectorActionRequirementRecord
	if err := repository.db.WithContext(ctx).Where("owner_user_id = ? AND state = 'pending'", ownerID).Order("created_at, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]cliconnector.ActionRequirement, 0, len(rows))
	for _, row := range rows {
		item, err := connectorActionDomain(row)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (repository *Repository) GetConnectorAction(ctx context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	var row connectorActionRequirementRecord
	if err := repository.db.WithContext(ctx).Where("id = ? AND owner_user_id = ?", id, ownerID).Take(&row).Error; err != nil {
		return cliconnector.ActionRequirement{}, mapNotFound(err)
	}
	return connectorActionDomain(row)
}

func (repository *Repository) UpdateConnectorActionReason(ctx context.Context, ownerID, id string, reason cliconnector.ActionReason) (cliconnector.ActionRequirement, error) {
	result := repository.db.WithContext(ctx).Model(&connectorActionRequirementRecord{}).
		Where("id = ? AND owner_user_id = ? AND state = 'pending' AND expires_at > ?", id, ownerID, time.Now().UTC()).
		Updates(map[string]any{"reason": string(reason), "action_url_token_hash": nil, "action_url_opened_at": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return cliconnector.ActionRequirement{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.ActionRequirement{}, domain.ErrConflict
	}
	return repository.GetConnectorAction(ctx, ownerID, id)
}

func (repository *Repository) ResolveConnectorAction(ctx context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	if err := repository.finishConnectorAction(ctx, ownerID, id, cliconnector.ActionResolved); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	return repository.GetConnectorAction(ctx, ownerID, id)
}

func (repository *Repository) CancelConnectorAction(ctx context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	if err := repository.finishConnectorAction(ctx, ownerID, id, cliconnector.ActionCancelled); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	return repository.GetConnectorAction(ctx, ownerID, id)
}

func (repository *Repository) finishConnectorAction(ctx context.Context, ownerID, id string, state cliconnector.ActionState) error {
	return repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row connectorActionRequirementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, ownerID).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.State == string(state) {
			return nil
		}
		if row.State != string(cliconnector.ActionPending) {
			return domain.ErrConflict
		}
		if state == cliconnector.ActionResolved && !time.Now().UTC().Before(row.ExpiresAt) {
			state = cliconnector.ActionExpired
		}
		if err := tx.Model(&connectorActionRequirementRecord{}).Where("id = ? AND state = 'pending'", id).Updates(map[string]any{"state": string(state), "action_url_token_hash": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")}).Error; err != nil {
			return err
		}
		request := cliconnector.ApprovalRequest{OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID, StageID: row.StageID}
		return transitionApprovalExecution(tx, request, true, row.ID, "connector.action.resolved")
	})
}

func (repository *Repository) connectorActionURL(ctx context.Context, item cliconnector.ActionRequirement) string {
	return connectorActionURL(repository.db.WithContext(ctx), item)
}

func (repository *Repository) ResetConnectorActionProviderURL(ctx context.Context, ownerID, id string) (cliconnector.ActionRequirement, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	hash := sha256.Sum256([]byte(token))
	result := repository.db.WithContext(ctx).Model(&connectorActionRequirementRecord{}).
		Where("id = ? AND owner_user_id = ? AND state = 'pending' AND expires_at > ?", id, ownerID, time.Now().UTC()).
		Updates(map[string]any{"action_url_token_hash": hash[:], "action_url_opened_at": nil, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return cliconnector.ActionRequirement{}, result.Error
	}
	if result.RowsAffected != 1 {
		return cliconnector.ActionRequirement{}, domain.ErrConflict
	}
	item, err := repository.GetConnectorAction(ctx, ownerID, id)
	item.ActionURLToken = token
	return item, err
}

func (repository *Repository) ConsumeConnectorActionProviderURL(ctx context.Context, id string, expectedVersion int64, token string) (string, error) {
	var providerURL string
	presentedHash := sha256.Sum256([]byte(token))
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row connectorActionRequirementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).Take(&row).Error; err != nil {
			return mapNotFound(err)
		}
		if row.Version != expectedVersion || row.State != string(cliconnector.ActionPending) || row.ActionURLOpenedAt != nil || len(row.ActionURLTokenHash) != sha256.Size || subtle.ConstantTimeCompare(row.ActionURLTokenHash, presentedHash[:]) != 1 || !time.Now().UTC().Before(row.ExpiresAt) {
			return domain.ErrConflict
		}
		item, err := connectorActionDomain(row)
		if err != nil {
			return err
		}
		providerURL = connectorActionURL(tx.WithContext(ctx), item)
		if providerURL == "" {
			return domain.ErrNotFound
		}
		now := time.Now().UTC()
		result := tx.Model(&connectorActionRequirementRecord{}).Where("id = ? AND version = ? AND action_url_opened_at IS NULL", id, expectedVersion).
			Updates(map[string]any{"action_url_token_hash": nil, "action_url_opened_at": now, "updated_at": gorm.Expr("now()"), "version": gorm.Expr("version + 1")})
		if result.Error != nil || result.RowsAffected != 1 {
			if result.Error != nil {
				return result.Error
			}
			return domain.ErrConflict
		}
		return nil
	})
	return providerURL, err
}

func connectorActionURL(db *gorm.DB, item cliconnector.ActionRequirement) string {
	if item.Reason == cliconnector.ReasonSetupRequired {
		var enablement cliConnectorEnablementRecord
		if db.Where("id = ? AND owner_user_id = ?", item.EnablementID, item.OwnerID).Take(&enablement).Error == nil && enablement.ActionURL != nil && enablement.ActionExpiresAt != nil && time.Now().UTC().Before(*enablement.ActionExpiresAt) {
			return *enablement.ActionURL
		}
		return ""
	}
	var attempt cliConnectorAuthorizationAttemptRecord
	if db.Where("owner_user_id = ? AND enablement_id = ? AND expires_at > ?", item.OwnerID, item.EnablementID, time.Now().UTC()).Order("created_at DESC").Take(&attempt).Error == nil {
		return attempt.ActionURL
	}
	return ""
}

func connectorActionDomain(row connectorActionRequirementRecord) (cliconnector.ActionRequirement, error) {
	var permissions []string
	var actions []cliconnector.UserAction
	var phrase map[string]string
	if err := json.Unmarshal(row.Permissions, &permissions); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	if err := json.Unmarshal(row.Actions, &actions); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	if err := json.Unmarshal(row.OperationPhrase, &phrase); err != nil {
		return cliconnector.ActionRequirement{}, err
	}
	return cliconnector.ActionRequirement{
		ContractVersion: 1, ID: row.ID, OwnerID: row.OwnerID, ExecutionKind: row.ExecutionKind, ExecutionID: row.ExecutionID,
		StageID: row.StageID, OperationID: row.OperationID, ConnectorID: row.ConnectorID, ConnectorName: row.ConnectorName,
		AuthorizationScheme: row.AuthorizationScheme,
		Identity:            cliconnector.Identity(row.Identity),
		EnablementID:        row.EnablementID, CapabilityID: row.CapabilityID, OperationPhrase: phrase,
		Reason: cliconnector.ActionReason(row.Reason), Permissions: permissions, Actions: actions,
		State: cliconnector.ActionState(row.State), ActionURLOpenedAt: row.ActionURLOpenedAt, ExpiresAt: row.ExpiresAt, Version: row.Version,
	}, nil
}

var _ cliconnector.ActionCoordinator = (*Repository)(nil)
