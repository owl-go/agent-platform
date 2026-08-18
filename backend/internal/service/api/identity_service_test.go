package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	identityv1 "agent-platform/backend/api/identity/v1"
	identityapplication "agent-platform/backend/internal/biz/identity/application"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	identityservice "agent-platform/backend/internal/service/identity"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestGetCurrentUserReturnsAuthenticatedIdentityProjection(t *testing.T) {
	teamID := "00000000-0000-4000-8000-000000000003"
	principal := identitydomain.Principal{
		UserID: "00000000-0000-4000-8000-000000000002", Email: "user@example.test", DisplayName: "Platform User",
		OrganizationID: "00000000-0000-4000-8000-000000000001", OrganizationSlug: "acme", OrganizationName: "Acme",
		Grants: []identitydomain.Grant{{Role: identitydomain.PlatformAdministrator}, {TeamID: &teamID, Role: identitydomain.AgentBuilder}},
	}
	service := &GeneratedServices{dependencies: Dependencies{CurrentUserAccess: currentUserAccessStub{principal: principal}}}
	response, err := service.GetCurrentUser(identityapplication.WithPrincipal(context.Background(), principal), &identityv1.GetCurrentUserRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.UserId != principal.UserID || response.Email != principal.Email || response.DisplayName != principal.DisplayName {
		t.Fatalf("CurrentUser = %+v", response)
	}
	if response.Organization == nil || response.Organization.Id != principal.OrganizationID || response.Organization.Slug != "acme" || response.Organization.Name != "Acme" {
		t.Fatalf("Organization = %+v", response.Organization)
	}
	if len(response.RoleGrants) != 2 || response.RoleGrants[0].TeamId != nil || response.RoleGrants[0].Role != string(identitydomain.PlatformAdministrator) || response.RoleGrants[1].GetTeamId() != teamID {
		t.Fatalf("Role Grants = %+v", response.RoleGrants)
	}
}

func TestCurrentUserHTTPRouteUsesAuthenticatedPrincipal(t *testing.T) {
	principal := identitydomain.Principal{
		UserID: "00000000-0000-4000-8000-000000000002", Email: "user@example.test", DisplayName: "Platform User",
		OrganizationID: "00000000-0000-4000-8000-000000000001", OrganizationSlug: "acme", OrganizationName: "Acme",
		Grants: []identitydomain.Grant{{Role: identitydomain.AgentUser}},
	}
	access, err := identityapplication.NewAccessService(
		identityVerifierStub{identity: identitydomain.VerifiedIdentity{Subject: "subject", OrganizationSlug: "acme"}},
		identityRepositoryHTTPStub{principal: principal},
	)
	if err != nil {
		t.Fatal(err)
	}
	authentication, err := identityservice.NewAuthenticationFilter(access)
	if err != nil {
		t.Fatal(err)
	}
	server := kratoshttp.NewServer(kratoshttp.Filter(authentication))
	service := &GeneratedServices{dependencies: Dependencies{CurrentUserAccess: access}}
	identityv1.RegisterIdentityServiceHTTPServer(server, service)

	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/me = (%d, %q)", response.Code, response.Body.String())
	}
	var current identityv1.CurrentUser
	if err := json.Unmarshal(response.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current.UserId != principal.UserID || current.Organization.GetSlug() != "acme" || len(current.RoleGrants) != 1 {
		t.Fatalf("CurrentUser = %+v", current)
	}
}

type currentUserAccessStub struct {
	principal identitydomain.Principal
	err       error
}

func (stub currentUserAccessStub) CurrentUser(context.Context) (identitydomain.Principal, error) {
	return stub.principal, stub.err
}

type identityVerifierStub struct {
	identity identitydomain.VerifiedIdentity
}

func (stub identityVerifierStub) Verify(context.Context, string) (identitydomain.VerifiedIdentity, error) {
	return stub.identity, nil
}

type identityRepositoryHTTPStub struct {
	principal identitydomain.Principal
}

func (stub identityRepositoryHTTPStub) FindPrincipal(context.Context, identitydomain.VerifiedIdentity) (identitydomain.Principal, error) {
	return stub.principal, nil
}

func (identityRepositoryHTTPStub) FindRunScope(context.Context, string) (identitydomain.RunScope, error) {
	return identitydomain.RunScope{}, identitydomain.ErrRunNotFound
}
