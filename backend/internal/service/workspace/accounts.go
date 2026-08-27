package workspace

import (
	"context"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) GetCurrentUser(ctx context.Context, _ *workspacev1.GetCurrentUserRequest) (*workspacev1.CurrentUser, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	_, settingsErr := service.workspace.Repository().GetSettings(ctx, principal.UserID)
	return &workspacev1.CurrentUser{Id: principal.UserID, Username: principal.Username, Email: principal.Email, DisplayName: principal.DisplayName, Administrator: principal.Administrator, SettingsReady: settingsErr == nil}, nil
}

func (service *Service) ListUsers(ctx context.Context, _ *workspacev1.ListUsersRequest) (*workspacev1.ListUsersResponse, error) {
	users, err := service.accounts.ListUsers(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	items := make([]*workspacev1.UserAccount, 0, len(users))
	for _, user := range users {
		items = append(items, userResponse(user))
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
	user, err := service.accounts.SetEnabled(ctx, request.UserId, request.Enabled, request.ExpectedVersion)
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

func userResponse(user accountdomain.User) *workspacev1.UserAccount {
	return &workspacev1.UserAccount{Id: user.ID, Username: user.Username, Email: user.Email, DisplayName: user.DisplayName, Administrator: user.Administrator, Enabled: user.Enabled, CreatedAt: timestamppb.New(user.CreatedAt), Version: user.Version}
}
