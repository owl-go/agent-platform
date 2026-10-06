package keycloak

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-platform/backend/internal/biz/account/domain"
)

func TestConfigureRegistrationUsesSignedOIDCPKCEWithoutProviderCredentials(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "open", false: "closed"}[enabled], func(t *testing.T) {
			var config map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					_, _ = w.Write([]byte(`{"access_token":"admin-token"}`))
					return
				}
				if strings.HasSuffix(r.URL.Path, "/users/profile") {
					if r.Method != http.MethodGet {
						t.Fatal("product service changed profile")
					}
					_, _ = w.Write([]byte(`{"attributes":[{"name":"aw_registration_subject","permissions":{"view":["admin"],"edit":["admin"]}},{"name":"aw_registration_provider","permissions":{"view":["admin"],"edit":["admin"]}}]}`))
					return
				}
				if r.Method == http.MethodGet {
					w.WriteHeader(404)
					return
				}
				if r.Method != http.MethodPost {
					t.Fatal("unexpected method")
				}
				if json.NewDecoder(r.Body).Decode(&config) != nil {
					t.Fatal("payload")
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			p := &Provider{client: server.Client(), baseURL: server.URL, realm: "workspace", clientID: "admin", clientSecret: "secret"}
			s := domain.RegistrationSettings{Provider: domain.RegistrationFeishu, Enabled: enabled, AppSecret: "external-secret"}
			if err := p.ConfigureRegistration(t.Context(), s, "https://workspace.test/api/v1/registration/feishu", strings.Repeat("b", 32)); err != nil {
				t.Fatal(err)
			}
			c := config["config"].(map[string]any)
			if c["pkceEnabled"] != "true" || c["pkceMethod"] != "S256" || c["validateSignature"] != "true" || c["disableUserInfo"] != "true" || config["trustEmail"] != false || config["storeToken"] != false {
				t.Fatal("unsafe broker configuration")
			}
			want := "true"
			if enabled {
				want = "false"
			}
			if c["hideOnLoginPage"] != want {
				t.Fatal("method not hidden")
			}
			encoded, _ := json.Marshal(config)
			if strings.Contains(string(encoded), "external-secret") {
				t.Fatal("external credentials forwarded to Keycloak")
			}
		})
	}
}
func TestRegistrationNeverLinksExistingPasswordAccount(t *testing.T) {
	identity, _ := domain.NewRegistrationIdentity(domain.RegistrationFeishu, "app", "person", "Name")
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			_, _ = w.Write([]byte(`{"access_token":"token"}`))
			return
		}
		if r.Method != http.MethodGet {
			writes++
		}
		_ = json.NewEncoder(w).Encode([]registrationUser{{ID: "existing-password-account", Username: identity.Username, Enabled: true}})
	}))
	defer server.Close()
	p := &Provider{client: server.Client(), baseURL: server.URL, realm: "workspace", clientID: "admin", clientSecret: "secret"}
	if _, err := p.EnsureRegistrationUser(t.Context(), domain.RegistrationFeishu, identity); err == nil || writes != 0 {
		t.Fatal("existing password account linked")
	}
}
