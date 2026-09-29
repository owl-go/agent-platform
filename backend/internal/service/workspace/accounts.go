package workspace

import (
	"context"
	"sort"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) GetCurrentUser(ctx context.Context, _ *workspacev1.GetCurrentUserRequest) (*workspacev1.CurrentUser, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	settings, settingsErr := service.workspace.Repository().GetSettings(ctx, principal.UserID)
	timezone := "Asia/Shanghai"
	if settingsErr == nil {
		timezone = settings.Timezone
	}
	balance, balanceErr := service.credits.Balance(ctx, principal.UserID, timezone)
	if balanceErr != nil {
		return nil, publicError(balanceErr)
	}
	policy, policyErr := service.credits.Policy(ctx)
	if policyErr != nil {
		return nil, publicError(policyErr)
	}
	return &workspacev1.CurrentUser{Id: principal.UserID, Username: principal.Username, Email: principal.Email, DisplayName: principal.DisplayName, Administrator: principal.Administrator, BootstrapAdministrator: principal.BootstrapAdministrator, ResourcePublisher: principal.ResourcePublisher, Groups: identityGroupResponses(principal.Groups), SettingsReady: settingsErr == nil, CreditBalance: creditBalanceResponse(balance, policy)}, nil
}

func (service *Service) ListUsers(ctx context.Context, _ *workspacev1.ListUsersRequest) (*workspacev1.ListUsersResponse, error) {
	users, err := service.accounts.ListUsers(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	items := make([]*workspacev1.UserAccount, 0, len(users))
	policy, policyErr := service.credits.Policy(ctx)
	if policyErr != nil {
		return nil, publicError(policyErr)
	}
	for _, user := range users {
		response := userResponse(user)
		balance, balanceErr := service.credits.Balance(ctx, user.ID, "")
		if balanceErr != nil {
			return nil, publicError(balanceErr)
		}
		response.CreditBalance = creditBalanceResponse(balance, policy)
		items = append(items, response)
	}
	return &workspacev1.ListUsersResponse{Items: items}, nil
}

func (service *Service) CreateUser(ctx context.Context, request *workspacev1.CreateUserRequest) (*workspacev1.CreateUserResponse, error) {
	user, password, err := service.accounts.CreateUser(ctx, accountdomain.NewUser{Username: request.Username, Email: request.Email, DisplayName: request.DisplayName})
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.CreateUserResponse{User: userResponse(user), TemporaryPassword: password}, nil
}

func (service *Service) SetUserEnabled(ctx context.Context, request *workspacev1.SetUserEnabledRequest) (*workspacev1.UserAccount, error) {
	user, err := service.accounts.SetEnabled(ctx, request.UserId, request.Enabled, request.ExpectedVersion, request.GetReason())
	if err != nil {
		return nil, publicError(err)
	}
	return userResponse(user), nil
}

func (service *Service) ResetUserPassword(ctx context.Context, request *workspacev1.ResetUserPasswordRequest) (*workspacev1.ResetUserPasswordResponse, error) {
	password, err := service.accounts.ResetPassword(ctx, request.UserId)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ResetUserPasswordResponse{TemporaryPassword: password}, nil
}

func (service *Service) SetUserRoles(ctx context.Context, request *workspacev1.SetUserRolesRequest) (*workspacev1.UserAccount, error) {
	user, err := service.accounts.SetRoles(ctx, request.GetUserId(), request.GetAdministrator(), request.GetResourcePublisher(), request.GetExpectedVersion(), request.GetReason())
	if err != nil {
		return nil, publicError(err)
	}
	return userResponse(user), nil
}

func (service *Service) ListIdentityGroups(ctx context.Context, _ *workspacev1.ListIdentityGroupsRequest) (*workspacev1.ListIdentityGroupsResponse, error) {
	groups, err := service.accounts.ListIdentityGroups(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListIdentityGroupsResponse{Items: identityGroupResponses(groups)}, nil
}

func (service *Service) SyncIdentityGroups(ctx context.Context, _ *workspacev1.SyncIdentityGroupsRequest) (*workspacev1.ListIdentityGroupsResponse, error) {
	groups, err := service.accounts.SyncIdentityGroups(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListIdentityGroupsResponse{Items: identityGroupResponses(groups)}, nil
}

func (service *Service) UpdateIdentityGroupBudget(ctx context.Context, request *workspacev1.UpdateIdentityGroupBudgetRequest) (*workspacev1.IdentityGroup, error) {
	group, err := service.accounts.UpdateIdentityGroupBudget(ctx, request.GetGroupId(), request.DailyCreditLimitHundredths, request.GetExpectedVersion(), request.GetReason())
	if err != nil {
		return nil, publicError(err)
	}
	return identityGroupResponse(group), nil
}

func (service *Service) ListGovernanceAuditEvents(ctx context.Context, request *workspacev1.ListGovernanceAuditEventsRequest) (*workspacev1.ListGovernanceAuditEventsResponse, error) {
	events, err := service.accounts.ListGovernanceAuditEvents(ctx, int(request.GetLimit()))
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListGovernanceAuditEventsResponse{Items: make([]*workspacev1.GovernanceAuditEvent, 0, len(events))}
	for _, event := range events {
		keys := make([]string, 0, len(event.Detail))
		for key := range event.Detail {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		metrics := make([]*workspacev1.GovernanceAuditMetric, 0, len(keys))
		for _, key := range keys {
			metrics = append(metrics, &workspacev1.GovernanceAuditMetric{Key: key, Value: event.Detail[key]})
		}
		response.Items = append(response.Items, &workspacev1.GovernanceAuditEvent{Id: event.ID, ActorUserId: event.ActorUserID, Action: event.Action, TargetType: event.TargetType, TargetId: event.TargetID, Reason: event.Reason, Metrics: metrics, OccurredAt: timestamppb.New(event.OccurredAt)})
	}
	return response, nil
}

func (service *Service) TransferGroupResources(ctx context.Context, request *workspacev1.TransferGroupResourcesRequest) (*workspacev1.TransferGroupResourcesResponse, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	count, err := service.workspace.Repository().TransferGroupResources(ctx, administrator.UserID, request.GetGroupId(), request.GetFromUserId(), request.GetToUserId(), request.GetReason())
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.TransferGroupResourcesResponse{KnowledgeBaseCount: count}, nil
}

func userResponse(user accountdomain.User) *workspacev1.UserAccount {
	return &workspacev1.UserAccount{Id: user.ID, Username: user.Username, Email: user.Email, DisplayName: user.DisplayName, Administrator: user.Administrator, BootstrapAdministrator: user.BootstrapAdministrator, ResourcePublisher: user.ResourcePublisher, Groups: identityGroupResponses(user.Groups), Enabled: user.Enabled, CreatedAt: timestamppb.New(user.CreatedAt), Version: user.Version}
}

func identityGroupResponses(groups []accountdomain.IdentityGroup) []*workspacev1.IdentityGroup {
	items := make([]*workspacev1.IdentityGroup, 0, len(groups))
	for _, group := range groups {
		items = append(items, identityGroupResponse(group))
	}
	return items
}

func identityGroupResponse(group accountdomain.IdentityGroup) *workspacev1.IdentityGroup {
	return &workspacev1.IdentityGroup{Id: group.ID, ExternalId: group.ExternalID, Name: group.Name, Path: group.Path, Department: group.Department, DailyCreditLimitHundredths: group.DailyCreditLimitHundredths, MemberCount: group.MemberCount, LastSyncedAt: timestamppb.New(group.LastSyncedAt), Version: group.Version}
}
