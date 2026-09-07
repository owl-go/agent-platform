package transportmeta

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONBodyLimitOnlyAllowsArchiveMutationRoutes(t *testing.T) {
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
