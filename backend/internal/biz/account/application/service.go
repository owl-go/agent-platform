package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

type TokenVerifier interface {
	Verify(context.Context, string) (domain.VerifiedIdentity, error)
}

type Repository interface {
	EnsureAdministrator(context.Context, domain.User) (domain.User, error)
	FindPrincipal(context.Context, string) (domain.Principal, error)
	ListUsers(context.Context) ([]domain.User, error)
	CreateUser(context.Context, domain.User) (domain.User, error)
	SetEnabled(context.Context, string, string, bool, int64, string) (domain.User, error)
	SetRoles(context.Context, string, string, bool, bool, int64, string) (domain.User, error)
	ReplaceIdentityGroups(context.Context, string, []domain.IdentityGroupSnapshot, time.Time) ([]domain.IdentityGroup, error)
	ListIdentityGroups(context.Context) ([]domain.IdentityGroup, error)
	UpdateIdentityGroupBudget(context.Context, string, string, *int64, int64, string) (domain.IdentityGroup, error)
	ListGovernanceAuditEvents(context.Context, int) ([]domain.GovernanceAuditEvent, error)
}

type IdentityProvider interface {
	CreateUser(context.Context, domain.NewUser) (subject, temporaryPassword string, err error)
	SetEnabled(context.Context, string, bool) error
	ResetPassword(context.Context, string) (string, error)
	ListGroups(context.Context) ([]domain.IdentityGroupSnapshot, error)
}

type Service struct {
	verifier TokenVerifier
	repo     Repository
	provider IdentityProvider
}

type principalContextKey struct{}

func New(verifier TokenVerifier, repo Repository, provider IdentityProvider) (*Service, error) {
	if verifier == nil || repo == nil || provider == nil {
		return nil, fmt.Errorf("Token Verifier, account Repository, and Identity Provider are required")
	}
	return &Service{verifier: verifier, repo: repo, provider: provider}, nil
}

func (service *Service) EnsureAdministrator(ctx context.Context, administrator domain.User) (domain.User, error) {
	if !administrator.Administrator || strings.TrimSpace(administrator.OIDCSubject) == "" {
		return domain.User{}, fmt.Errorf("bootstrap Administrator identity is invalid")
	}
	return service.repo.EnsureAdministrator(ctx, administrator)
}

func WithPrincipal(ctx context.Context, principal domain.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (domain.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(domain.Principal)
	return principal, ok
}

func (service *Service) Authenticate(ctx context.Context, token string) (domain.Principal, error) {
	if principal, ok := PrincipalFromContext(ctx); ok {
		return principal, principal.Validate()
	}
	if strings.TrimSpace(token) == "" {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	identity, err := service.verifier.Verify(ctx, token)
	if err != nil || strings.TrimSpace(identity.Subject) == "" {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	principal, err := service.repo.FindPrincipal(ctx, identity.Subject)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return domain.Principal{}, fmt.Errorf("resolve User: %w", err)
	}
	return principal, principal.Validate()
}

func (service *Service) Current(ctx context.Context) (domain.Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return domain.Principal{}, domain.ErrUnauthenticated
	}
	return principal, principal.Validate()
}

func (service *Service) ListUsers(ctx context.Context) ([]domain.User, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return nil, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return nil, err
	}
	return service.repo.ListUsers(ctx)
}

func (service *Service) CreateUser(ctx context.Context, input domain.NewUser) (domain.User, string, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return domain.User{}, "", err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return domain.User{}, "", err
	}
	if err := input.Validate(); err != nil {
		return domain.User{}, "", err
	}
	subject, password, err := service.provider.CreateUser(ctx, input)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("create Identity Provider User: %w", err)
	}
	created, err := service.repo.CreateUser(ctx, domain.User{
		OIDCSubject: subject, Username: strings.TrimSpace(input.Username), Email: strings.TrimSpace(input.Email),
		DisplayName: strings.TrimSpace(input.DisplayName), Enabled: true,
	})
	if err != nil {
		_ = service.provider.SetEnabled(context.WithoutCancel(ctx), subject, false)
		return domain.User{}, "", err
	}
	return created, password, nil
}

func (service *Service) SetEnabled(ctx context.Context, userID string, enabled bool, expectedVersion int64, reason string) (domain.User, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return domain.User{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return domain.User{}, fmt.Errorf("User status change requires a reason")
	}
	users, err := service.repo.ListUsers(ctx)
	if err != nil {
		return domain.User{}, err
	}
	var target domain.User
	for _, user := range users {
		if user.ID == userID {
			target = user
			break
		}
	}
	if target.ID == "" {
		return domain.User{}, domain.ErrNotFound
	}
	if target.Administrator {
		return domain.User{}, fmt.Errorf("Administrator must be demoted before the account can be disabled")
	}
	if err := service.provider.SetEnabled(ctx, target.OIDCSubject, enabled); err != nil {
		return domain.User{}, fmt.Errorf("update Identity Provider User: %w", err)
	}
	updated, err := service.repo.SetEnabled(ctx, principal.UserID, userID, enabled, expectedVersion, reason)
	if err != nil {
		_ = service.provider.SetEnabled(context.WithoutCancel(ctx), target.OIDCSubject, target.Enabled)
		return domain.User{}, err
	}
	return updated, nil
}

func (service *Service) ResetPassword(ctx context.Context, userID string) (string, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return "", err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return "", err
	}
	users, err := service.repo.ListUsers(ctx)
	if err != nil {
		return "", err
	}
	for _, user := range users {
		if user.ID == userID {
			return service.provider.ResetPassword(ctx, user.OIDCSubject)
		}
	}
	return "", domain.ErrNotFound
}

func (service *Service) SetRoles(ctx context.Context, userID string, administrator, resourcePublisher bool, expectedVersion int64, reason string) (domain.User, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return domain.User{}, err
	}
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(userID) == "" || expectedVersion < 1 || reason == "" || len(reason) > 500 {
		return domain.User{}, fmt.Errorf("governance role change requires a User, version, and reason")
	}
	return service.repo.SetRoles(ctx, principal.UserID, userID, administrator, resourcePublisher, expectedVersion, reason)
}

func (service *Service) SyncIdentityGroups(ctx context.Context) ([]domain.IdentityGroup, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return nil, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return nil, err
	}
	groups, err := service.provider.ListGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("read identity groups: %w", err)
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if err := group.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seen[group.ExternalID]; exists {
			return nil, fmt.Errorf("identity source returned duplicate group %q", group.ExternalID)
		}
		seen[group.ExternalID] = struct{}{}
	}
	return service.repo.ReplaceIdentityGroups(ctx, principal.UserID, groups, time.Now().UTC())
}

func (service *Service) ListIdentityGroups(ctx context.Context) ([]domain.IdentityGroup, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return nil, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return nil, err
	}
	return service.repo.ListIdentityGroups(ctx)
}

func (service *Service) UpdateIdentityGroupBudget(ctx context.Context, groupID string, dailyLimit *int64, expectedVersion int64, reason string) (domain.IdentityGroup, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return domain.IdentityGroup{}, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return domain.IdentityGroup{}, err
	}
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(groupID) == "" || expectedVersion < 1 || reason == "" || len(reason) > 500 || (dailyLimit != nil && *dailyLimit < 0) {
		return domain.IdentityGroup{}, fmt.Errorf("group budget requires a Group, non-negative limit, version, and reason")
	}
	return service.repo.UpdateIdentityGroupBudget(ctx, principal.UserID, groupID, dailyLimit, expectedVersion, reason)
}

func (service *Service) ListGovernanceAuditEvents(ctx context.Context, limit int) ([]domain.GovernanceAuditEvent, error) {
	principal, err := service.Current(ctx)
	if err != nil {
		return nil, err
	}
	if err := principal.RequireAdministrator(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	return service.repo.ListGovernanceAuditEvents(ctx, limit)
}
