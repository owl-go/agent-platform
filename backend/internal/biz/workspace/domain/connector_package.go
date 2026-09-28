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

type ConnectorPublicationState string

const (
	ConnectorPublicationAvailable ConnectorPublicationState = "available"
	ConnectorPublicationDisabled  ConnectorPublicationState = "disabled"
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

type ConnectorPublication struct {
	PackageSource    string
	ActiveRevisionID string
	State            ConnectorPublicationState
	AdministratorID  string
	Version          int64
	UpdatedAt        time.Time
}

type ConnectorPublicationHealth struct {
	Publication              ConnectorPublication
	Revision                 ConnectorRevision
	InstallationCount        int64
	ActiveInstallationCount  int64
	ActiveAuthorizationCount int64
}

func (publication *ConnectorPublication) Publish(administratorID, revisionID string, now time.Time) error {
	if publication == nil || strings.TrimSpace(publication.PackageSource) == "" || administratorID == "" || revisionID == "" {
		return fmt.Errorf("%w: connector publication is incomplete", ErrInvalid)
	}
	publication.ActiveRevisionID = revisionID
	publication.State = ConnectorPublicationAvailable
	publication.AdministratorID = administratorID
	publication.Version++
	publication.UpdatedAt = now
	return nil
}

func (publication *ConnectorPublication) Disable(administratorID string, now time.Time) error {
	if publication == nil || publication.PackageSource == "" || publication.ActiveRevisionID == "" || administratorID == "" {
		return fmt.Errorf("%w: connector publication is incomplete", ErrInvalid)
	}
	publication.State = ConnectorPublicationDisabled
	publication.AdministratorID = administratorID
	publication.Version++
	publication.UpdatedAt = now
	return nil
}

type ConnectorInstallation struct {
	ID               string
	OwnerID          string
	PackageSource    string
	ActiveRevisionID string
	AuthorizationID  string
	Authorized       bool
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
	ID                          string
	OwnerID                     string
	InstallationID              string
	IdentityRef                 string
	ExternalIdentityID          string
	ExternalDisplayName         string
	Scopes                      []string
	CredentialCiphertext        []byte
	CredentialAAD               string
	CredentialFormat            string
	RefreshCredentialCiphertext []byte
	RefreshCredentialAAD        string
	State                       ConnectorAuthorizationState
	ExpiresAt                   *time.Time
	Version                     int64
	UpdatedAt                   time.Time
}

type ConnectorAuthorizationMaterial struct {
	CredentialCiphertext []byte
	CredentialAAD        string
	CredentialFormat     string
	Scopes               []string
	AppIDCiphertext      []byte
	AppSecretCiphertext  []byte
}

type ConnectorSetup struct {
	ID                   string
	OwnerID              string
	InstallationID       string
	State                string
	ActionURL            string
	ExpiresAt            *time.Time
	DeviceCodeCiphertext []byte
	ProviderName         string
	DeveloperConsoleURL  string
}

type ConnectorProviderApplication struct {
	OwnerID             string
	InstallationID      string
	AppIDCiphertext     []byte
	AppSecretCiphertext []byte
	ProviderName        string
	DeveloperConsoleURL string
}

type ConnectorAuthorizationAttempt struct {
	ID                   string
	OwnerID              string
	InstallationID       string
	Identity             string
	Scopes               []string
	ActionURL            string
	ExpiresAt            time.Time
	DeviceCodeCiphertext []byte
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
