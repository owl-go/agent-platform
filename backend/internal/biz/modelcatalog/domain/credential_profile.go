package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrCredentialProfileNotFound = errors.New("Credential Profile not found")
	ErrConfiguredModelNotFound   = errors.New("Configured Model not found")
	ErrConcurrentUpdate          = errors.New("catalog resource was modified concurrently")
	ErrCatalogNameExists         = errors.New("catalog resource name already exists")
	ErrInvalidCatalogInput       = errors.New("invalid Model Catalog input")
	secretReferencePattern       = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://[^\s]+$`)
)

type CredentialKind string

const (
	ModelCredential         CredentialKind = "model"
	GitSSHCredential        CredentialKind = "git_ssh"
	BuildCredential         CredentialKind = "build"
	ObjectStorageCredential CredentialKind = "object_storage"
)

type CredentialProfile struct {
	ID             string
	OrganizationID string
	TeamID         *string
	Name           string
	Kind           CredentialKind
	SecretRef      string
	DisabledAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Version        int64
}

type CredentialRegistration struct {
	ID             string
	OrganizationID string
	TeamID         *string
	Name           string
	Kind           CredentialKind
	SecretRef      string
	Now            time.Time
}

func RegisterCredential(registration CredentialRegistration) (CredentialProfile, error) {
	if strings.TrimSpace(registration.ID) == "" || strings.TrimSpace(registration.OrganizationID) == "" || strings.TrimSpace(registration.Name) == "" {
		return CredentialProfile{}, invalidf("Credential Profile ID, Organization ID, and name are required")
	}
	if registration.TeamID != nil && strings.TrimSpace(*registration.TeamID) == "" {
		return CredentialProfile{}, invalidf("Credential Profile Team ID cannot be empty")
	}
	if err := validateCredentialKind(registration.Kind); err != nil {
		return CredentialProfile{}, err
	}
	if !secretReferencePattern.MatchString(registration.SecretRef) {
		return CredentialProfile{}, invalidf("Credential Profile must contain a Secret Manager reference, not secret material")
	}
	if registration.Now.IsZero() {
		return CredentialProfile{}, invalidf("registration time is required")
	}
	now := registration.Now.UTC()
	return CredentialProfile{
		ID: registration.ID, OrganizationID: registration.OrganizationID, TeamID: cloneString(registration.TeamID),
		Name: strings.TrimSpace(registration.Name), Kind: registration.Kind, SecretRef: registration.SecretRef,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}, nil
}

func RestoreCredential(id, organizationID string, teamID *string, name, kindValue, secretRef string, disabledAt *time.Time, createdAt, updatedAt time.Time, version int64) (CredentialProfile, error) {
	kind := CredentialKind(kindValue)
	if err := validateCredentialKind(kind); err != nil {
		return CredentialProfile{}, err
	}
	if id == "" || organizationID == "" || name == "" || !secretReferencePattern.MatchString(secretRef) || createdAt.IsZero() || updatedAt.IsZero() || version <= 0 {
		return CredentialProfile{}, invalidf("invalid persisted Credential Profile")
	}
	return CredentialProfile{
		ID: id, OrganizationID: organizationID, TeamID: cloneString(teamID), Name: name, Kind: kind,
		SecretRef: secretRef, DisabledAt: cloneTime(disabledAt), CreatedAt: createdAt, UpdatedAt: updatedAt, Version: version,
	}, nil
}

func (profile CredentialProfile) Enabled() bool { return profile.DisabledAt == nil }

func (profile *CredentialProfile) SetEnabled(enabled bool, now time.Time) error {
	if now.IsZero() {
		return invalidf("status change time is required")
	}
	if enabled == profile.Enabled() {
		return nil
	}
	if enabled {
		profile.DisabledAt = nil
	} else {
		disabledAt := now.UTC()
		profile.DisabledAt = &disabledAt
	}
	profile.UpdatedAt = now.UTC()
	profile.Version++
	return nil
}

func validateCredentialKind(kind CredentialKind) error {
	switch kind {
	case ModelCredential, GitSSHCredential, BuildCredential, ObjectStorageCredential:
		return nil
	default:
		return invalidf("unknown Credential Profile kind %q", kind)
	}
}

func invalidf(format string, arguments ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidCatalogInput, fmt.Sprintf(format, arguments...))
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
