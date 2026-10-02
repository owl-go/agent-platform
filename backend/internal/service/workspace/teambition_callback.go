package workspace

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/klingmcp"
	"agent-platform/backend/internal/teambitioncli"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

const teambitionOAuthCallbackPath = "/api/v1/connectors/teambition/oauth/callback"
const teambitionCallbackAAD = "teambition-oauth-callback"

type browserOAuthCallbackReference struct {
	OwnerID string `json:"owner_id"`
	FlowID  string `json:"flow_id"`
}

func (s *Service) teambitionCallbackURL() (string, error) {
	u, e := url.Parse(s.config.Authentication.RedirectURI)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Teambition requires platform HTTPS origin")
	}
	return "https://" + u.Host + teambitionOAuthCallbackPath, nil
}

type browserOAuthProfile struct {
	callbackPath, aad, authorizationURL, issuer, returnKey string
	accept                                                 func(string, string, string) (string, error)
	matches                                                func(connectorRevisionPolicy) bool
	issuerRequired                                         bool
}

var teambitionBrowserOAuth = browserOAuthProfile{teambitionOAuthCallbackPath, teambitionCallbackAAD, teambitioncli.Issuer + "/oauth2/mcp/authorize", teambitioncli.Issuer, "teambition_auth", teambitioncli.AcceptCallback, isTeambitionCLILoginPolicy, true}
var klingBrowserOAuth = browserOAuthProfile{klingOAuthCallbackPath, "kling-ai-oauth-callback", klingmcp.Issuer + "/authorize", klingmcp.Issuer, "connector_auth", klingmcp.AcceptCallback, isKlingMCPLoginPolicy, false}

const klingOAuthCallbackPath = "/api/v1/connectors/kling-ai/oauth/callback"

func (s *Service) sealTeambitionCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	return s.sealBrowserOAuthCallback(ctx, r, flow, teambitionBrowserOAuth)
}
func (s *Service) sealBrowserOAuthCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt, profile browserOAuthProfile) (domain.ConnectorAuthorizationAttempt, error) {
	reference, _ := json.Marshal(browserOAuthCallbackReference{flow.OwnerID, flow.ID})
	sealed, e := s.box.Encrypt(reference, profile.aad)
	if e != nil {
		return flow, e
	}
	action, e := url.Parse(flow.ActionURL)
	if e != nil || action.Scheme != "https" || action.Scheme+"://"+action.Host+action.Path != profile.authorizationURL || action.User != nil || action.Fragment != "" {
		return flow, errors.New("invalid Connector authorization URL")
	}
	q := action.Query()
	q.Set("state", base64.RawURLEncoding.EncodeToString(sealed))
	action.RawQuery = q.Encode()
	flow.ActionURL = action.String()
	e = r.UpdateConnectorAuthorizationFlow(ctx, flow.OwnerID, flow.ID, flow.DeviceCodeCiphertext, flow.DeviceCodeCiphertext, flow.ActionURL)
	return flow, e
}
func (s *Service) teambitionOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, teambitionBrowserOAuth)
}
func (s *Service) klingOAuthCallback(w http.ResponseWriter, req *http.Request) {
	s.browserOAuthCallback(w, req, klingBrowserOAuth)
}
func (s *Service) browserOAuthCallback(w http.ResponseWriter, req *http.Request, profile browserOAuthProfile) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	fail := func() {
		http.Error(w, "Connector authorization was not completed. Return to Connector settings and connect again.", http.StatusBadRequest)
	}
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := req.URL.Query()
	encoded := q.Get("state")
	if len(q["state"]) != 1 || len(q["code"]) != 1 || len(q["iss"]) > 1 || len(encoded) > 2048 || q.Get("error") != "" || (q.Get("iss") != "" && q.Get("iss") != profile.issuer) || (profile.issuerRequired && q.Get("iss") == "") {
		fail()
		return
	}
	cipher, e := base64.RawURLEncoding.DecodeString(encoded)
	if e != nil {
		fail()
		return
	}
	raw, e := s.box.Decrypt(cipher, profile.aad)
	if e != nil {
		fail()
		return
	}
	defer clear(raw)
	var ref browserOAuthCallbackReference
	if json.Unmarshal(raw, &ref) != nil || ref.OwnerID == "" || ref.FlowID == "" {
		fail()
		return
	}
	repository, e := s.connectorPackages()
	if e != nil {
		fail()
		return
	}
	flow, e := repository.GetConnectorAuthorizationFlow(req.Context(), ref.OwnerID, ref.FlowID)
	if e != nil {
		fail()
		return
	}
	action, e := url.Parse(flow.ActionURL)
	if e != nil || action.Query().Get("state") != encoded || !flow.ExpiresAt.After(time.Now()) {
		fail()
		return
	}
	_, _, policy, e := connectorInstallationPolicy(req.Context(), repository, ref.OwnerID, flow.InstallationID)
	if e != nil || !profile.matches(policy) {
		fail()
		return
	}
	plaintext, e := s.box.Decrypt(flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD(ref.OwnerID, flow.InstallationID, flow.Identity))
	if e != nil {
		fail()
		return
	}
	defer clear(plaintext)
	state, e := profile.accept(string(plaintext), q.Get("code"), q.Get("iss"))
	if e != nil {
		fail()
		return
	}
	next, e := s.box.Encrypt([]byte(state), connectorAuthorizationFlowAAD(ref.OwnerID, flow.InstallationID, flow.Identity))
	if e != nil {
		fail()
		return
	}
	if e = repository.UpdateConnectorAuthorizationFlow(req.Context(), ref.OwnerID, flow.ID, flow.DeviceCodeCiphertext, next, flow.ActionURL); e != nil {
		fail()
		return
	}
	// Only an owner-authenticated completion request exchanges the code and stores the grant.
	// The callback never accepts an owner ID or emits provider credentials.
	http.Redirect(w, req, "/resources?tab=connectors&"+profile.returnKey+"="+url.QueryEscape(flow.ID), http.StatusSeeOther)
}
