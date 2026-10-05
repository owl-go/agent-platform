package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type registrationHTTPFixture struct {
	workspacev1.AgentWorkspaceServiceHTTPServer
	request *workspacev1.UpdateRegistrationSettingsRequest
}

func (f *registrationHTTPFixture) UpdateRegistrationSettings(_ context.Context, request *workspacev1.UpdateRegistrationSettingsRequest) (*workspacev1.RegistrationMethod, error) {
	f.request = request
	return &workspacev1.RegistrationMethod{Provider: request.Provider}, nil
}

func TestRegistrationSettingsRequiresJSONMediaType(t *testing.T) {
	for _, test := range []struct {
		mediaType string
		status    int
	}{
		{"text/plain;charset=UTF-8", http.StatusBadRequest},
		{"application/json", http.StatusOK},
	} {
		t.Run(test.mediaType, func(t *testing.T) {
			fixture := &registrationHTTPFixture{}
			server := kratoshttp.NewServer()
			workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, fixture)
			request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/registration-methods/wechat_official", strings.NewReader(`{"enabled":true,"app_id":"fixture-app","reason":"enable registration","expected_version":0}`))
			request.Header.Set("Content-Type", test.mediaType)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d, want %d: %s", response.Code, test.status, response.Body.String())
			}
			if test.status == http.StatusBadRequest {
				if fixture.request != nil {
					t.Fatal("unsupported media type reached the settings use case")
				}
				return
			}
			if fixture.request == nil || fixture.request.Provider != "wechat_official" || !fixture.request.Enabled || fixture.request.AppId != "fixture-app" || fixture.request.Reason != "enable registration" {
				t.Fatalf("incorrectly decoded settings: %+v", fixture.request)
			}
		})
	}
}
