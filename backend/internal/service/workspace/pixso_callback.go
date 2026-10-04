package workspace

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/pixsomcp"
	"context"
	"errors"
	"net/http"
	"net/url"
)

const pixsoOAuthCallbackPath = "/api/v1/connectors/pixso/oauth/callback"
const pixsoCallbackAAD = "pixso-oauth-callback"

var pixsoBrowserOAuth = browserOAuthProfile{pixsoOAuthCallbackPath, pixsoCallbackAAD, pixsomcp.Issuer + "/api/user/pixso/oauth2/authorize", pixsomcp.Issuer, "connector_auth", pixsomcp.AcceptCallback, isPixsoMCPPolicy, false}

func (s *Service) pixsoCallbackURL() (string, error) {
	u, err := url.Parse(s.config.Authentication.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Pixso requires platform HTTPS origin")
	}
	return "https://" + u.Host + pixsoOAuthCallbackPath, nil
}
func (s *Service) sealPixsoCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	return s.sealBrowserOAuthCallback(ctx, r, flow, pixsoBrowserOAuth)
}
func (s *Service) pixsoOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, pixsoBrowserOAuth)
}
