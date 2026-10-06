package domain

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

var (
	ErrUnauthenticated         = errors.New("request is not authenticated")
	ErrForbidden               = errors.New("request is not authorized")
	ErrNotFound                = errors.New("User not found")
	ErrConflict                = errors.New("User conflicts with current state")
	ErrLoginCodeConflict       = errors.New("registration login code is already reserved")
	ErrRegistrationRateLimited = errors.New("registration verification rate limit reached")
)

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{2,63}$`)

type VerifiedIdentity struct {
	Subject string
}

type Principal struct {
	UserID                 string
	Username               string
	Email                  string
	DisplayName            string
	Administrator          bool
	BootstrapAdministrator bool
	ResourcePublisher      bool
	Groups                 []IdentityGroup
	Disabled               bool
}

func (principal Principal) Validate() error {
	if principal.Disabled || strings.TrimSpace(principal.UserID) == "" {
		return ErrUnauthenticated
	}
	return nil
}

func (principal Principal) RequireAdministrator() error {
	if err := principal.Validate(); err != nil {
		return err
	}
	if !principal.Administrator {
		return ErrForbidden
	}
	return nil
}

type User struct {
	ID                     string
	OIDCSubject            string
	Username               string
	Email                  string
	DisplayName            string
	Administrator          bool
	BootstrapAdministrator bool
	ResourcePublisher      bool
	Enabled                bool
	Groups                 []IdentityGroup
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
}

type IdentityGroup struct {
	ID                         string
	ExternalID                 string
	Name                       string
	Path                       string
	Department                 bool
	DailyCreditLimitHundredths *int64
	MemberCount                int64
	LastSyncedAt               time.Time
	DeletedAt                  *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	Version                    int64
}

type IdentityGroupSnapshot struct {
	ExternalID     string
	Name           string
	Path           string
	Department     bool
	MemberSubjects []string
}

func (group IdentityGroupSnapshot) Validate() error {
	if strings.TrimSpace(group.ExternalID) == "" || strings.TrimSpace(group.Name) == "" || strings.TrimSpace(group.Path) == "" {
		return fmt.Errorf("identity group external ID, name, and path are required")
	}
	if len(group.MemberSubjects) > 10_000 {
		return fmt.Errorf("identity group has too many members")
	}
	return nil
}

type GovernanceAuditEvent struct {
	ID          int64
	ActorUserID string
	Action      string
	TargetType  string
	TargetID    string
	Reason      string
	Detail      map[string]int64
	OccurredAt  time.Time
}

type NewUser struct {
	Username    string
	Email       string
	DisplayName string
}

func (user NewUser) Validate() error {
	if !usernamePattern.MatchString(strings.TrimSpace(user.Username)) {
		return fmt.Errorf("username must start with a letter and contain 3-64 lowercase letters, numbers, dots, dashes, or underscores")
	}
	address, err := mail.ParseAddress(strings.TrimSpace(user.Email))
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(user.Email)) {
		return fmt.Errorf("email must be a valid address")
	}
	name := strings.TrimSpace(user.DisplayName)
	if len(name) < 1 || len(name) > 100 {
		return fmt.Errorf("display name must contain 1-100 characters")
	}
	return nil
}
