package messagechannel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func TestFeishuIdentifiesTenantFromApplicationCredentials(t *testing.T) {
	for _, region := range []string{"feishu", "lark"} {
		t.Run(region, func(t *testing.T) {
			calls := 0
			adapter := &Feishu{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != strings.TrimPrefix(feishuBase(region), "https://") {
					t.Fatalf("wrong region: %s", r.URL.Host)
				}
				switch r.URL.Path {
				case "/open-apis/auth/v3/tenant_access_token/internal":
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || r.Method != http.MethodPost || len(body) != 2 || body["app_id"] != "app" || body["app_secret"] != "secret" {
						t.Fatalf("unexpected token request: %v, %v", body, err)
					}
					return jsonResponse(`{"code":0,"tenant_access_token":"access-token"}`, http.StatusOK), nil
				case "/open-apis/bot/v3/info", "/open-apis/tenant/v2/tenant/query":
					if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer access-token" {
						t.Fatal("identity lookup did not use application token")
					}
					if r.URL.Path == "/open-apis/bot/v3/info" {
						return jsonResponse(`{"code":0,"bot":{"open_id":"bot","app_name":"Bot","activate_status":2}}`, http.StatusOK), nil
					}
					return jsonResponse(`{"code":0,"data":{"tenant":{"tenant_key":"verified-tenant","display_id":"F123"}}}`, http.StatusOK), nil
				default:
					t.Fatalf("unexpected request: %s", r.URL.Path)
					return nil, errors.New("unexpected request")
				}
			}))}
			credentials := application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}
			for _, legacyTenant := range []string{"", "untrusted-tenant"} {
				if legacyTenant != "" {
					credentials["tenant_key"] = legacyTenant
				}
				identity, err := adapter.Identify(context.Background(), credentials, region)
				if err != nil || identity.ID != "bot" || identity.Name != "Bot" || identity.TenantID != "verified-tenant" || identity.BindingID != feishuBase(region)+":app" {
					t.Fatalf("identity: %#v, %v", identity, err)
				}
			}
			stored := application.ChannelStored{Channel: domain.MessageChannel{Region: region, AccountID: "bot", TenantID: "verified-tenant"}}
			if err := adapter.Configure(context.Background(), stored, credentials, ""); err != nil {
				t.Fatal(err)
			}
			stored.Channel.TenantID = "other-tenant"
			if err := adapter.Configure(context.Background(), stored, credentials, ""); err == nil {
				t.Fatal("changed tenant accepted")
			}
			if calls != 12 {
				t.Fatalf("expected authenticated lookup on each identity check, got %d requests", calls)
			}
		})
	}
}

func TestFeishuRejectsUnverifiedTenant(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		network    bool
	}{
		{"permission denied", `{"code":1184001,"msg":"private-provider-detail"}`, http.StatusForbidden, false},
		{"provider error", `{"code":1184000,"msg":"private-provider-detail"}`, http.StatusOK, false},
		{"missing code", `{"data":{"tenant":{"tenant_key":"tenant"}}}`, http.StatusOK, false},
		{"null code", `{"code":null,"data":{"tenant":{"tenant_key":"tenant"}}}`, http.StatusOK, false},
		{"invalid code", `{"code":"0","data":{"tenant":{"tenant_key":"tenant"}}}`, http.StatusOK, false},
		{"missing data", `{"code":0}`, http.StatusOK, false},
		{"empty tenant", `{"code":0,"data":{"tenant":{"tenant_key":""}}}`, http.StatusOK, false},
		{"blank tenant", `{"code":0,"data":{"tenant":{"tenant_key":" "}}}`, http.StatusOK, false},
		{"wrong tenant type", `{"code":0,"data":{"tenant":{"tenant_key":42}}}`, http.StatusOK, false},
		{"invalid json", `{`, http.StatusOK, false},
		{"network failure", "", 0, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := &Feishu{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/open-apis/auth/v3/tenant_access_token/internal":
					return jsonResponse(`{"code":0,"tenant_access_token":"access-token"}`, http.StatusOK), nil
				case "/open-apis/bot/v3/info":
					return jsonResponse(`{"code":0,"bot":{"open_id":"bot","activate_status":2}}`, http.StatusOK), nil
				default:
					if test.network {
						return nil, errors.New("private-provider-detail")
					}
					return jsonResponse(test.body, test.status), nil
				}
			}))}
			identity, err := adapter.Identify(context.Background(), application.ChannelCredentials{"app_id": "app", "app_secret": "secret", "tenant_key": "legacy-tenant"}, "feishu")
			if err == nil || identity != (application.ChannelIdentity{}) || !errors.Is(err, domain.ErrInvalid) || strings.Contains(err.Error(), "private-provider-detail") {
				t.Fatalf("unsafe tenant lookup result: %#v, %v", identity, err)
			}
		})
	}
}

func TestFeishuRequiresBothApplicationCredentials(t *testing.T) {
	adapter := &Feishu{NewHTTP(roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("incomplete credentials reached provider")
		return nil, nil
	}))}
	for _, credentials := range []application.ChannelCredentials{nil, {"app_id": "app"}, {"app_secret": "secret"}} {
		if _, err := adapter.Identify(context.Background(), credentials, "feishu"); err == nil {
			t.Fatal("incomplete credentials accepted")
		}
	}
}

func TestFeishuAccountFailureReportsSafeStep(t *testing.T) {
	tests := []struct {
		name, path, body, code string
		status                 int
	}{
		{"credential rejection", "/open-apis/auth/v3/tenant_access_token/internal", `{"code":10003,"msg":"secret-provider-detail"}`, "feishu_credentials_rejected", 200},
		{"bot unavailable", "/open-apis/bot/v3/info", `{"code":99991672,"msg":"secret-provider-detail"}`, "feishu_bot_unavailable", 403},
		{"bot inactive", "/open-apis/bot/v3/info", `{"code":0,"bot":{"open_id":"bot","activate_status":1}}`, "feishu_bot_inactive", 200},
		{"missing tenant scope", "/open-apis/tenant/v2/tenant/query", `{"code":99991672,"msg":"secret-provider-detail"}`, "feishu_tenant_permission_required", 403},
		{"tenant denied", "/open-apis/tenant/v2/tenant/query", `{"code":1184001,"msg":"secret-provider-detail"}`, "feishu_tenant_permission_required", 403},
		{"tenant absent", "/open-apis/tenant/v2/tenant/query", `{"code":1184000,"msg":"secret-provider-detail"}`, "feishu_tenant_unavailable", 404},
		{"malformed token success", "/open-apis/auth/v3/tenant_access_token/internal", `{"code":"0","tenant_access_token":"token"}`, "feishu_authentication_unavailable", 200},
		{"malformed bot success", "/open-apis/bot/v3/info", `{"code":"0","bot":{"open_id":"bot","activate_status":2}}`, "feishu_bot_unavailable", 200},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := &Feishu{NewHTTP(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == test.path {
					return jsonResponse(test.body, test.status), nil
				}
				switch r.URL.Path {
				case "/open-apis/auth/v3/tenant_access_token/internal":
					return jsonResponse(`{"code":0,"tenant_access_token":"token"}`, 200), nil
				case "/open-apis/bot/v3/info":
					return jsonResponse(`{"code":0,"bot":{"open_id":"bot","activate_status":2}}`, 200), nil
				default:
					t.Fatal("unexpected request after failure")
					return nil, errors.New("unexpected request")
				}
			}))}
			identity, err := a.Identify(context.Background(), application.ChannelCredentials{"app_id": "app", "app_secret": "secret"}, "feishu")
			if identity != (application.ChannelIdentity{}) || err == nil || !strings.Contains(err.Error(), test.code) || strings.Contains(err.Error(), "secret-provider-detail") || !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("failure lost safe step: %#v, %v", identity, err)
			}
		})
	}
}
