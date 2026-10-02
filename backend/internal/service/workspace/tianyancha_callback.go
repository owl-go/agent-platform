package workspace

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/tianyanchamcp"
	"context"
	"errors"
	"net/http"
	"net/url"
)

const tianyanchaOAuthCallbackPath = "/api/v1/connectors/tianyancha/oauth/callback"
const tianyanchaCallbackAAD = "tianyancha-oauth-callback"

var tianyanchaBrowserOAuth = browserOAuthProfile{tianyanchaOAuthCallbackPath, tianyanchaCallbackAAD, tianyanchamcp.Issuer + "/authorize", tianyanchamcp.Issuer, "connector_auth", tianyanchamcp.AcceptCallback, isTianyanchaMCPPolicy, false}

func (s *Service) tianyanchaCallbackURL() (string, error) {
	u, err := url.Parse(s.config.Authentication.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Tianyancha requires platform HTTPS origin")
	}
	return "https://" + u.Host + tianyanchaOAuthCallbackPath, nil
}
func (s *Service) sealTianyanchaCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	return s.sealBrowserOAuthCallback(ctx, r, flow, tianyanchaBrowserOAuth)
}
func (s *Service) tianyanchaOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, tianyanchaBrowserOAuth)
}
