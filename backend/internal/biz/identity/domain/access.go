package domain

import (
	"errors"
	"fmt"
)

var (
	ErrUnauthenticated           = errors.New("request is not authenticated")
	ErrAuthenticationUnavailable = errors.New("authentication infrastructure is unavailable")
	ErrForbidden                 = errors.New("request is not authorized")
	ErrUserNotFound              = errors.New("User not found")
	ErrRunNotFound               = errors.New("Run not found")
)

type Role string

const (
	PlatformAdministrator Role = "platform_administrator"
	AgentBuilder          Role = "agent_builder"
	AgentUser             Role = "agent_user"
	RunOperator           Role = "run_operator"
)

type Grant struct {
	TeamID *string
	Role   Role
}

type VerifiedIdentity struct {
	Subject          string
	OrganizationSlug string
}

type Principal struct {
	UserID         string
	OrganizationID string
	Disabled       bool
	Grants         []Grant
}

type Actor struct {
	UserID         string
	OrganizationID string
	TeamID         string
}

func (principal Principal) AuthorizeTeamRead(teamID string) (Actor, error) {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return Actor{}, ErrUnauthenticated
	}
	if teamID == "" {
		return Actor{}, ErrForbidden
	}
	for _, grant := range principal.Grants {
		if grant.TeamID == nil || *grant.TeamID == teamID {
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser, RunOperator:
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: teamID}, nil
			}
		}
	}
	return Actor{}, ErrForbidden
}

func (principal Principal) AuthorizeAgentBuild(teamID string) (Actor, error) {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return Actor{}, ErrUnauthenticated
	}
	if teamID == "" {
		return Actor{}, ErrForbidden
	}
	for _, grant := range principal.Grants {
		if grant.TeamID == nil && grant.Role == PlatformAdministrator || (grant.TeamID == nil || *grant.TeamID == teamID) && grant.Role == AgentBuilder {
			return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: teamID}, nil
		}
	}
	return Actor{}, ErrForbidden
}

func (principal Principal) AuthorizeTaskUse(teamID string) (Actor, error) {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return Actor{}, ErrUnauthenticated
	}
	if teamID == "" {
		return Actor{}, ErrForbidden
	}
	for _, grant := range principal.Grants {
		if grant.TeamID == nil || *grant.TeamID == teamID {
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser:
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: teamID}, nil
			}
		}
	}
	return Actor{}, ErrForbidden
}

type RunScope struct {
	OrganizationID string
	TeamID         string
}

func ParseRole(value string) (Role, error) {
	role := Role(value)
	switch role {
	case PlatformAdministrator, AgentBuilder, AgentUser, RunOperator:
		return role, nil
	default:
		return "", fmt.Errorf("unknown Role %q", value)
	}
}

func (principal Principal) AuthorizeRunRead(scope RunScope) error {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return ErrUnauthenticated
	}
	if scope.OrganizationID == "" || scope.TeamID == "" || principal.OrganizationID != scope.OrganizationID {
		return ErrForbidden
	}
	for _, grant := range principal.Grants {
		if grant.TeamID == nil || *grant.TeamID == scope.TeamID {
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser, RunOperator:
				return nil
			}
		}
	}
	return ErrForbidden
}

func (principal Principal) AuthorizeRunControl(scope RunScope, action string) (Actor, error) {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return Actor{}, ErrUnauthenticated
	}
	if scope.OrganizationID == "" || scope.TeamID == "" || principal.OrganizationID != scope.OrganizationID {
		return Actor{}, ErrForbidden
	}
	for _, grant := range principal.Grants {
		if grant.TeamID != nil && *grant.TeamID != scope.TeamID {
			continue
		}
		switch action {
		case "interrupt", "cancel":
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser, RunOperator:
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: scope.TeamID}, nil
			}
		case "resume":
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser:
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: scope.TeamID}, nil
			}
		case "approval_request", "approval_decide":
			switch grant.Role {
			case PlatformAdministrator, AgentBuilder, AgentUser:
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: scope.TeamID}, nil
			}
		case "kill":
			if grant.Role == RunOperator || grant.Role == PlatformAdministrator && grant.TeamID == nil {
				return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID, TeamID: scope.TeamID}, nil
			}
		default:
			return Actor{}, ErrForbidden
		}
	}
	return Actor{}, ErrForbidden
}

func (principal Principal) AuthorizeRuntimeImageRead() error {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return ErrUnauthenticated
	}
	for _, grant := range principal.Grants {
		switch grant.Role {
		case PlatformAdministrator, AgentBuilder, AgentUser, RunOperator:
			return nil
		}
	}
	return ErrForbidden
}

func (principal Principal) AuthorizeRuntimeImageWrite() error {
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return ErrUnauthenticated
	}
	for _, grant := range principal.Grants {
		if grant.TeamID == nil && grant.Role == PlatformAdministrator {
			return nil
		}
	}
	return ErrForbidden
}

func (principal Principal) AuthorizeModelCatalogRead() (Actor, error) {
	if err := principal.AuthorizeRuntimeImageRead(); err != nil {
		return Actor{}, err
	}
	return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID}, nil
}

func (principal Principal) AuthorizeModelCatalogWrite() (Actor, error) {
	if err := principal.AuthorizeRuntimeImageWrite(); err != nil {
		return Actor{}, err
	}
	return Actor{UserID: principal.UserID, OrganizationID: principal.OrganizationID}, nil
}
