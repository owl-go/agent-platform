package keycloak

import (
	"github.com/google/uuid"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/account/domain"
)

// The supplied realm must be disposable and have a service client named
// scan-admin with manage-users, view-users and manage-identity-providers.
func TestKeycloakRegistrationProvisioningIntegration(t *testing.T) {
	base := os.Getenv("WORKSPACE_TEST_KEYCLOAK_URL")
	if base == "" {
		t.Skip("WORKSPACE_TEST_KEYCLOAK_URL is not set")
	}
	p := &Provider{client: &http.Client{Timeout: 10 * time.Second}, baseURL: strings.TrimRight(base, "/"), realm: "scan-fixture", clientID: "scan-admin", clientSecret: "scan-fixture"}
	settings := domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: true}
	if err := p.ConfigureRegistration(t.Context(), settings, "http://host.docker.internal:58958/api/v1/registration/feishu", strings.Repeat("b", 32)); err != nil {
		t.Fatal(err)
	}
	identity, err := domain.NewRegistrationIdentity(settings.Provider, "fixture-app", t.Name()+uuid.NewString(), "企业用户")
	if err != nil {
		t.Fatal(err)
	}
	user, err := p.EnsureRegistrationUser(t.Context(), settings.Provider, identity)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := p.EnsureRegistrationUser(t.Context(), settings.Provider, identity)
	if err != nil || repeat.OIDCSubject != user.OIDCSubject {
		t.Fatal("returning identity changed", err)
	}
	if err = p.SetEnabled(t.Context(), user.OIDCSubject, false); err != nil {
		t.Fatal(err)
	}
	if _, err = p.EnsureRegistrationUser(t.Context(), settings.Provider, identity); err == nil {
		t.Fatal("disabled Keycloak user authenticated")
	}
}
