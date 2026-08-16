package identity_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	identityapplication "agent-platform/backend/internal/biz/identity/application"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	identityservice "agent-platform/backend/internal/service/identity"
)

type verifierFunc func(context.Context, string) (identitydomain.VerifiedIdentity, error)

func (function verifierFunc) Verify(ctx context.Context, token string) (identitydomain.VerifiedIdentity, error) {
	return function(ctx, token)
}

type repositoryStub struct {
	principal identitydomain.Principal
}

func (repositoryStub) FindRunScope(context.Context, string) (identitydomain.RunScope, error) {
	return identitydomain.RunScope{}, nil
}
func (repository repositoryStub) FindPrincipal(context.Context, identitydomain.VerifiedIdentity) (identitydomain.Principal, error) {
	return repository.principal, nil
}

func TestAuthenticationFilterIsPublicOnlyForHealthAndReadiness(t *testing.T) {
	access, err := identityapplication.NewAccessService(
		verifierFunc(func(context.Context, string) (identitydomain.VerifiedIdentity, error) {
			return identitydomain.VerifiedIdentity{Subject: "subject", OrganizationSlug: "org"}, nil
		}),
		repositoryStub{principal: identitydomain.Principal{UserID: "user", OrganizationID: "org"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	filter, err := identityservice.NewAuthenticationFilter(access)
	if err != nil {
		t.Fatal(err)
	}
	handler := filter(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/healthz" {
			if _, ok := identityapplication.PrincipalFromContext(request.Context()); !ok {
				t.Fatal("authenticated Principal missing from request Context")
			}
		}
		writer.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		path, authorization string
		want                int
	}{
		{path: "/healthz", want: http.StatusNoContent},
		{path: "/v1/runs", want: http.StatusUnauthorized},
		{path: "/v1/runs", authorization: "Bearer valid", want: http.StatusNoContent},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Authorization", test.authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("GET %s status=%d body=%s, want %d", test.path, response.Code, response.Body.String(), test.want)
		}
	}
}

func TestAuthenticationFilterDistinguishesInvalidTokenFromInfrastructureFailure(t *testing.T) {
	for _, test := range []struct {
		name       string
		verifyErr  error
		wantStatus int
	}{
		{name: "invalid", verifyErr: identitydomain.ErrUnauthenticated, wantStatus: http.StatusUnauthorized},
		{name: "unavailable", verifyErr: errors.New("jwks offline"), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			access, err := identityapplication.NewAccessService(verifierFunc(func(context.Context, string) (identitydomain.VerifiedIdentity, error) {
				return identitydomain.VerifiedIdentity{}, test.verifyErr
			}), repositoryStub{})
			if err != nil {
				t.Fatal(err)
			}
			filter, err := identityservice.NewAuthenticationFilter(access)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/v1/runs", nil)
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			filter(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
