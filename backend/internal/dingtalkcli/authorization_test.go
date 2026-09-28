package dingtalkcli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeviceAuthorizationAndRefresh(t *testing.T) {
	status := "PENDING"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/clientId":
			_, _ = w.Write([]byte(`{"success":true,"result":"client-1"}`))
		case "/oauth2/device/code.json":
			if r.FormValue("client_id") != "client-1" {
				t.Error("device flow lost Client ID")
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"deviceCode":"device-secret","flowId":"flow-1","verificationUriComplete":"https://login.dingtalk.com/verify?user_code=ABCD","expiresIn":600}}`))
		case "/cli/oauth/device/poll":
			if r.URL.Query().Get("flowId") != "flow-1" {
				t.Error("flow ID was not preserved")
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"status":"` + status + `","authCode":"one-use-code"}}`))
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"accessToken":"access","refreshToken":"refresh","corpId":"corp-1","userId":"user-1","userName":"Alice","expiresIn":7200}`))
		case "/cli/cliAuthEnabled":
			if r.Header.Get("x-user-access-token") != "access" {
				t.Error("CLI permission check lost bearer token")
			}
			_, _ = w.Write([]byte(`{"success":true,"result":{"cliAuthEnabled":true}}`))
		case "/oauth2/refreshToken":
			_, _ = w.Write([]byte(`{"accessToken":"new-access","refreshToken":"new-refresh","expiresIn":7200}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, loginBase: server.URL, pollBase: server.URL}
	challenge, err := client.Begin(context.Background())
	if err != nil || !strings.HasPrefix(challenge.ActionURL, "https://login.dingtalk.com/") || challenge.State == "" {
		t.Fatalf("challenge = %#v, %v", challenge, err)
	}
	if _, err := client.Poll(context.Background(), challenge.State); !errors.Is(err, ErrPending) {
		t.Fatalf("pending = %v", err)
	}
	status = "APPROVED"
	grant, err := client.Poll(context.Background(), challenge.State)
	if err != nil || grant.ExternalID != "corp-1:user-1" || grant.RefreshToken != "refresh" || grant.ClientID != "client-1" {
		t.Fatalf("grant = %#v, %v", grant, err)
	}
	rotated, err := client.Refresh(context.Background(), grant.ClientID, grant.RefreshToken)
	if err != nil || rotated.AccessToken != "new-access" || rotated.RefreshToken != "new-refresh" {
		t.Fatalf("refresh = %#v, %v", rotated, err)
	}
}

func TestUnsafeDeviceActionURLIsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cli/clientId" {
			_, _ = w.Write([]byte(`{"success":true,"result":"client-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":{"deviceCode":"secret","verificationUriComplete":"https://evil.example/steal","expiresIn":600}}`))
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, loginBase: server.URL}
	if _, err := client.Begin(context.Background()); err == nil {
		t.Fatal("unsafe action URL was accepted")
	}
}

func TestDeviceAuthorizationResolvesIdentityWhenTokenHasNoUserID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"accessToken":"access","refreshToken":"refresh","corpId":"corp-1","expiresIn":7200}`))
		case "/contact":
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Error("contact lookup did not use the new token")
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\"result\":[{\"orgEmployeeModel\":{\"orgUserId\":\"user-1\",\"orgUserName\":\"Alice\"}}]}"}]}}`))
		case "/cli/cliAuthEnabled":
			_, _ = w.Write([]byte(`{"success":true,"result":{"cliAuthEnabled":true}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, contactURL: server.URL + "/contact"}
	grant, err := client.exchange(context.Background(), "client-1", "one-use-code")
	if err != nil || grant.ExternalID != "corp-1:user-1" || grant.DisplayName != "Alice" {
		t.Fatalf("grant identity = %q / %q, error %v", grant.ExternalID, grant.DisplayName, err)
	}
}

func TestDeviceAuthorizationKeepsOrganizationGrantWhenContactProfileIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"accessToken":"access","refreshToken":"refresh","corpId":"corp-1","expiresIn":7200}`))
		case "/contact":
			w.WriteHeader(http.StatusForbidden)
		case "/cli/cliAuthEnabled":
			_, _ = w.Write([]byte(`{"success":true,"result":{"cliAuthEnabled":true}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, contactURL: server.URL + "/contact"}
	grant, err := client.exchange(context.Background(), "client-1", "one-use-code")
	if err != nil || grant.ExternalID != "corp:corp-1" || grant.DisplayName != "corp-1" {
		t.Fatalf("organization grant = %q / %q, error %v", grant.ExternalID, grant.DisplayName, err)
	}
}

func TestDeviceAuthorizationRejectsContactIdentityFromAnotherOrganization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"accessToken":"access","refreshToken":"refresh","corpId":"corp-1","expiresIn":7200}`))
		case "/contact":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":3,"result":{"content":[{"type":"text","text":"{\"result\":[{\"orgEmployeeModel\":{\"corpId\":\"corp-2\",\"orgUserId\":\"user-1\"}}]}"}]}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, contactURL: server.URL + "/contact"}
	if _, err := client.exchange(context.Background(), "client-1", "one-use-code"); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("mismatched organization error = %v", err)
	}
}

func TestAccessOnlyTokenUsesAccessExpiry(t *testing.T) {
	grant, err := (tokenResponse{AccessToken: "access", CorpID: "corp-1", UserID: "user-1", ExpiresIn: 7200}).grant("client-1")
	if err != nil || grant.AccessToken != "access" || !grant.RefreshExpiresAt.IsZero() {
		t.Fatalf("access-only grant expiry = %v, error %v", grant.RefreshExpiresAt, err)
	}
}

func TestDeviceAuthorizationReportsCLIRestriction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"accessToken":"access","refreshToken":"refresh","corpId":"corp-1","userId":"user-1","expiresIn":7200}`))
		case "/cli/cliAuthEnabled":
			_, _ = w.Write([]byte(`{"success":false,"errorCode":"ENTERPRISE_NOT_AUTHORIZED"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL}
	if _, err := client.exchange(context.Background(), "client-1", "one-use-code"); !errors.Is(err, ErrCLIAuthDisabled) {
		t.Fatalf("CLI restriction error = %v", err)
	}
}

func TestCLIRestrictionReason(t *testing.T) {
	tests := []struct {
		name, response, want string
	}{
		{"enterprise", `{"success":false,"errorCode":"ENTERPRISE_NOT_AUTHORIZED"}`, "enterprise_not_authorized"},
		{"channel", `{"success":false,"errorCode":"CHANNEL_REQUIRED"}`, "channel_required"},
		{"no auth", `{"success":false,"errorCode":"NO_AUTH"}`, "no_auth"},
		{"user forbidden", `{"success":true,"result":{"cliAuthEnabled":false,"userScope":"forbidden"}}`, "user_forbidden"},
		{"channel restricted", `{"success":true,"result":{"cliAuthEnabled":false,"channelScope":"specified"}}`, "channel_required"},
		{"user excluded", `{"success":true,"result":{"cliAuthEnabled":false,"userScope":"specified"}}`, "user_not_allowed"},
		{"unspecified restriction", `{"success":true,"result":{"cliAuthEnabled":false}}`, "cli_not_enabled"},
		{"allowed", `{"success":true,"result":{"cliAuthEnabled":true}}`, ""},
		{"unavailable", `{"success":false,"errorCode":"SERVER_ERROR"}`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var status cliAuthStatus
			if err := json.Unmarshal([]byte(test.response), &status); err != nil {
				t.Fatal(err)
			}
			if got := cliRestrictionReason(status); got != test.want {
				t.Fatalf("reason = %q, want %q", got, test.want)
			}
		})
	}
}

func TestApprovedDeviceFlowMarksExchangeFailureAsConsumed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/oauth/device/poll":
			_, _ = w.Write([]byte(`{"success":true,"result":{"status":"APPROVED","authCode":"one-use-code"}}`))
		case "/oauth2/getToken":
			_, _ = w.Write([]byte(`{"errorCode":"AUTH_CODE_USED"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := &Client{httpClient: server.Client(), mcpBase: server.URL, pollBase: server.URL}
	_, err := client.Poll(context.Background(), `{"client_id":"client-1","device_code":"device-secret","flow_id":"flow-1"}`)
	if !errors.Is(err, ErrApprovalConsumed) {
		t.Fatalf("approved one-use code must be terminal: %v", err)
	}
}
