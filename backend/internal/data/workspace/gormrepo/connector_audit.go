package gormrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/cliconnector"

	"github.com/google/uuid"
)

var connectorAuditDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (repository *Repository) RecordConnectorAudit(ctx context.Context, record cliconnector.AuditRecord) error {
	if record.ContractVersion != 1 || record.OperationID == "" || record.UserID == "" || record.ConnectorID == "" ||
		record.CapabilityID == "" || !strings.HasPrefix(record.Action, "operation.") || !connectorAuditDigest.MatchString(record.InputDigest) || record.OccurredAt.IsZero() {
		return fmt.Errorf("%w: invalid Connector audit record", domain.ErrInvalid)
	}
	permissions, err := json.Marshal(record.Permissions)
	if err != nil {
		return fmt.Errorf("encode Connector audit Permissions: %w", err)
	}
	var authorizationID *string
	if record.AuthorizationID != "" {
		authorizationID = &record.AuthorizationID
	}
	row := connectorAuditRecord{
		ID: uuid.NewString(), OperationID: record.OperationID, OwnerID: record.UserID, ConnectorID: record.ConnectorID,
		ManifestVersion: record.ManifestVersion, CapabilityID: record.CapabilityID, Permissions: permissions,
		ExecutionIdentity: string(record.ExecutionIdentity), AuthorizationID: authorizationID, Action: record.Action, Reason: record.Reason, Result: record.Result,
		TargetSummary: record.TargetSummary, InputDigest: record.InputDigest, OccurredAt: record.OccurredAt,
	}
	if err := repository.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("persist Connector audit record: %w", err)
	}
	return nil
}
