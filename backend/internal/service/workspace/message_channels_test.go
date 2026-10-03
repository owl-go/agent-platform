package workspace

import (
	accountapplication "agent-platform/backend/internal/biz/account/application"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/productanalytics"
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
		{"POST", "/api/v1/message-channel-callbacks/slack/06e1cec8-e4e3-4f75-98cd-5a3222d2b80c/actions", 401},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s %s: %d", test.method, test.path, recorder.Code)
		}
	}
}
