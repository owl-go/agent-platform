package workspace

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/xiaoemcp"
	"context"
	"errors"
	"net/http"
	"net/url"
)

const xiaoeOAuthCallbackPath = "/api/v1/connectors/xiaoe/oauth/callback"
const xiaoeCallbackAAD = "xiaoe-oauth-callback"

var xiaoeBrowserOAuth = browserOAuthProfile{xiaoeOAuthCallbackPath, xiaoeCallbackAAD, xiaoemcp.Issuer + "/oauth/authorize", xiaoemcp.Issuer, "xiaoe_auth", xiaoemcp.AcceptCallback, isXiaoeMCPLoginPolicy, true}

func (s *Service) xiaoeCallbackURL() (string, error) {
	u, err := url.Parse(s.config.Authentication.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Xiaoe requires platform HTTPS origin")
	}
	return "https://" + u.Host + xiaoeOAuthCallbackPath, nil
}
func (s *Service) sealXiaoeCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	return s.sealBrowserOAuthCallback(ctx, r, flow, xiaoeBrowserOAuth)
}
func (s *Service) xiaoeOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, xiaoeBrowserOAuth)
}
