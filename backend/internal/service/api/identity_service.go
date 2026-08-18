package api

import (
	"context"

	identityv1 "agent-platform/backend/api/identity/v1"
)

func (service *GeneratedServices) GetCurrentUser(ctx context.Context, _ *identityv1.GetCurrentUserRequest) (*identityv1.CurrentUser, error) {
	principal, err := service.dependencies.CurrentUserAccess.CurrentUser(ctx)
	if err != nil {
		return nil, mapAuthorizationError(err, "identity_access_denied")
	}
	grants := make([]*identityv1.RoleGrant, 0, len(principal.Grants))
	for _, grant := range principal.Grants {
		item := &identityv1.RoleGrant{Role: string(grant.Role)}
		if grant.TeamID != nil {
			teamID := *grant.TeamID
			item.TeamId = &teamID
		}
		grants = append(grants, item)
	}
	teams := make([]*identityv1.Team, 0, len(principal.Teams))
	for _, team := range principal.Teams {
		teams = append(teams, &identityv1.Team{Id: team.ID, Slug: team.Slug, Name: team.Name})
	}
	return &identityv1.CurrentUser{
		UserId: principal.UserID, Email: principal.Email, DisplayName: principal.DisplayName,
		Organization: &identityv1.Organization{Id: principal.OrganizationID, Slug: principal.OrganizationSlug, Name: principal.OrganizationName},
		RoleGrants:   grants,
		Teams:        teams,
	}, nil
}
