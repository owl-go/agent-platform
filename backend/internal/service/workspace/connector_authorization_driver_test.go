package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"agent-platform/backend/internal/connectorpackage"
	"agent-platform/backend/internal/feishucli"
)

type recordingFeishuAuthorizationRegistrar struct {
	beginAppID, beginSecret string
	pollState               string
	pollError               error
}

func (*recordingFeishuAuthorizationRegistrar) Begin(context.Context) (feishucli.Registration, error) {
	return feishucli.Registration{}, nil
}
func (*recordingFeishuAuthorizationRegistrar) Poll(context.Context, string) (feishucli.Application, error) {
	return feishucli.Application{}, nil
}
func (registrar *recordingFeishuAuthorizationRegistrar) BeginAuthorization(_ context.Context, appID, secret string, scopes []string) (feishucli.AuthorizationRequest, error) {
	registrar.beginAppID, registrar.beginSecret = appID, secret
	return feishucli.AuthorizationRequest{DeviceCode: "opaque-code", ActionURL: "https://open.feishu.cn/authorize", Scopes: scopes, ExpiresAt: time.Now().Add(time.Minute)}, nil
}
func (registrar *recordingFeishuAuthorizationRegistrar) PollAuthorization(_ context.Context, _, _, state string) (feishucli.Authorization, error) {
	registrar.pollState = state
	if registrar.pollError != nil {
		return feishucli.Authorization{}, registrar.pollError
	}
	return feishucli.Authorization{ExternalID: "account-1", AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (*recordingFeishuAuthorizationRegistrar) RefreshAuthorization(context.Context, string, string, string) (feishucli.Authorization, error) {
	return feishucli.Authorization{ExternalID: "account-1", AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestFeishuAuthorizationDriverPreservesPendingStateAndCredentials(t *testing.T) {
	registrar := &recordingFeishuAuthorizationRegistrar{}
	driver := feishuConnectorAuthorizationDriver{registrar: registrar}
	challenge, err := driver.Begin(context.Background(), "app-id", "app-secret", []string{"docs:read"})
	if err != nil || challenge.State != "opaque-code" || challenge.ActionURL == "" || registrar.beginAppID != "app-id" || registrar.beginSecret != "app-secret" {
		t.Fatalf("begin challenge is invalid: %v", err)
	}
	registrar.pollError = feishucli.ErrPending
	if _, err := driver.Poll(context.Background(), "app-id", "app-secret", challenge.State); !errors.Is(err, errConnectorAuthorizationPending) {
		t.Fatalf("pending authorization error = %v", err)
	}
	registrar.pollError = feishucli.ErrDenied
	if _, err := driver.Poll(context.Background(), "app-id", "app-secret", challenge.State); !errors.Is(err, errConnectorAuthorizationDenied) {
		t.Fatalf("denied authorization error = %v", err)
	}
	registrar.pollError = feishucli.ErrExpired
	if _, err := driver.Poll(context.Background(), "app-id", "app-secret", challenge.State); !errors.Is(err, errConnectorAuthorizationExpired) {
		t.Fatalf("expired authorization error = %v", err)
	}
	registrar.pollError = nil
	grant, err := driver.Poll(context.Background(), "app-id", "app-secret", challenge.State)
	if err != nil || registrar.pollState != "opaque-code" || grant.ExternalID != "account-1" || grant.RefreshToken != "refresh" {
		t.Fatalf("completed grant identity or refresh material is invalid: %v", err)
	}
	rotated, err := driver.Refresh(context.Background(), "app-id", "app-secret", grant.RefreshToken)
	if err != nil || rotated.RefreshToken != "new-refresh" {
		t.Fatalf("refresh material did not rotate: %v", err)
	}
}

func TestGitHubPolicyRequiresBrowserAuthorizationAndOnlyReleasesAccessCredential(t *testing.T) {
	policy := connectorRevisionPolicy{Metadata: connectorpackage.Metadata{Source: "github"}, AuthMode: "oauth", CLI: &connectorpackage.CLIManifest{AuthenticationDriver: "connector_package"}}
	if connectorAuthorizationMode(policy) != "interactive" {
		t.Fatal("GitHub fell back to manual token connection")
	}
	fields := connectorAuthorizationCredentialFields(policy, connectorAuthorizationGrant{AccessToken: "access", RefreshToken: "refresh", ClientID: "client"})
	if len(fields) != 1 || fields["access_token"] != "access" {
		t.Fatal("unexpected process credential fields")
	}
	policy.AuthMode = "cli"
	if isGitHubCLILoginPolicy(policy) {
		t.Fatal("unreviewed auth mode received GitHub device driver")
	}
	policy.AuthMode = "oauth"
	policy.Metadata.Source = "unknown"
	if isGitHubCLILoginPolicy(policy) {
		t.Fatal("unknown package received GitHub driver")
	}
}
