package keycloak

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"agent-platform/backend/internal/platformconfig"
)

func TestIdentityNamesAlwaysSatisfyKeycloakRequiredProfile(t *testing.T) {
	tests := []struct {
		displayName string
		first       string
		last        string
	}{
		{displayName: "Ada Lovelace", first: "Ada", last: "Lovelace"},
		{displayName: "张三", first: "张三", last: "张三"},
		{displayName: "Platform Acceptance User", first: "Platform Acceptance", last: "User"},
	}
	for _, test := range tests {
		first, last := identityNames(test.displayName)
		if first != test.first || last != test.last {
			t.Fatalf("identityNames(%q) = (%q, %q), want (%q, %q)", test.displayName, first, last, test.first, test.last)
		}
	}
}

func TestListGroupsReadsHierarchyDepartmentMarkerAndMembersWithoutWritingIdentitySource(t *testing.T) {
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		switch request.URL.Path {
		case "/realms/workspace/protocol/openid-connect/token":
			_ = json.NewEncoder(response).Encode(map[string]string{"access_token": "test-token"})
		case "/admin/realms/workspace/groups":
			if request.Method != http.MethodGet || request.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("unexpected Group request: %s %q", request.Method, request.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(response).Encode([]map[string]any{{
				"id": "group-root", "name": "Company", "path": "/Company", "subGroups": []map[string]any{{
					"id": "group-finance", "name": "Finance", "path": "/Company/Finance", "attributes": map[string][]string{"agent_workspace_department": {"true"}},
				}},
			}})
		case "/admin/realms/workspace/groups/group-root/members":
			_ = json.NewEncoder(response).Encode([]map[string]string{{"id": "user-admin"}})
		case "/admin/realms/workspace/groups/group-finance/members":
			_ = json.NewEncoder(response).Encode([]map[string]string{{"id": "user-finance"}, {"id": "user-admin"}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	provider, err := New(platformconfig.AccountsConfig{
		KeycloakBaseURL: server.URL, Realm: "workspace", AdminClientID: "admin", AdminClientSecret: "secret",
		BootstrapSubject: "user-admin", BootstrapUsername: "admin", BootstrapEmail: "admin@example.test", BootstrapDisplayName: "Administrator",
	}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	groups, err := provider.ListGroups(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[1].ExternalID != "group-finance" || groups[1].Path != "/Company/Finance" || !groups[1].Department {
		t.Fatalf("unexpected synchronized groups: %#v", groups)
	}
	if len(groups[1].MemberSubjects) != 2 || groups[1].MemberSubjects[0] != "user-finance" {
		t.Fatalf("unexpected Department membership: %#v", groups[1].MemberSubjects)
	}
	for _, request := range requests {
		if len(request) < len(http.MethodGet) || (request[:len(http.MethodGet)] != http.MethodGet && request[:len(http.MethodPost)] != http.MethodPost) {
			t.Fatalf("identity synchronization issued a mutation: %s", request)
		}
	}
}
