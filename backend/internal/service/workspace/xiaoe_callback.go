package workspace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/xiaoemcp"
)

const xiaoeOAuthCallbackPath = "/api/v1/connectors/xiaoe/oauth/callback"
const xiaoeCallbackAAD = "xiaoe-oauth-callback"

type xiaoeCallbackReference struct {
	OwnerID string `json:"owner_id"`
	FlowID  string `json:"flow_id"`
}

func (s *Service) xiaoeCallbackURL() (string, error) {
	u, e := url.Parse(s.config.Authentication.RedirectURI)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return "", errors.New("Xiaoe requires platform HTTPS origin")
	}
	return "https://" + u.Host + xiaoeOAuthCallbackPath, nil
}
func (s *Service) sealXiaoeCallback(ctx context.Context, r connectorPackageRepository, flow domain.ConnectorAuthorizationAttempt) (domain.ConnectorAuthorizationAttempt, error) {
	reference, _ := json.Marshal(xiaoeCallbackReference{flow.OwnerID, flow.ID})
	sealed, e := s.box.Encrypt(reference, xiaoeCallbackAAD)
	if e != nil {
		return flow, e
	}
	action, e := url.Parse(flow.ActionURL)
	if e != nil || action.Scheme != "https" || action.Host != "agent.xiaoe-tech.com" || action.Path != "/oauth/authorize" {
		return flow, errors.New("invalid Xiaoe authorization URL")
	}
	q := action.Query()
	q.Set("state", base64.RawURLEncoding.EncodeToString(sealed))
	action.RawQuery = q.Encode()
	flow.ActionURL = action.String()
	e = r.UpdateConnectorAuthorizationFlow(ctx, flow.OwnerID, flow.ID, flow.DeviceCodeCiphertext, flow.DeviceCodeCiphertext, flow.ActionURL)
	return flow, e
}
func (s *Service) xiaoeOAuthCallback(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	fail := func() {
		http.Error(w, "Xiaoe authorization was not completed. Return to Connector settings and connect again.", http.StatusBadRequest)
	}
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	q := req.URL.Query()
	encoded := q.Get("state")
	if len(encoded) > 2048 || q.Get("error") != "" || q.Get("iss") != xiaoemcp.Issuer {
		fail()
		return
	}
	cipher, e := base64.RawURLEncoding.DecodeString(encoded)
	if e != nil {
		fail()
		return
	}
	raw, e := s.box.Decrypt(cipher, xiaoeCallbackAAD)
	if e != nil {
		fail()
		return
	}
	defer clear(raw)
	var ref xiaoeCallbackReference
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
	if e != nil || action.Query().Get("state") != encoded {
		fail()
		return
	}
	_, _, policy, e := connectorInstallationPolicy(req.Context(), repository, ref.OwnerID, flow.InstallationID)
	if e != nil || !isXiaoeMCPLoginPolicy(policy) {
		fail()
		return
	}
	plaintext, e := s.box.Decrypt(flow.DeviceCodeCiphertext, connectorAuthorizationFlowAAD(ref.OwnerID, flow.InstallationID, flow.Identity))
	if e != nil {
		fail()
		return
	}
	defer clear(plaintext)
	state, e := xiaoemcp.AcceptCallback(string(plaintext), q.Get("code"), q.Get("iss"))
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
	http.Redirect(w, req, "/resources?tab=connectors&xiaoe_auth="+url.QueryEscape(flow.ID), http.StatusSeeOther)
}
