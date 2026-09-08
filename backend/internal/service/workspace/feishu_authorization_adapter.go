package workspace

import (
	"context"
	"errors"
	"net/url"

	"agent-platform/backend/internal/cliconnector"
	"agent-platform/backend/internal/feishucli"
)

type feishuAuthorizationAdapter struct {
	registrar feishuApplicationRegistrar
}

func (adapter feishuAuthorizationAdapter) BeginSetup(ctx context.Context) (cliconnector.SetupChallenge, error) {
	registration, err := adapter.registrar.Begin(ctx)
	if err != nil {
		return cliconnector.SetupChallenge{}, err
	}
	return cliconnector.SetupChallenge{ActionURL: registration.ActionURL, ExpiresAt: registration.ExpiresAt, Continuation: []byte(registration.DeviceCode)}, nil
}

func (adapter feishuAuthorizationAdapter) PollSetup(ctx context.Context, continuation []byte) (cliconnector.SetupResult, error) {
	application, err := adapter.registrar.Poll(ctx, string(continuation))
	if err != nil {
		return cliconnector.SetupResult{}, mapFeishuAuthorizationError(err)
	}
	return cliconnector.SetupResult{
		CredentialSlots: map[string]string{"app_id": application.AppID, "app_secret": application.AppSecret},
		DisplayName:     application.UserName, ManagementURL: "https://open.feishu.cn/app/" + url.PathEscape(application.AppID),
	}, nil
}

func (adapter feishuAuthorizationAdapter) BeginAuthorization(ctx context.Context, setup map[string]string, permissions []string) (cliconnector.AuthorizationChallenge, error) {
	flow, err := adapter.registrar.BeginAuthorization(ctx, setup["app_id"], setup["app_secret"], permissions)
	if err != nil {
		return cliconnector.AuthorizationChallenge{}, err
	}
	return cliconnector.AuthorizationChallenge{ActionURL: flow.ActionURL, ExpiresAt: flow.ExpiresAt, Permissions: append([]string(nil), flow.Scopes...), Continuation: []byte(flow.DeviceCode)}, nil
}

func (adapter feishuAuthorizationAdapter) PollAuthorization(ctx context.Context, setup map[string]string, continuation []byte) (cliconnector.AuthorizationResult, error) {
	result, err := adapter.registrar.PollAuthorization(ctx, setup["app_id"], setup["app_secret"], string(continuation))
	if err != nil {
		return cliconnector.AuthorizationResult{}, mapFeishuAuthorizationError(err)
	}
	return cliconnector.AuthorizationResult{
		ExternalIdentityID: result.ExternalID, ExternalDisplayName: result.DisplayName,
		Permissions:     append([]string(nil), result.Scopes...),
		CredentialSlots: map[string]string{"access_token": result.AccessToken, "refresh_token": result.RefreshToken},
		ExpiresAt:       result.ExpiresAt,
	}, nil
}

func mapFeishuAuthorizationError(err error) error {
	switch {
	case errors.Is(err, feishucli.ErrPending):
		return cliconnector.ErrAuthorizationPending
	case errors.Is(err, feishucli.ErrDenied):
		return cliconnector.ErrAuthorizationDenied
	case errors.Is(err, feishucli.ErrExpired):
		return cliconnector.ErrAuthorizationExpired
	default:
		return err
	}
}

var _ cliconnector.AuthorizationAdapter = feishuAuthorizationAdapter{}
