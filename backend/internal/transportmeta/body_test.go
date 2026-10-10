package transportmeta

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONBodyLimitOnlyAllowsArchiveMutationRoutes(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		large  bool
	}{
		{http.MethodPost, "/api/v1/admin/connectors/packages", true},
		{http.MethodPost, "/api/v1/admin/connectors/packages/", false},
		{http.MethodGet, "/api/v1/admin/connectors/packages", false},
		{http.MethodPost, "/api/v1/admin/connectors/publications", false},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		want := MaxJSONBody
		if test.large {
			want = MaxArchiveJSONBody
		}
		if got := JSONBodyLimit(request); got != want {
			t.Fatalf("%s %s limit=%d, want %d", test.method, test.path, got, want)
		}
	}
	for _, path := range []string{"/api/v1/skills", "/api/v1/admin/connectors/cli"} {
		for _, test := range []struct {
			method string
			suffix string
			large  bool
		}{
			{http.MethodPost, "", true},
			{http.MethodPatch, "/resource-1", true},
			{http.MethodPost, "/", false},
			{http.MethodPost, "/resource-1", false},
			{http.MethodPatch, "", false},
			{http.MethodPatch, "/", false},
			{http.MethodPatch, "/..", false},
			{http.MethodPatch, "/resource-1/publish", false},
			{http.MethodPatch, "/resource-1%2Fpublish", false},
			{http.MethodDelete, "/resource-1", false},
			{http.MethodGet, "", false},
		} {
			t.Run(test.method+path+test.suffix, func(t *testing.T) {
				request := httptest.NewRequest(test.method, path+test.suffix, nil)
				want := MaxJSONBody
				if test.large {
					want = MaxArchiveJSONBody
				}
				if got := JSONBodyLimit(request); got != want {
					t.Fatalf("limit=%d, want %d", got, want)
				}
			})
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", nil)
	if JSONBodyLimit(request) != 64*1024 {
		t.Fatal("ordinary JSON request limit changed")
	}
}

func TestExpertMutationBodyLimitsCoverPortableArchivesAndValidatedProfiles(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/v1/expert-packages/import", MaxExpertPackageJSONBody},
		{http.MethodPost, "/api/v1/experts", MaxExpertProfileJSONBody},
		{http.MethodPatch, "/api/v1/experts/id", MaxExpertProfileJSONBody},
		{http.MethodPost, "/api/v1/expert-teams", MaxExpertTeamJSONBody},
		{http.MethodPatch, "/api/v1/expert-teams/id", MaxExpertTeamJSONBody},
		{http.MethodPost, "/api/v1/resource-creation-actions/id/decision", MaxExpertProfileJSONBody},
		{http.MethodGet, "/api/v1/expert-packages/import", MaxJSONBody},
		{http.MethodPost, "/api/v1/expert-packages/import/", MaxJSONBody},
		{http.MethodPatch, "/api/v1/experts/id/extra", MaxJSONBody},
		{http.MethodPost, "/api/v1/resource-creation-actions/id/extra/decision", MaxJSONBody},
	} {
		if got := JSONBodyLimit(httptest.NewRequest(tc.method, tc.path, nil)); got != tc.want {
			t.Errorf("%s %s=%d want=%d", tc.method, tc.path, got, tc.want)
		}
	}
}
