package domain

import (
	"fmt"
	"strings"
	"time"
)

type ConnectorMode string

const (
	ConnectorModeMCP ConnectorMode = "mcp"
	ConnectorModeCLI ConnectorMode = "cli"
)

type ConnectorInstallationState string

const (
	ConnectorInstallationPending     ConnectorInstallationState = "pending"
	ConnectorInstallationActive      ConnectorInstallationState = "active"
	ConnectorInstallationDisabled    ConnectorInstallationState = "disabled"
	ConnectorInstallationUninstalled ConnectorInstallationState = "uninstalled"
)

type ConnectorAuthorizationState string

const (
	ConnectorAuthorizationActive       ConnectorAuthorizationState = "active"
	ConnectorAuthorizationExpired      ConnectorAuthorizationState = "expired"
	ConnectorAuthorizationDisconnected ConnectorAuthorizationState = "disconnected"
	ConnectorAuthorizationRevoked      ConnectorAuthorizationState = "revoked"
)

type ConnectorRevision struct {
	ID            string
	PackageSource string
	Version       string
	Mode          ConnectorMode
	PackageSHA256 string
	RuntimePolicy []byte
	ObjectKey     string
	CreatedAt     time.Time
}

type ConnectorInstallation struct {
	ID               string
	OwnerID          string
	PackageSource    string
	ActiveRevisionID string
	AuthorizationID  string
	State            ConnectorInstallationState
	Version          int64
	UpdatedAt        time.Time
}

func (installation *ConnectorInstallation) ActivateRevision(ownerID, revisionID string, now time.Time) error {
	if installation == nil || ownerID == "" || ownerID != installation.OwnerID || revisionID == "" {
		return fmt.Errorf("%w: connector installation owner or revision is invalid", ErrInvalid)
	}
	if installation.State == ConnectorInstallationUninstalled {
		return fmt.Errorf("%w: connector installation is uninstalled", ErrConflict)
	}
	installation.ActiveRevisionID = revisionID
	installation.State = ConnectorInstallationActive
	installation.UpdatedAt = now
	installation.Version++
	return nil
}

func (installation *ConnectorInstallation) RollbackRevision(ownerID, revisionID, reason string, now time.Time) error {
	if installation == nil || ownerID == "" || ownerID != installation.OwnerID || revisionID == "" {
		return fmt.Errorf("%w: connector installation owner or revision is invalid", ErrInvalid)
	}
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("%w: connector rollback reason is required", ErrInvalid)
	}
	if installation.State == ConnectorInstallationUninstalled {
		return fmt.Errorf("%w: connector installation is uninstalled", ErrConflict)
	}
	installation.ActiveRevisionID = revisionID
	installation.State = ConnectorInstallationActive
	installation.UpdatedAt = now
	installation.Version++
	return nil
}

type ConnectorAuthorization struct {
	ID                   string
	OwnerID              string
	InstallationID       string
	IdentityRef          string
	Scopes               []string
	CredentialCiphertext []byte
	State                ConnectorAuthorizationState
	ExpiresAt            *time.Time
	Version              int64
	UpdatedAt            time.Time
}

func (authorization ConnectorAuthorization) CanInvoke() bool {
	return authorization.State == ConnectorAuthorizationActive && (authorization.ExpiresAt == nil || time.Now().UTC().Before(*authorization.ExpiresAt))
}

func (authorization *ConnectorAuthorization) Disconnect(ownerID string) error {
	if authorization == nil || ownerID == "" || authorization.OwnerID != ownerID {
		return ErrNotFound
	}
	if authorization.State == ConnectorAuthorizationDisconnected {
		return nil
	}
	if authorization.State != ConnectorAuthorizationActive && authorization.State != ConnectorAuthorizationExpired {
		return fmt.Errorf("%w: connector authorization cannot be disconnected", ErrConflict)
	}
	authorization.State = ConnectorAuthorizationDisconnected
	authorization.Version++
	authorization.UpdatedAt = time.Now().UTC()
	return nil
}

type ConnectorAuditRecord struct {
	ID                string
	OwnerID           string
	InstallationID    string
	RevisionID        string
	Mode              ConnectorMode
	Operation         string
	Risk              string
	IdentityRef       string
	ApprovalReference string
	Outcome           string
	ErrorType         string
	RequestID         string
	PolicyRevision    string
	CreatedAt         time.Time
}
