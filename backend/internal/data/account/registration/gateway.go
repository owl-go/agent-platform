// Package registration isolates Feishu and WeChat Official Account protocols
// from account use cases. It never issues product Access Tokens.
package registration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

var ErrProvider = errors.New("registration provider rejected the request; check credentials, permissions and published application configuration")

type Gateway struct{ client *http.Client }

func New(client *http.Client) *Gateway {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	// Never forward application credentials to a redirect destination.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Gateway{client: &copyClient}
}
func (g *Gateway) request(ctx context.Context, method, endpoint, token string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, err := json.Marshal(input)
		if err != nil {
			return ErrProvider
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return ErrProvider
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return ErrProvider
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ErrProvider
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(output) != nil {
		return ErrProvider
	}
	return nil
}
func (g *Gateway) VerifySettings(ctx context.Context, s domain.RegistrationSettings) (domain.RegistrationSettings, error) {
	if s.Provider == domain.RegistrationWeChat {
		_, err := g.wechatToken(ctx, s)
		return s, err
	}
	var token struct {
		Code  *int   `json:"code"`
		Token string `json:"tenant_access_token"`
	}
	err := g.request(ctx, "POST", "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal", "", map[string]string{"app_id": s.AppID, "app_secret": s.AppSecret}, &token)
	if err != nil || token.Code == nil || *token.Code != 0 || token.Token == "" {
		return s, ErrProvider
	}
	var tenant struct {
		Code *int `json:"code"`
		Data struct {
			Tenant struct {
				Key string `json:"tenant_key"`
			} `json:"tenant"`
		} `json:"data"`
	}
	err = g.request(ctx, "GET", "https://open.feishu.cn/open-apis/tenant/v2/tenant/query", token.Token, nil, &tenant)
	if err != nil || tenant.Code == nil || *tenant.Code != 0 || strings.TrimSpace(tenant.Data.Tenant.Key) == "" {
		return s, ErrProvider
	}
	s.TenantKey = tenant.Data.Tenant.Key
	return s, nil
}
func (g *Gateway) FeishuURL(s domain.RegistrationSettings, callback, state string) string {
	return "https://accounts.feishu.cn/open-apis/authen/v1/authorize?" + url.Values{"client_id": {s.AppID}, "redirect_uri": {callback}, "response_type": {"code"}, "scope": {"contact:user.base:readonly"}, "state": {state}}.Encode()
}
func (g *Gateway) FeishuIdentity(ctx context.Context, s domain.RegistrationSettings, callback, code string) (domain.RegistrationIdentity, error) {
	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	err := g.request(ctx, "POST", "https://accounts.feishu.cn/oauth/v3/token", "", map[string]string{"grant_type": "authorization_code", "client_id": s.AppID, "client_secret": s.AppSecret, "redirect_uri": callback, "code": code}, &token)
	if err != nil || token.Error != "" || token.AccessToken == "" {
		return domain.RegistrationIdentity{}, ErrProvider
	}
	var user struct {
		Code *int `json:"code"`
		Data struct {
			OpenID    string `json:"open_id"`
			Name      string `json:"name"`
			TenantKey string `json:"tenant_key"`
		} `json:"data"`
	}
	err = g.request(ctx, "GET", "https://open.feishu.cn/open-apis/authen/v1/user_info", token.AccessToken, nil, &user)
	if err != nil || user.Code == nil || *user.Code != 0 || user.Data.TenantKey == "" || user.Data.TenantKey != s.TenantKey {
		return domain.RegistrationIdentity{}, ErrProvider
	}
	return domain.NewRegistrationIdentity(s.Provider, s.AppID, user.Data.OpenID, user.Data.Name)
}
func (g *Gateway) wechatToken(ctx context.Context, s domain.RegistrationSettings) (string, error) {
	var result struct {
		AccessToken string `json:"access_token"`
		ErrorCode   int    `json:"errcode"`
	}
	err := g.request(ctx, "POST", "https://api.weixin.qq.com/cgi-bin/stable_token", "", map[string]any{"grant_type": "client_credential", "appid": s.AppID, "secret": s.AppSecret, "force_refresh": false}, &result)
	if err != nil || result.ErrorCode != 0 || result.AccessToken == "" {
		return "", ErrProvider
	}
	return result.AccessToken, nil
}
func (g *Gateway) WeChatQR(ctx context.Context, s domain.RegistrationSettings, scene string) (string, error) {
	token, err := g.wechatToken(ctx, s)
	if err != nil {
		return "", err
	}
	var result struct {
		Ticket    string `json:"ticket"`
		ErrorCode int    `json:"errcode"`
	}
	err = g.request(ctx, "POST", "https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token="+url.QueryEscape(token), "", map[string]any{"expire_seconds": 300, "action_name": "QR_STR_SCENE", "action_info": map[string]any{"scene": map[string]string{"scene_str": scene}}}, &result)
	if err != nil || result.ErrorCode != 0 || result.Ticket == "" {
		return "", ErrProvider
	}
	return result.Ticket, nil
}
