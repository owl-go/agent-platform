package messagechannel

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
)

// Only server-selected HTTPS iLink hosts are accepted. Neither owner inputs nor
// arbitrary provider redirects can send bot tokens to an untrusted origin.
func wechatAPIBase(value string) (string, error) {
	if value == "" {
		return "https://ilinkai.weixin.qq.com", nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || !strings.HasPrefix(u.Hostname(), "ilink") || !strings.HasSuffix(u.Hostname(), ".weixin.qq.com") {
		return "", providerError("provider_endpoint_invalid")
	}
	return "https://" + u.Hostname(), nil
}
func wechatLoginHeaders() http.Header {
	return http.Header{"iLink-App-Id": {"bot"}, "iLink-App-ClientVersion": {"132104"}}
}
func (a *WeChat) StartLogin(ctx context.Context) (application.ChannelLoginStep, error) {
	// QR creation is unauthenticated. Existing users' bot tokens are never mixed.
	headers := wechatLoginHeaders()
	headers.Set("AuthorizationType", "ilink_bot_token")
	uin := make([]byte, 4)
	if _, err := rand.Read(uin); err != nil {
		return application.ChannelLoginStep{}, providerError("provider_request_invalid")
	}
	headers.Set("X-WECHAT-UIN", wechatUIN(uin))
	result, status, _, err := a.requestHeaders(ctx, http.MethodPost, "https://ilinkai.weixin.qq.com/ilink/bot/get_bot_qrcode?bot_type=3", "", map[string]any{"local_token_list": []string{}}, headers)
	if err != nil || status != 200 || rawString(result["qrcode"]) == "" || rawString(result["qrcode_img_content"]) == "" {
		return application.ChannelLoginStep{}, providerError("provider_login_failed")
	}
	return application.ChannelLoginStep{Status: "waiting", QRContent: rawString(result["qrcode_img_content"]), State: map[string]string{"qrcode": rawString(result["qrcode"]), "baseurl": "https://ilinkai.weixin.qq.com"}}, nil
}
func (a *WeChat) PollLogin(ctx context.Context, state map[string]string, code string) (application.ChannelLoginStep, error) {
	base, err := wechatAPIBase(state["baseurl"])
	if err != nil {
		return application.ChannelLoginStep{}, err
	}
	target := base + "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(state["qrcode"])
	if code != "" {
		target += "&verify_code=" + url.QueryEscape(code)
	}
	// iLink holds an unscanned QR request for about 30 seconds. Keep the
	// provider poll below the API's 30-second unary deadline; a local polling
	// deadline means still waiting, while cancellation of the caller propagates.
	pollCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	result, status, _, err := a.requestHeaders(pollCtx, http.MethodGet, target, "", nil, wechatLoginHeaders())
	if pollCtx.Err() != nil && ctx.Err() == nil {
		return application.ChannelLoginStep{Status: "waiting", State: state}, nil
	}
	if err != nil || status != 200 {
		return application.ChannelLoginStep{}, providerError("provider_login_failed")
	}
	step := application.ChannelLoginStep{State: state}
	switch rawString(result["status"]) {
	case "wait":
		step.Status = "waiting"
	case "scaned":
		step.Status = "scanned"
	case "need_verifycode":
		step.Status = "verification_required"
	case "expired":
		step.Status = "expired"
	case "verify_code_blocked", "binded_redirect":
		step.Status = "failed"
	case "scaned_but_redirect":
		redirected, err := wechatAPIBase("https://" + rawString(result["redirect_host"]))
		if err != nil {
			return step, err
		}
		step.State["baseurl"] = redirected
		step.Status = "scanned"
	case "confirmed":
		base, err = wechatAPIBase(rawString(result["baseurl"]))
		if err != nil {
			return step, err
		}
		c := application.ChannelCredentials{"bot_token": rawString(result["bot_token"]), "account_id": rawString(result["ilink_bot_id"]), "user_id": rawString(result["ilink_user_id"]), "baseurl": base}
		if c["bot_token"] == "" || c["account_id"] == "" || c["user_id"] == "" {
			return step, providerError("provider_login_failed")
		}
		step.Status, step.Credentials, step.SuggestedSenderID = "connected", c, c["user_id"]
	default:
		return step, providerError("provider_response_invalid")
	}
	return step, nil
}

// Tencent's qqbot-connector 1.2.0: create_bind_task / poll_bind_result with
// a client-generated AES-256-GCM key (nonce || ciphertext || 16-byte tag).
func (a *QQBot) StartLogin(ctx context.Context) (application.ChannelLoginStep, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return application.ChannelLoginStep{}, providerError("provider_request_invalid")
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	result, status, _, err := a.request(ctx, http.MethodPost, "https://q.qq.com/lite/create_bind_task", "", map[string]string{"key": encoded})
	var data struct {
		TaskID string `json:"task_id"`
	}
	if err != nil || status != 200 || result["retcode"] == nil || rawNumber(result["retcode"]) != 0 || json.Unmarshal(result["data"], &data) != nil || data.TaskID == "" {
		return application.ChannelLoginStep{}, providerError("provider_login_failed")
	}
	return application.ChannelLoginStep{Status: "waiting", QRContent: "https://q.qq.com/qqbot/openclaw/connect.html?task_id=" + url.QueryEscape(data.TaskID) + "&source=&_wv=2", State: map[string]string{"task_id": data.TaskID, "key": encoded}}, nil
}
func (a *QQBot) PollLogin(ctx context.Context, state map[string]string, _ string) (application.ChannelLoginStep, error) {
	result, status, _, err := a.request(ctx, http.MethodPost, "https://q.qq.com/lite/poll_bind_result", "", map[string]string{"task_id": state["task_id"]})
	var data struct {
		Status int             `json:"status"`
		AppID  json.RawMessage `json:"bot_appid"`
		Secret string          `json:"bot_encrypt_secret"`
		User   string          `json:"user_openid"`
	}
	if err != nil || status != 200 || result["retcode"] == nil || rawNumber(result["retcode"]) != 0 || json.Unmarshal(result["data"], &data) != nil {
		return application.ChannelLoginStep{}, providerError("provider_login_failed")
	}
	step := application.ChannelLoginStep{State: state, Status: "waiting"}
	switch data.Status {
	case 0, 1:
		return step, nil
	case 3:
		step.Status = "expired"
		return step, nil
	case 2:
		key, err := base64.StdEncoding.DecodeString(state["key"])
		if err != nil || len(key) != 32 {
			return step, providerError("provider_login_failed")
		}
		sealed, err := base64.StdEncoding.DecodeString(data.Secret)
		if err != nil || len(sealed) < 28 {
			return step, providerError("provider_login_failed")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return step, providerError("provider_login_failed")
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return step, providerError("provider_login_failed")
		}
		secret, err := gcm.Open(nil, sealed[:12], sealed[12:], nil)
		if err != nil {
			return step, providerError("provider_login_failed")
		}
		appID := rawString(data.AppID)
		if appID == "" {
			var number json.Number
			if json.Unmarshal(data.AppID, &number) == nil {
				appID = number.String()
			}
		}
		if appID == "" || len(secret) == 0 {
			return step, providerError("provider_login_failed")
		}
		step.Status, step.Credentials, step.SuggestedSenderID = "connected", application.ChannelCredentials{"app_id": appID, "app_secret": string(secret)}, data.User
		return step, nil
	default:
		return step, providerError("provider_response_invalid")
	}
}
