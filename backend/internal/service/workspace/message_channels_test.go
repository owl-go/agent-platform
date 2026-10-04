package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/productanalytics"
	"fmt"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnlyAuthenticatedProviderCallbackRoutesBypassOIDC(t *testing.T) {
	filter, err := NewAuthenticationFilter(&accountapplication.Service{}, &workspaceapplication.Service{}, productanalytics.Nop{})
	if err != nil {
		t.Fatal(err)
	}
	handler := filter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, test := range []struct {
		method, path string
		status       int
	}{
		{"POST", "/api/v1/message-channel-callbacks/telegram/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 204},
		{"POST", "/api/v1/message-channel-callbacks/slack/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 204},
		{"POST", "/api/v1/message-channel-callbacks/whatsapp/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 204},
		{"GET", "/api/v1/message-channel-callbacks/whatsapp/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 204},
		{"POST", "/api/v1/message-channel-callbacks/qqbot/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 204},
		{"GET", "/api/v1/message-channel-callbacks/qqbot/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 401},
		{"PUT", "/api/v1/message-channel-callbacks/whatsapp/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 401},
		{"GET", "/api/v1/message-channel-callbacks/slack/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 401},
		{"POST", "/api/v1/message-channel-callbacks/discord/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c", 401},
		{"POST", "/api/v1/message-channel-callbacks/telegram/not-a-uuid", 401},
		{"POST", "/api/v1/workflows/workflow/message-channels", 401},
		{"POST", "/api/v1/workflows/workflow/channel-logins", 401},
		{"POST", "/api/v1/workflows/workflow/channel-logins/login/poll", 401},
		{"DELETE", "/api/v1/workflows/workflow/channel-logins/login", 401},
		{"POST", "/api/v1/message-channel-callbacks/slack/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c/actions", 401},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, recorder.Code)
		}
	}
}

func TestChannelAccountFailureHasSafePublicReason(t *testing.T) {
	for _, code := range []string{"feishu_credentials_rejected", "feishu_authentication_unavailable", "feishu_bot_unavailable", "feishu_bot_inactive", "feishu_tenant_permission_required", "feishu_tenant_unavailable"} {
		t.Run(code, func(t *testing.T) {
			err := publicError(fmt.Errorf("private-wrapper-detail: %w", &workspaceapplication.ChannelAccountFailure{Code: code, ProviderCode: 99991672, HTTPStatus: 403}))
			result := kratoserrors.FromError(err)
			if result.Code != 422 || result.Reason != code || result.Message != code || result.Metadata["provider_code"] != "99991672" || result.Metadata["provider_http_status"] != "403" {
				t.Fatalf("public failure: %#v", result)
			}
		})
	}
	unknown := kratoserrors.FromError(publicError(&workspaceapplication.ChannelAccountFailure{Code: "private-provider-detail"}))
	if unknown.Reason != "invalid_input" || unknown.Message != "channel account connection failed" {
		t.Fatal("unknown provider detail exposed")
	}
}
