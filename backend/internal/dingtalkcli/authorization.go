package dingtalkcli

import (
	"bytes"
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

var (
	ErrPending          = errors.New("DingTalk authorization is pending")
	ErrDenied           = errors.New("DingTalk authorization was denied")
	ErrExpired          = errors.New("DingTalk authorization expired")
	ErrIdentityMismatch = errors.New("DingTalk contact identity belongs to another organization")
	ErrCLIAuthDisabled  = errors.New("DingTalk organization has not enabled CLI access")
)

// Client implements the OAuth device protocol used by pinned DWS v1.0.62.
// It never accepts provider endpoints from a package or an API request.
type Client struct {
	httpClient *http.Client
	mcpBase    string
	loginBase  string
	pollBase   string
	contactURL string
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		mcpBase:    "https://mcp.dingtalk.com", loginBase: "https://login.dingtalk.com",
		pollBase:   "https://open-dev.dingtalk.com",
		contactURL: "https://mcp-gw.dingtalk.com/server/db4b26cb38ea6a8739ad55d1997fa1da608cd36b33a6cf0f77884f70c49382fe",
	}
}

type Challenge struct {
	State     string
	ActionURL string
	ExpiresAt time.Time
}

type Grant struct {
	ExternalID, DisplayName     string
	AccessToken, RefreshToken   string
	ClientID                    string
	ExpiresAt, RefreshExpiresAt time.Time
}

// Credentials are encrypted by the platform. Only AccessToken is released to
// the isolated DWS process; the rotating refresh token never enters Runtime.
type Credentials struct {
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	ClientID        string    `json:"client_id"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
}

type deviceState struct {
	ClientID   string `json:"client_id"`
	DeviceCode string `json:"device_code"`
	FlowID     string `json:"flow_id"`
}

func (client *Client) Begin(ctx context.Context) (Challenge, error) {
	var identity struct {
		Success bool   `json:"success"`
		Result  string `json:"result"`
	}
	if err := client.call(ctx, http.MethodGet, client.mcpBase+"/cli/clientId", nil, nil, &identity); err != nil || !identity.Success || identity.Result == "" {
		return Challenge{}, errors.New("DingTalk authorization client is unavailable")
	}
	form := url.Values{"client_id": {identity.Result}, "scope": {"openid corpid"}}
	var device struct {
		Success bool `json:"success"`
		Result  struct {
			DeviceCode              string `json:"deviceCode"`
			FlowID                  string `json:"flowId"`
			VerificationURIComplete string `json:"verificationUriComplete"`
			ExpiresIn               int    `json:"expiresIn"`
		} `json:"result"`
	}
	if err := client.call(ctx, http.MethodPost, client.loginBase+"/oauth2/device/code.json", []byte(form.Encode()), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, &device); err != nil || !device.Success {
		return Challenge{}, errors.New("DingTalk device authorization is unavailable")
	}
	action, err := url.Parse(device.Result.VerificationURIComplete)
	if err != nil || action == nil || action.Scheme != "https" || action.User != nil || action.Fragment != "" || (action.Hostname() != "login.dingtalk.com" && action.Hostname() != "login.dingtalk.io") || action.Port() != "" {
		return Challenge{}, errors.New("DingTalk authorization returned an unsafe action URL")
	}
	if device.Result.DeviceCode == "" || device.Result.ExpiresIn < 1 || device.Result.ExpiresIn > 900 {
		return Challenge{}, errors.New("DingTalk device authorization is incomplete")
	}
	state, _ := json.Marshal(deviceState{ClientID: identity.Result, DeviceCode: device.Result.DeviceCode, FlowID: device.Result.FlowID})
	return Challenge{State: string(state), ActionURL: action.String(), ExpiresAt: time.Now().UTC().Add(time.Duration(device.Result.ExpiresIn) * time.Second)}, nil
}

func (client *Client) Poll(ctx context.Context, encodedState string) (Grant, error) {
	var state deviceState
	if err := json.Unmarshal([]byte(encodedState), &state); err != nil || state.ClientID == "" || state.DeviceCode == "" {
		return Grant{}, errors.New("DingTalk device authorization state is invalid")
	}
	code := ""
	if state.FlowID != "" {
		var response struct {
			Success bool                              `json:"success"`
			Data    struct{ Status, AuthCode string } `json:"data"`
			Result  struct{ Status, AuthCode string } `json:"result"`
		}
		endpoint := client.pollBase + "/cli/oauth/device/poll?flowId=" + url.QueryEscape(state.FlowID)
		if err := client.call(ctx, http.MethodGet, endpoint, nil, nil, &response); err != nil {
			return Grant{}, err
		}
		status, authCode := response.Data.Status, response.Data.AuthCode
		if status == "" {
			status, authCode = response.Result.Status, response.Result.AuthCode
		}
		switch strings.ToUpper(status) {
		case "PENDING":
			return Grant{}, ErrPending
		case "REJECTED":
			return Grant{}, ErrDenied
		case "EXPIRED":
			return Grant{}, ErrExpired
		case "APPROVED":
			code = authCode
		default:
			return Grant{}, errors.New("DingTalk authorization returned an unknown status")
		}
	} else {
		form := url.Values{"client_id": {state.ClientID}, "device_code": {state.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}
		var response struct {
			Success bool                             `json:"success"`
			Result  struct{ AuthCode, Error string } `json:"result"`
		}
		if err := client.call(ctx, http.MethodPost, client.loginBase+"/oauth2/device/token.json", []byte(form.Encode()), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, &response); err != nil {
			return Grant{}, err
		}
		switch response.Result.Error {
		case "authorization_pending", "slow_down":
			return Grant{}, ErrPending
		case "access_denied":
			return Grant{}, ErrDenied
		case "expired_token":
			return Grant{}, ErrExpired
		case "":
			if response.Success {
				code = response.Result.AuthCode
			}
		}
	}
	if code == "" {
		return Grant{}, errors.New("DingTalk authorization returned no code")
	}
	return client.exchange(ctx, state.ClientID, code)
}

func (client *Client) exchange(ctx context.Context, clientID, code string) (Grant, error) {
	request := map[string]string{"clientId": clientID, "authCode": code, "grantType": "authorization_code"}
	body, _ := json.Marshal(request)
	var result tokenResponse
	if err := client.call(ctx, http.MethodPost, client.mcpBase+"/oauth2/getToken", body, map[string]string{"Content-Type": "application/json"}, &result); err != nil {
		return Grant{}, err
	}
	grant, err := result.grant(clientID)
	if err != nil {
		return Grant{}, err
	}
	if grant.ExternalID == "" {
		userID, userName, lookupErr := client.currentUser(ctx, grant.AccessToken, result.CorpID)
		if errors.Is(lookupErr, ErrIdentityMismatch) {
			return Grant{}, lookupErr
		}
		if lookupErr == nil {
			grant.ExternalID, grant.DisplayName = result.CorpID+":"+userID, userName
		} else {
			// The official DWS CLI retains valid organization grants when the
			// contact service cannot resolve an external-worker user ID.
			grant.ExternalID, grant.DisplayName = "corp:"+result.CorpID, result.CorpID
		}
	}
	var permission struct {
		Success   bool   `json:"success"`
		ErrorCode string `json:"errorCode"`
		Result    *struct {
			CLIAuthEnabled bool `json:"cliAuthEnabled"`
		} `json:"result"`
	}
	if err := client.call(ctx, http.MethodGet, client.mcpBase+"/cli/cliAuthEnabled", nil, map[string]string{"x-user-access-token": grant.AccessToken}, &permission); err != nil || !permission.Success || permission.Result == nil || !permission.Result.CLIAuthEnabled {
		if err == nil && (permission.Success && permission.Result != nil && !permission.Result.CLIAuthEnabled || permission.ErrorCode == "ENTERPRISE_NOT_AUTHORIZED" || permission.ErrorCode == "NO_AUTH" || permission.ErrorCode == "CHANNEL_REQUIRED") {
			return Grant{}, ErrCLIAuthDisabled
		}
		return Grant{}, errors.New("DingTalk CLI access check is unavailable")
	}
	return grant, nil
}

func (client *Client) Refresh(ctx context.Context, clientID, refreshToken string) (Grant, error) {
	if clientID == "" || refreshToken == "" {
		return Grant{}, errors.New("DingTalk refresh credential is incomplete")
	}
	body, _ := json.Marshal(map[string]string{"clientId": clientID, "refreshToken": refreshToken, "grantType": "refresh_token"})
	var result tokenResponse
	if err := client.call(ctx, http.MethodPost, client.mcpBase+"/oauth2/refreshToken", body, map[string]string{"Content-Type": "application/json"}, &result); err != nil {
		return Grant{}, err
	}
	if result.ErrorCode != "" || result.AccessToken == "" || result.RefreshToken == "" || result.ExpiresIn < 60 || result.ExpiresIn > 86400 {
		return Grant{}, errors.New("DingTalk refresh response is incomplete")
	}
	now := time.Now().UTC()
	grant := Grant{AccessToken: result.AccessToken, RefreshToken: result.RefreshToken, ClientID: clientID, ExpiresAt: now.Add(time.Duration(result.ExpiresIn) * time.Second), RefreshExpiresAt: now.Add(30 * 24 * time.Hour)}
	if result.CorpID != "" && result.UserID != "" {
		grant.ExternalID = result.CorpID + ":" + result.UserID
		grant.DisplayName = result.UserName
	}
	return grant, nil
}

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ClientID     string `json:"clientId"`
	CorpID       string `json:"corpId"`
	UserID       string `json:"userId"`
	UserName     string `json:"userName"`
	ExpiresIn    int64  `json:"expiresIn"`
	ErrorCode    string `json:"errorCode"`
}

func (result tokenResponse) grant(clientID string) (Grant, error) {
	if result.ErrorCode != "" || result.AccessToken == "" || result.CorpID == "" || result.ExpiresIn < 60 || result.ExpiresIn > 86400 {
		return Grant{}, errors.New("DingTalk token response is incomplete")
	}
	now := time.Now().UTC()
	grant := Grant{DisplayName: result.UserName, AccessToken: result.AccessToken, RefreshToken: result.RefreshToken, ClientID: clientID, ExpiresAt: now.Add(time.Duration(result.ExpiresIn) * time.Second)}
	if result.RefreshToken != "" {
		grant.RefreshExpiresAt = now.Add(30 * 24 * time.Hour)
	}
	if result.UserID != "" {
		grant.ExternalID = result.CorpID + ":" + result.UserID
	}
	return grant, nil
}

// DWS resolves a missing userId through the same contact product after token
// exchange. Accept only one identity in the exact organization returned by OAuth.
func (client *Client) currentUser(ctx context.Context, token, corpID string) (string, string, error) {
	if client.contactURL == "" || token == "" || corpID == "" {
		return "", "", errors.New("DingTalk current-user lookup is unavailable")
	}
	request := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_current_user_profile","arguments":{}}}`)
	var response struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := client.call(ctx, http.MethodPost, client.contactURL, request, map[string]string{"Content-Type": "application/json", "Accept": "application/json", "Authorization": "Bearer " + token, "x-user-access-token": token}, &response); err != nil || len(response.Error) != 0 {
		return "", "", errors.New("DingTalk current-user lookup failed")
	}
	var matches, withoutCorp []struct{ userID, name string }
	resultCount := 0
	foreignCorp := false
	for _, block := range response.Result.Content {
		if block.Type != "text" {
			continue
		}
		var payload struct {
			Result []struct {
				Employee struct {
					CorpID       string `json:"corpId"`
					UserID       string `json:"userId"`
					UserIDLower  string `json:"userid"`
					OrgUserID    string `json:"orgUserId"`
					Name         string `json:"orgUserName"`
					FallbackName string `json:"name"`
				} `json:"orgEmployeeModel"`
			} `json:"result"`
		}
		if json.Unmarshal([]byte(block.Text), &payload) != nil {
			continue
		}
		resultCount += len(payload.Result)
		for _, item := range payload.Result {
			userID := strings.TrimSpace(item.Employee.UserID)
			lower := strings.TrimSpace(item.Employee.UserIDLower)
			orgUserID := strings.TrimSpace(item.Employee.OrgUserID)
			if userID == "" {
				userID = lower
			}
			if userID == "" {
				userID = orgUserID
			}
			if userID == "" || (lower != "" && lower != userID) || (orgUserID != "" && orgUserID != userID) {
				continue
			}
			name := strings.TrimSpace(item.Employee.Name)
			if name == "" {
				name = strings.TrimSpace(item.Employee.FallbackName)
			}
			candidate := struct{ userID, name string }{userID, name}
			switch strings.TrimSpace(item.Employee.CorpID) {
			case corpID:
				matches = append(matches, candidate)
			case "":
				withoutCorp = append(withoutCorp, candidate)
			default:
				foreignCorp = true
			}
		}
	}
	if len(matches) == 0 && len(withoutCorp) == 1 && resultCount == 1 {
		matches = withoutCorp
	}
	if len(matches) != 1 {
		if foreignCorp {
			return "", "", ErrIdentityMismatch
		}
		return "", "", errors.New("DingTalk current-user identity is missing or ambiguous")
	}
	return matches[0].userID, matches[0].name, nil
}

func (client *Client) call(ctx context.Context, method, endpoint string, body []byte, headers map[string]string, target any) error {
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) {
			return fmt.Errorf("DingTalk authorization request failed: %w", urlError.Err)
		}
		return fmt.Errorf("DingTalk authorization request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("DingTalk authorization returned HTTP %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return errors.New("DingTalk authorization response could not be read")
	}
	if err := json.Unmarshal(contents, target); err != nil {
		return errors.New("DingTalk authorization response is invalid")
	}
	return nil
}
