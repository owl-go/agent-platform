package domain

import (
	"errors"
	"testing"
)

func TestPrincipalAuthorizesOrganizationAndTeamScopedRoles(t *testing.T) {
	teamA := "team-a"
	tests := []struct {
		name      string
		principal Principal
		scope     RunScope
		wantError error
	}{
		{name: "organization grant", principal: Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{Role: RunOperator}}}, scope: RunScope{OrganizationID: "o", TeamID: "team-b"}},
		{name: "matching team", principal: Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{TeamID: &teamA, Role: AgentUser}}}, scope: RunScope{OrganizationID: "o", TeamID: teamA}},
		{name: "other organization", principal: Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{Role: PlatformAdministrator}}}, scope: RunScope{OrganizationID: "other", TeamID: teamA}, wantError: ErrForbidden},
		{name: "other team", principal: Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{TeamID: &teamA, Role: AgentBuilder}}}, scope: RunScope{OrganizationID: "o", TeamID: "team-b"}, wantError: ErrForbidden},
		{name: "no grants", principal: Principal{UserID: "u", OrganizationID: "o"}, scope: RunScope{OrganizationID: "o", TeamID: teamA}, wantError: ErrForbidden},
		{name: "disabled", principal: Principal{UserID: "u", OrganizationID: "o", Disabled: true, Grants: []Grant{{Role: RunOperator}}}, scope: RunScope{OrganizationID: "o", TeamID: teamA}, wantError: ErrUnauthenticated},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.principal.AuthorizeRunRead(test.scope)
			if !errors.Is(err, test.wantError) || test.wantError == nil && err != nil {
				t.Fatalf("AuthorizeRunRead() error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestPrincipalRestrictsRuntimeImageWritesToOrganizationAdministrators(t *testing.T) {
	team := "team-a"
	if err := (Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{Role: PlatformAdministrator}}}).AuthorizeRuntimeImageWrite(); err != nil {
		t.Fatalf("organization administrator write: %v", err)
	}
	for _, principal := range []Principal{
		{UserID: "u", OrganizationID: "o", Grants: []Grant{{TeamID: &team, Role: PlatformAdministrator}}},
		{UserID: "u", OrganizationID: "o", Grants: []Grant{{Role: AgentBuilder}}},
		{UserID: "u", OrganizationID: "o"},
	} {
		if err := principal.AuthorizeRuntimeImageWrite(); !errors.Is(err, ErrForbidden) {
			t.Fatalf("write error = %v for %+v", err, principal)
		}
	}
	if err := (Principal{UserID: "u", OrganizationID: "o", Grants: []Grant{{TeamID: &team, Role: AgentUser}}}).AuthorizeRuntimeImageRead(); err != nil {
		t.Fatalf("team user read: %v", err)
	}
}

func TestPrincipalScopesAgentBuildToBuildersAndOrganizationAdministrators(t *testing.T) {
	teamA, teamB := "team-a", "team-b"
	principals := []Principal{
		{UserID: "admin", OrganizationID: "org", Grants: []Grant{{Role: PlatformAdministrator}}},
		{UserID: "builder", OrganizationID: "org", Grants: []Grant{{Role: AgentBuilder}}},
		{UserID: "team-builder", OrganizationID: "org", Grants: []Grant{{TeamID: &teamA, Role: AgentBuilder}}},
	}
	for _, principal := range principals {
		if _, err := principal.AuthorizeAgentBuild(teamA); err != nil {
			t.Fatalf("AuthorizeAgentBuild(%+v): %v", principal, err)
		}
	}
	if _, err := principals[2].AuthorizeAgentBuild(teamB); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-Team Agent Builder error = %v", err)
	}
	for _, role := range []Role{AgentUser, RunOperator} {
		if _, err := (Principal{UserID: "user", OrganizationID: "org", Grants: []Grant{{Role: role}}}).AuthorizeAgentBuild(teamA); !errors.Is(err, ErrForbidden) {
			t.Fatalf("Role %q Agent Build error = %v", role, err)
		}
	}
	if actor, err := (Principal{UserID: "user", OrganizationID: "org", Grants: []Grant{{TeamID: &teamA, Role: AgentUser}}}).AuthorizeTeamRead(teamA); err != nil || actor.TeamID != teamA {
		t.Fatalf("AuthorizeTeamRead() = (%+v, %v)", actor, err)
	}
}

func TestPrincipalAuthorizesTaskUseButKeepsRunOperatorReadOnly(t *testing.T) {
	team := "team-a"
	for _, role := range []Role{PlatformAdministrator, AgentBuilder, AgentUser} {
		principal := Principal{UserID: "user", OrganizationID: "org", Grants: []Grant{{TeamID: &team, Role: role}}}
		if _, err := principal.AuthorizeTaskUse(team); err != nil {
			t.Fatalf("Role %q task use: %v", role, err)
		}
	}
	operator := Principal{UserID: "operator", OrganizationID: "org", Grants: []Grant{{TeamID: &team, Role: RunOperator}}}
	if _, err := operator.AuthorizeTaskUse(team); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Run Operator task use error = %v", err)
	}
}

func TestPrincipalSeparatesRunCollaborationFromOperatorKill(t *testing.T) {
	team := "team-a"
	scope := RunScope{OrganizationID: "org", TeamID: team}
	user := Principal{UserID: "user", OrganizationID: "org", Grants: []Grant{{TeamID: &team, Role: AgentUser}}}
	for _, action := range []string{"interrupt", "resume", "cancel"} {
		if _, err := user.AuthorizeRunControl(scope, action); err != nil {
			t.Fatalf("Agent User %s: %v", action, err)
		}
	}
	if _, err := user.AuthorizeRunControl(scope, "kill"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Agent User kill error = %v", err)
	}
	operator := Principal{UserID: "operator", OrganizationID: "org", Grants: []Grant{{TeamID: &team, Role: RunOperator}}}
	for _, action := range []string{"interrupt", "cancel", "kill"} {
		if _, err := operator.AuthorizeRunControl(scope, action); err != nil {
			t.Fatalf("Run Operator %s: %v", action, err)
		}
	}
	if _, err := operator.AuthorizeRunControl(scope, "resume"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Run Operator resume error = %v", err)
	}
	for _, action := range []string{"approval_request", "approval_decide"} {
		if _, err := user.AuthorizeRunControl(scope, action); err != nil {
			t.Fatalf("Agent User %s: %v", action, err)
		}
		if _, err := operator.AuthorizeRunControl(scope, action); !errors.Is(err, ErrForbidden) {
			t.Fatalf("Run Operator %s error = %v", action, err)
		}
	}
}
