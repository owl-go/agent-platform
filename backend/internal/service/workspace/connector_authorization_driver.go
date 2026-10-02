package workspace

import (
	"agent-platform/backend/internal/xiaoemcp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/camscannercli"
	"agent-platform/backend/internal/dingtalkcli"
	"agent-platform/backend/internal/feishucli"
	"agent-platform/backend/internal/githubcli"
	"agent-platform/backend/internal/klingmcp"
	"agent-platform/backend/internal/linearmcp"
	"agent-platform/backend/internal/notioncli"
	"agent-platform/backend/internal/pixsomcp"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/teambitioncli"
	"agent-platform/backend/internal/tianyanchamcp"
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
	IsDomestic                  string
	Scopes                      []string
	ExpiresAt, RefreshExpiresAt time.Time
}

type dingtalkConnectorAuthorizationDriver struct{ client *dingtalkcli.Client }

type teambitionConnectorAuthorizationDriver struct{ client *teambitioncli.Client }

func (d teambitionConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d teambitionConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d teambitionConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, teambitioncli.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, teambitioncli.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return teambitionGrant(v), e
}
func (d teambitionConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return teambitionGrant(v), e
}
func teambitionGrant(v teambitioncli.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

type klingConnectorAuthorizationDriver struct{ client *klingmcp.Client }

func (d klingConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d klingConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d klingConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, klingmcp.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, klingmcp.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return klingGrant(v), e
}
func (d klingConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return klingGrant(v), e
}
func klingGrant(v klingmcp.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}
func isKlingMCPLoginPolicy(policy connectorRevisionPolicy) bool {
	return policy.Metadata.Source == "kling-ai" && policy.AuthMode == "oauth" && policy.CLI == nil && policy.MCP != nil && policy.MCP.Transport == "streamable_http" && policy.MCP.URL == klingmcp.Resource && len(policy.MCP.EgressHosts) == 1 && policy.MCP.EgressHosts[0] == "klingai.com" && len(policy.MCP.Headers) == 0 && len(policy.MCP.Environment) == 0
}
func isBrowserOAuthPolicy(policy connectorRevisionPolicy) bool {
	return isTianyanchaMCPPolicy(policy) || isXiaoeMCPLoginPolicy(policy) || isTeambitionCLILoginPolicy(policy) || isKlingMCPLoginPolicy(policy) || isLinearMCPPolicy(policy) || isPixsoMCPPolicy(policy)
}
func browserOAuthProfileFor(policy connectorRevisionPolicy) browserOAuthProfile {
	if isTianyanchaMCPPolicy(policy) {
		return tianyanchaBrowserOAuth
	}
	if isXiaoeMCPLoginPolicy(policy) {
		return xiaoeBrowserOAuth
	}
	if isPixsoMCPPolicy(policy) {
		return pixsoBrowserOAuth
	}
	if isLinearMCPPolicy(policy) {
		return linearBrowserOAuth
	}
	if isKlingMCPLoginPolicy(policy) {
		return klingBrowserOAuth
	}
	return teambitionBrowserOAuth
}
func (s *Service) klingCallbackURL() (string, error) {
	u, e := url.Parse(s.config.Authentication.RedirectURI)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", fmt.Errorf("%w: Kling AI requires platform HTTPS origin", domain.ErrInvalid)
	}
	return "https://" + u.Host + klingOAuthCallbackPath, nil
}

type linearConnectorAuthorizationDriver struct{ client *linearmcp.Client }

func (d linearConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d linearConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d linearConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, linearmcp.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, linearmcp.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return linearGrant(v), e
}
func (d linearConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return linearGrant(v), e
}
func linearGrant(v linearmcp.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

type pixsoConnectorAuthorizationDriver struct{ client *pixsomcp.Client }

func (d pixsoConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d pixsoConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d pixsoConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, pixsomcp.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, pixsomcp.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return pixsoGrant(v), e
}
func (d pixsoConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return pixsoGrant(v), e
}
func pixsoGrant(v pixsomcp.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

type tianyanchaConnectorAuthorizationDriver struct{ client *tianyanchamcp.Client }

func (d tianyanchaConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d tianyanchaConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d tianyanchaConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, tianyanchamcp.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, tianyanchamcp.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return tianyanchaGrant(v), e
}
func (d tianyanchaConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return tianyanchaGrant(v), e
}
func tianyanchaGrant(v tianyanchamcp.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

type camscannerConnectorAuthorizationDriver struct{ client *camscannercli.Client }

func (d camscannerConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d camscannerConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	if len(scopes) != 0 {
		return connectorAuthorizationChallenge{}, fmt.Errorf("%w: CamScanner CLI login does not accept granular scopes", domain.ErrInvalid)
	}
	v, e := d.client.Begin(ctx)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, ExpiresAt: v.ExpiresAt}, e
}
func (d camscannerConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, camscannercli.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, camscannercli.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return camscannerGrant(v), e
}
func (d camscannerConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return camscannerGrant(v), e
}
func camscannerGrant(v camscannercli.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{ExternalID: v.UserID, DisplayName: v.UserID, AccessToken: v.Token, RefreshToken: v.Token, ExpiresAt: v.ExpiresAt, IsDomestic: v.IsDomestic}
}

type githubConnectorAuthorizationDriver struct{ client *githubcli.Client }

func (d githubConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d githubConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d githubConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	switch {
	case errors.Is(e, githubcli.ErrPending):
		e = errConnectorAuthorizationPending
	case errors.Is(e, githubcli.ErrDenied):
		e = errConnectorAuthorizationDenied
	case errors.Is(e, githubcli.ErrExpired):
		e = errConnectorAuthorizationExpired
	}
	return connectorAuthorizationGrant{ExternalID: v.ExternalID, DisplayName: v.DisplayName, AccessToken: v.AccessToken, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d githubConnectorAuthorizationDriver) Refresh(context.Context, string, string, string) (connectorAuthorizationGrant, error) {
	return connectorAuthorizationGrant{}, fmt.Errorf("%w: reconnect GitHub to renew access", domain.ErrInvalid)
}

type xiaoeConnectorAuthorizationDriver struct{ client *xiaoemcp.Client }

func (d xiaoeConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}
func (d xiaoeConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	v, e := d.client.Begin(ctx, scopes)
	return connectorAuthorizationChallenge{State: v.State, ActionURL: v.ActionURL, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt}, e
}
func (d xiaoeConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Poll(ctx, state)
	if errors.Is(e, xiaoemcp.ErrPending) {
		e = errConnectorAuthorizationPending
	}
	if errors.Is(e, xiaoemcp.ErrExpired) {
		e = errConnectorAuthorizationExpired
	}
	return xiaoeGrant(v), e
}
func (d xiaoeConnectorAuthorizationDriver) Refresh(ctx context.Context, id, _, token string) (connectorAuthorizationGrant, error) {
	v, e := d.client.Refresh(ctx, id, token)
	return xiaoeGrant(v), e
}
func xiaoeGrant(v xiaoemcp.Grant) connectorAuthorizationGrant {
	return connectorAuthorizationGrant{AccessToken: v.AccessToken, RefreshToken: v.RefreshToken, ClientID: v.ClientID, Scopes: v.Scopes, ExpiresAt: v.ExpiresAt, RefreshExpiresAt: v.RefreshExpiresAt}
}

type notionConnectorAuthorizationDriver struct{ login *notioncli.Login }

func (driver notionConnectorAuthorizationDriver) Application(context.Context, string, string) (string, string, error) {
	return "", "", nil
}

func (driver notionConnectorAuthorizationDriver) Begin(ctx context.Context, _, _ string, scopes []string) (connectorAuthorizationChallenge, error) {
	if len(scopes) != 0 {
		return connectorAuthorizationChallenge{}, fmt.Errorf("%w: Notion login does not accept platform scopes", domain.ErrInvalid)
	}
	challenge, err := driver.login.Begin(ctx)
	return connectorAuthorizationChallenge{State: challenge.State, ActionURL: challenge.ActionURL, ExpiresAt: challenge.ExpiresAt}, err
}

func (driver notionConnectorAuthorizationDriver) Poll(ctx context.Context, _, _, state string) (connectorAuthorizationGrant, error) {
	grant, err := driver.login.Poll(ctx, state)
	switch {
	case errors.Is(err, notioncli.ErrPending):
		return connectorAuthorizationGrant{}, errConnectorAuthorizationPending
	case errors.Is(err, notioncli.ErrExpired):
		return connectorAuthorizationGrant{}, errConnectorAuthorizationExpired
	case err != nil:
		return connectorAuthorizationGrant{}, err
	}
	return connectorAuthorizationGrant{ExternalID: grant.WorkspaceID, DisplayName: grant.WorkspaceID, AccessToken: grant.Token}, nil
}

func (driver notionConnectorAuthorizationDriver) Refresh(context.Context, string, string, string) (connectorAuthorizationGrant, error) {
	return connectorAuthorizationGrant{}, fmt.Errorf("%w: reconnect Notion to renew access", domain.ErrInvalid)
}

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
	if isLinearMCPPolicy(policy) {
		redirect, err := service.linearCallbackURL()
		if err != nil {
			return nil, err
		}
		return linearConnectorAuthorizationDriver{client: linearmcp.NewClient(redirect)}, nil
	}
	if isPixsoMCPPolicy(policy) {
		redirect, err := service.pixsoCallbackURL()
		if err != nil {
			return nil, err
		}
		return pixsoConnectorAuthorizationDriver{client: pixsomcp.NewClient(redirect)}, nil
	}
	if isTianyanchaMCPPolicy(policy) {
		redirect, err := service.tianyanchaCallbackURL()
		if err != nil {
			return nil, err
		}
		return tianyanchaConnectorAuthorizationDriver{client: tianyanchamcp.NewClient(redirect)}, nil
	}
	if isKlingMCPLoginPolicy(policy) {
		redirect, err := service.klingCallbackURL()
		if err != nil {
			return nil, err
		}
		return klingConnectorAuthorizationDriver{client: klingmcp.NewClient(redirect)}, nil
	}
	if isXiaoeMCPLoginPolicy(policy) {
		redirect, err := service.xiaoeCallbackURL()
		if err != nil {
			return nil, err
		}
		return xiaoeConnectorAuthorizationDriver{client: xiaoemcp.NewClient(redirect)}, nil
	}
	if policy.CLI == nil {
		return nil, fmt.Errorf("%w: authorization adapter is unavailable", domain.ErrInvalid)
	}
	if policy.CLI.AuthenticationDriver == "dingtalk" {
		return dingtalkConnectorAuthorizationDriver{client: dingtalkcli.NewClient()}, nil
	}
	if isGitHubCLILoginPolicy(policy) {
		return githubConnectorAuthorizationDriver{client: githubcli.NewClient()}, nil
	}
	if isTeambitionCLILoginPolicy(policy) {
		redirect, err := service.teambitionCallbackURL()
		if err != nil {
			return nil, err
		}
		return teambitionConnectorAuthorizationDriver{client: teambitioncli.NewClient(redirect)}, nil
	}
	if isCamScannerCLILoginPolicy(policy) {
		return camscannerConnectorAuthorizationDriver{client: camscannercli.NewClient()}, nil
	}
	if isNotionCLILoginPolicy(policy) {
		return notionConnectorAuthorizationDriver{login: notioncli.NewLogin()}, nil
	}
	if service.feishu == nil {
		return nil, fmt.Errorf("%w: Feishu authorization adapter is unavailable", domain.ErrInvalid)
	}
	return feishuConnectorAuthorizationDriver{registrar: service.feishu, repository: repository, box: service.box}, nil
}
