package workspace

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/linearmcp"
	"context"
	"errors"
	"net/http"
	"net/url"
)

const linearOAuthCallbackPath = "/api/v1/connectors/linear/oauth/callback"
const linearCallbackAAD = "linear-oauth-callback"

var linearBrowserOAuth = browserOAuthProfile{linearOAuthCallbackPath, linearCallbackAAD, linearmcp.Issuer + "/authorize", linearmcp.Issuer, "linear_auth", linearmcp.AcceptCallback, isLinearMCPPolicy, true}

func (s *Service) linearCallbackURL() (string, error) {
	u, err := url.Parse(s.config.Authentication.RedirectURI)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Linear requires platform HTTPS origin")
	}
	return "https://" + u.Host + linearOAuthCallbackPath, nil
}
func (s *Service) sealLinearCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	return s.sealBrowserOAuthCallback(ctx, r, flow, linearBrowserOAuth)
}
func (s *Service) linearOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, linearBrowserOAuth)
}
