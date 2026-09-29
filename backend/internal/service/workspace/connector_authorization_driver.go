package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/dingtalkcli"
	"agent-platform/backend/internal/feishucli"
	"agent-platform/backend/internal/secretcrypto"
)

var (
	errConnectorAuthorizationPending = errors.New("Connector authorization is pending")
	errConnectorAuthorizationDenied  = errors.New("Connector authorization was denied")
	errConnectorAuthorizationExpired = errors.New("Connector authorization expired")
)

type connectorAuthorizationChallenge struct {
	State     string
	ActionURL string
	ExpiresAt time.Time
	Scopes    []string
}

type connectorAuthorizationGrant struct {
	ExternalID, DisplayName     string
	AccessToken, RefreshToken   string
	ClientID                    string
	Scopes                      []string
	ExpiresAt, RefreshExpiresAt time.Time
}

type dingtalkConnectorAuthorizationDriver struct{ client *dingtalkcli.Client }

func (driver dingtalkConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}

func (driver dingtalkConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, _ []string) (connectorAuthorizationChallenge, error) {
	value, err := driver.client.Begin(ctx)
	if err != nil {
		return connectorAuthorizationChallenge{}, err
	}
	return connectorAuthorizationChallenge{State: value.State, ActionURL: value.ActionURL, ExpiresAt: value.ExpiresAt}, nil
}

func (driver dingtalkConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	value, err := driver.client.Poll(ctx, state)
	return dingtalkAuthorizationGrant(value), translateDingTalkAuthorizationError(err)
}

func (driver dingtalkConnectorAuthorizationDriver) Refresh(ctx context.Context, clientID, _, token string) (connectorAuthorizationGrant, error) {
	value, err := driver.client.Refresh(ctx, clientID, token)
	return dingtalkAuthorizationGrant(value), err
}

func dingtalkAuthorizationGrant(value dingtalkcli.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{ExternalID: value.ExternalID, DisplayName: value.DisplayName, AccessToken: value.AccessToken, RefreshToken: value.RefreshToken, ClientID: value.ClientID, ExpiresAt: value.ExpiresAt, RefreshExpiresAt: value.RefreshExpiresAt}
}

func translateDingTalkAuthorizationError(err error) error {
	switch {
	case errors.Is(err, dingtalkcli.ErrPending):
		return errConnectorAuthorizationPending
	case errors.Is(err, dingtalkcli.ErrDenied):
		return errConnectorAuthorizationDenied
	case errors.Is(err, dingtalkcli.ErrExpired):
		return errConnectorAuthorizationExpired
	default:
		return err
	}
}

// This seam keeps the User-owned flow and credential lifecycle in the platform.
// A reviewed provider adapter owns only its external authorization protocol.
type interactiveConnectorAuthorizationDriver interface {
	Application(context.Context, string, string) (string, string, error)
	Begin(context.Context, string, string, []string) (connectorAuthorizationChallenge, error)
	Poll(context.Context, string, string, string) (connectorAuthorizationGrant, error)
	Refresh(context.Context, string, string, string) (connectorAuthorizationGrant, error)
}

type feishuConnectorAuthorizationDriver struct {
	registrar  feishuApplicationRegistrar
	repository connectorPackageRepository
	box        *secretcrypto.Box
}

func (driver feishuConnectorAuthorizationDriver) Application(ctx context.Context, ownerID, installationID string) (string, string, error) {
	application, err := driver.repository.GetConnectorProviderApplication(ctx, ownerID, installationID)
	if err != nil {
		return "", "", err
	}
	appID, err := driver.box.Decrypt(application.AppIDCiphertext, feishuApplicationAAD(ownerID))
	if err != nil {
		return "", "", err
	}
	appSecret, err := driver.box.Decrypt(application.AppSecretCiphertext, feishuApplicationAAD(ownerID))
	if err != nil {
		clear(appID)
		return "", "", err
	}
	defer clear(appID)
	defer clear(appSecret)
	return string(appID), string(appSecret), nil
}

func (driver feishuConnectorAuthorizationDriver) Begin(ctx context.Context, appID, appSecret string, scopes []string) (connectorAuthorizationChallenge, error) {
	value, err := driver.registrar.BeginAuthorization(ctx, appID, appSecret, scopes)
	if err != nil {
		return connectorAuthorizationChallenge{}, err
	}
	return connectorAuthorizationChallenge{State: value.DeviceCode, ActionURL: value.ActionURL, ExpiresAt: value.ExpiresAt, Scopes: value.Scopes}, nil
}

func (driver feishuConnectorAuthorizationDriver) Poll(ctx context.Context, appID, appSecret, state string) (connectorAuthorizationGrant, error) {
	value, err := driver.registrar.PollAuthorization(ctx, appID, appSecret, state)
	return feishuAuthorizationGrant(value), translateFeishuAuthorizationError(err)
}

func (driver feishuConnectorAuthorizationDriver) Refresh(ctx context.Context, appID, appSecret, token string) (connectorAuthorizationGrant, error) {
	value, err := driver.registrar.RefreshAuthorization(ctx, appID, appSecret, token)
	return feishuAuthorizationGrant(value), translateFeishuAuthorizationError(err)
}

func feishuAuthorizationGrant(value feishucli.Authorization) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{ExternalID: value.ExternalID, DisplayName: value.DisplayName, AccessToken: value.AccessToken, RefreshToken: value.RefreshToken, Scopes: value.Scopes, ExpiresAt: value.ExpiresAt}
}

func translateFeishuAuthorizationError(err error) error {
	switch {
	case errors.Is(err, feishucli.ErrPending):
		return errConnectorAuthorizationPending
	case errors.Is(err, feishucli.ErrDenied):
		return errConnectorAuthorizationDenied
	case errors.Is(err, feishucli.ErrExpired):
		return errConnectorAuthorizationExpired
	default:
		return err
	}
}

func (service *Service) interactiveConnectorDriver(policy connectorRevisionPolicy, repository connectorPackageRepository) (interactiveConnectorAuthorizationDriver, error) {
	if err := validateInteractiveConnectorDriver(policy); err != nil {
		return nil, err
	}
	if policy.CLI.AuthenticationDriver == "dingtalk" {
		return dingtalkConnectorAuthorizationDriver{client: dingtalkcli.NewClient()}, nil
	}
	if service.feishu == nil {
		return nil, fmt.Errorf("%w: Feishu authorization adapter is unavailable", domain.ErrInvalid)
	}
	return feishuConnectorAuthorizationDriver{registrar: service.feishu, repository: repository, box: service.box}, nil
}
