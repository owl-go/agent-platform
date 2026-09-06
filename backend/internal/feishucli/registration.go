package feishucli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	accountsBaseURL  = "https://accounts.feishu.cn"
	openBaseURL      = "https://open.feishu.cn"
	registrationPath = "/oauth/v1/app/registration"
	cliVersion       = "1.0.93"
	maxResponseBytes = 1 << 20
)

var (
	ErrPending = errors.New("Feishu application registration is pending")
	ErrDenied  = errors.New("Feishu application registration was denied")
	ErrExpired = errors.New("Feishu application registration expired")
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Registration struct {
	DeviceCode string
	ActionURL  string
	ExpiresAt  time.Time
}

type Application struct {
	AppID, AppSecret, UserName string
}

// Registrar implements the application device flow published by the official
// Feishu CLI without exposing registration codes or application credentials.
type Registrar struct {
	client                    HTTPClient
	accountsBase, openBaseURL string
	now                       func() time.Time
}

func NewRegistrar(client HTTPClient) *Registrar {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Registrar{client: client, accountsBase: accountsBaseURL, openBaseURL: openBaseURL, now: func() time.Time { return time.Now().UTC() }}
}

func (registrar *Registrar) Begin(ctx context.Context) (Registration, error) {
	values := url.Values{
		"action":            {"begin"},
		"archetype":         {"PersonalAgent"},
		"auth_method":       {"client_secret"},
		"request_user_info": {"open_id tenant_brand"},
	}
	body, err := registrar.post(ctx, values)
	if err != nil {
		return Registration{}, fmt.Errorf("begin Feishu application registration: %w", err)
	}
	deviceCode, userCode := stringField(body, "device_code"), stringField(body, "user_code")
	expiresIn := integerField(body, "expire_in")
	if expiresIn <= 0 {
		expiresIn = integerField(body, "expires_in")
	}
	if deviceCode == "" || userCode == "" || expiresIn <= 0 || expiresIn > 24*60*60 {
		return Registration{}, errors.New("begin Feishu application registration: invalid provider response")
	}
	action := registrar.openBaseURL + "/page/cli?" + url.Values{
		"user_code": {userCode}, "lpv": {cliVersion}, "ocv": {cliVersion}, "from": {"cli"},
	}.Encode()
	return Registration{DeviceCode: deviceCode, ActionURL: action, ExpiresAt: registrar.now().Add(time.Duration(expiresIn) * time.Second)}, nil
}

func (registrar *Registrar) Poll(ctx context.Context, deviceCode string) (Application, error) {
	if strings.TrimSpace(deviceCode) == "" {
		return Application{}, errors.New("poll Feishu application registration: device code is required")
	}
	body, err := registrar.post(ctx, url.Values{"action": {"poll"}, "device_code": {deviceCode}})
	if err != nil {
		return Application{}, fmt.Errorf("poll Feishu application registration: %w", err)
	}
	switch stringField(body, "error") {
	case "authorization_pending", "slow_down":
		return Application{}, ErrPending
	case "access_denied":
		return Application{}, ErrDenied
	case "expired_token", "invalid_grant":
		return Application{}, ErrExpired
	case "":
		application := Application{AppID: stringField(body, "client_id"), AppSecret: stringField(body, "client_secret")}
		if userInfo, ok := body["user_info"].(map[string]any); ok {
			application.UserName = stringField(userInfo, "name")
			if application.UserName == "" {
				application.UserName = stringField(userInfo, "display_name")
			}
		}
		if application.AppID == "" || application.AppSecret == "" {
			return Application{}, ErrPending
		}
		return application, nil
	default:
		return Application{}, errors.New("poll Feishu application registration: provider rejected request")
	}
}

func (registrar *Registrar) post(ctx context.Context, values url.Values) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, registrar.accountsBase+registrationPath, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := registrar.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, errors.New("provider returned invalid JSON")
	}
	if (response.StatusCode < 200 || response.StatusCode >= 300) && stringField(body, "error") == "" {
		return nil, fmt.Errorf("provider returned HTTP %d", response.StatusCode)
	}
	return body, nil
}

func stringField(body map[string]any, name string) string {
	value, _ := body[name].(string)
	return value
}

func integerField(body map[string]any, name string) int {
	value, _ := body[name].(float64)
	return int(value)
}
