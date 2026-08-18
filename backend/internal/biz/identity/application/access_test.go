package application

import (
	"context"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/identity/domain"
)

type verifierFunc func(context.Context, string) (domain.VerifiedIdentity, error)

func (function verifierFunc) Verify(ctx context.Context, token string) (domain.VerifiedIdentity, error) {
	return function(ctx, token)
}

type identityRepositoryStub struct {
	principal    domain.Principal
	principalErr error
	scope        domain.RunScope
	scopeErr     error
}

func (repository identityRepositoryStub) FindPrincipal(context.Context, domain.VerifiedIdentity) (domain.Principal, error) {
	return repository.principal, repository.principalErr
}
func (repository identityRepositoryStub) FindRunScope(context.Context, string) (domain.RunScope, error) {
	return repository.scope, repository.scopeErr
}

func TestAccessServiceAuthorizesVerifiedPrincipal(t *testing.T) {
	service, err := NewAccessService(
		verifierFunc(func(_ context.Context, token string) (domain.VerifiedIdentity, error) {
			if token != "valid-token" {
				t.Fatalf("token = %q", token)
			}
			return domain.VerifiedIdentity{Subject: "oidc-subject", OrganizationSlug: "acme"}, nil
		}),
		identityRepositoryStub{
			principal: domain.Principal{UserID: "user-1", OrganizationID: "org-1", Grants: []domain.Grant{{Role: domain.RunOperator}}},
			scope:     domain.RunScope{OrganizationID: "org-1", TeamID: "team-1"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.AuthorizeRunRead(context.Background(), "valid-token", "run-1"); err != nil {
		t.Fatalf("AuthorizeRunRead(): %v", err)
	}
	scope, err := service.ResolveReadScope(context.Background(), "valid-token")
	if err != nil || scope.OrganizationID != "org-1" || !scope.AllTeams {
		t.Fatalf("ResolveReadScope() = (%+v, %v)", scope, err)
	}
}

func TestAccessServiceFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		verifier   verifierFunc
		repository identityRepositoryStub
		want       error
	}{
		{name: "invalid token", verifier: func(context.Context, string) (domain.VerifiedIdentity, error) {
			return domain.VerifiedIdentity{}, domain.ErrUnauthenticated
		}, want: domain.ErrUnauthenticated},
		{name: "authentication unavailable", verifier: func(context.Context, string) (domain.VerifiedIdentity, error) {
			return domain.VerifiedIdentity{}, errors.New("jwks offline")
		}, want: domain.ErrAuthenticationUnavailable},
		{name: "unknown user", verifier: func(context.Context, string) (domain.VerifiedIdentity, error) {
			return domain.VerifiedIdentity{Subject: "subject", OrganizationSlug: "acme"}, nil
		}, repository: identityRepositoryStub{principalErr: domain.ErrUserNotFound}, want: domain.ErrUnauthenticated},
		{name: "unknown run", verifier: func(context.Context, string) (domain.VerifiedIdentity, error) {
			return domain.VerifiedIdentity{Subject: "subject", OrganizationSlug: "acme"}, nil
		}, repository: identityRepositoryStub{principal: domain.Principal{UserID: "u", OrganizationID: "o"}, scopeErr: domain.ErrRunNotFound}, want: domain.ErrForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := NewAccessService(test.verifier, test.repository)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.AuthorizeRunRead(context.Background(), "token", "run"); !errors.Is(err, test.want) {
				t.Fatalf("AuthorizeRunRead() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestAccessServiceRejectsDisabledPrincipalAtAuthenticationBoundary(t *testing.T) {
	service, err := NewAccessService(
		verifierFunc(func(context.Context, string) (domain.VerifiedIdentity, error) {
			return domain.VerifiedIdentity{Subject: "subject", OrganizationSlug: "acme"}, nil
		}),
		identityRepositoryStub{principal: domain.Principal{UserID: "user-1", OrganizationID: "org-1", Disabled: true}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(context.Background(), "token"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("Authenticate() error = %v, want ErrUnauthenticated", err)
	}
	ctx := WithPrincipal(context.Background(), domain.Principal{UserID: "user-1", OrganizationID: "org-1", Disabled: true})
	if _, err := service.CurrentUser(ctx); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("CurrentUser() error = %v, want ErrUnauthenticated", err)
	}
}
