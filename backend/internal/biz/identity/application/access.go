package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/authz"
	"agent-platform/backend/internal/biz/identity/domain"
)

type TokenVerifier interface {
	Verify(context.Context, string) (domain.VerifiedIdentity, error)
}

type Repository interface {
	FindPrincipal(context.Context, domain.VerifiedIdentity) (domain.Principal, error)
	FindRunScope(context.Context, string) (domain.RunScope, error)
}

type AccessService struct {
	verifier   TokenVerifier
	repository Repository
}

type principalContextKey struct{}

func WithPrincipal(ctx context.Context, principal domain.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (domain.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(domain.Principal)
	return principal, ok
}

func NewAccessService(verifier TokenVerifier, repository Repository) (*AccessService, error) {
	if verifier == nil || repository == nil {
		return nil, fmt.Errorf("Token Verifier and Identity Repository are required")
	}
	return &AccessService{verifier: verifier, repository: repository}, nil
}

func (service *AccessService) AuthorizeRunRead(ctx context.Context, token, runID string) error {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return err
	}
	scope, err := service.repository.FindRunScope(ctx, runID)
	if errors.Is(err, domain.ErrRunNotFound) {
		// Do not reveal whether an inaccessible Run exists.
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("resolve Run scope: %w", err)
	}
	return principal.AuthorizeRunRead(scope)
}

func (service *AccessService) AuthorizeRunControl(ctx context.Context, token, runID, action string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	scope, err := service.repository.FindRunScope(ctx, runID)
	if errors.Is(err, domain.ErrRunNotFound) {
		return domain.Actor{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Actor{}, fmt.Errorf("resolve Run scope: %w", err)
	}
	return principal.AuthorizeRunControl(scope, action)
}

func (service *AccessService) AuthorizeRuntimeImageRead(ctx context.Context, token string) error {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return err
	}
	return principal.AuthorizeRuntimeImageRead()
}

func (service *AccessService) AuthorizeRuntimeImageWrite(ctx context.Context, token string) error {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return err
	}
	return principal.AuthorizeRuntimeImageWrite()
}

func (service *AccessService) AuthorizeModelCatalogRead(ctx context.Context, token string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	return principal.AuthorizeModelCatalogRead()
}

func (service *AccessService) AuthorizeModelCatalogWrite(ctx context.Context, token string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	return principal.AuthorizeModelCatalogWrite()
}

func (service *AccessService) AuthorizeTeamRead(ctx context.Context, token, teamID string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	return principal.AuthorizeTeamRead(teamID)
}

func (service *AccessService) AuthorizeAgentBuild(ctx context.Context, token, teamID string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	return principal.AuthorizeAgentBuild(teamID)
}

func (service *AccessService) AuthorizeTaskUse(ctx context.Context, token, teamID string) (domain.Actor, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return domain.Actor{}, err
	}
	return principal.AuthorizeTaskUse(teamID)
}

func (service *AccessService) ResolveReadScope(ctx context.Context, token string) (authz.ReadScope, error) {
	principal, err := service.authenticate(ctx, token)
	if err != nil {
		return authz.ReadScope{}, err
	}
	if principal.Disabled || principal.UserID == "" || principal.OrganizationID == "" {
		return authz.ReadScope{}, domain.ErrUnauthenticated
	}

	teams := make(map[string]struct{})
	allTeams := false
	for _, grant := range principal.Grants {
		switch grant.Role {
		case domain.PlatformAdministrator, domain.AgentBuilder, domain.AgentUser, domain.RunOperator:
		default:
			continue
		}
		if grant.TeamID == nil {
			allTeams = true
			continue
		}
		if teamID := strings.TrimSpace(*grant.TeamID); teamID != "" {
			teams[teamID] = struct{}{}
		}
	}
	if !allTeams && len(teams) == 0 {
		return authz.ReadScope{}, domain.ErrForbidden
	}
	teamIDs := make([]string, 0, len(teams))
	for teamID := range teams {
		teamIDs = append(teamIDs, teamID)
	}
	return authz.ReadScope{OrganizationID: principal.OrganizationID, TeamIDs: teamIDs, AllTeams: allTeams}, nil
}

func (service *AccessService) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	return service.authenticate(ctx, token)
}

func (service *AccessService) CurrentUser(ctx context.Context) (domain.Principal, error) {
	return service.authenticate(ctx, "")
}

func (service *AccessService) authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if principal, ok := PrincipalFromContext(ctx); ok {
		return authenticatedPrincipal(principal)
	}
	if strings.TrimSpace(token) == "" {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	identity, err := service.verifier.Verify(ctx, token)
	if errors.Is(err, domain.ErrUnauthenticated) {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return domain.Principal{}, fmt.Errorf("%w: verify Bearer Token", domain.ErrAuthenticationUnavailable)
	}
	if strings.TrimSpace(identity.Subject) == "" || strings.TrimSpace(identity.OrganizationSlug) == "" {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	principal, err := service.repository.FindPrincipal(ctx, identity)
	if errors.Is(err, domain.ErrUserNotFound) {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return domain.Principal{}, fmt.Errorf("resolve authenticated User: %w", err)
	}
	return authenticatedPrincipal(principal)
}

func authenticatedPrincipal(principal domain.Principal) (domain.Principal, error) {
	if principal.Disabled || strings.TrimSpace(principal.UserID) == "" || strings.TrimSpace(principal.OrganizationID) == "" {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	return principal, nil
}
