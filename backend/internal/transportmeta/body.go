package transportmeta

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

const MaxJSONBody = 64 * 1024

// Archive uploads encode at most 50 MiB as Base64, plus ordinary JSON metadata.
const MaxArchiveJSONBody = 4*((50*1024*1024+2)/3) + MaxJSONBody

// Portable Expert archives allow 100 MiB before Base64; direct profile writes
// allow a validated 2 MiB avatar, or one Team profile plus ten member profiles.
const MaxExpertPackageJSONBody = 4*((100*1024*1024+2)/3) + MaxJSONBody
const MaxExpertProfileJSONBody = 4 * 1024 * 1024
const MaxExpertTeamJSONBody = 40 * 1024 * 1024

func JSONBodyLimit(request *http.Request) int {
	if request.Method == http.MethodPost && request.URL.Path == "/api/v1/expert-packages/import" {
		return MaxExpertPackageJSONBody
	}
	for _, profile := range []struct {
		path  string
		limit int
	}{{"/api/v1/experts", MaxExpertProfileJSONBody}, {"/api/v1/expert-teams", MaxExpertTeamJSONBody}} {
		if request.Method == http.MethodPost && request.URL.Path == profile.path {
			return profile.limit
		}
		if request.Method == http.MethodPatch {
			id, ok := strings.CutPrefix(request.URL.Path, profile.path+"/")
			if ok && singleResourcePath(id) {
				return profile.limit
			}
		}
	}
	if request.Method == http.MethodPost {
		id, ok := strings.CutPrefix(request.URL.Path, "/api/v1/resource-creation-actions/")
		if ok {
			id, ok = strings.CutSuffix(id, "/decision")
			if ok && singleResourcePath(id) {
				return MaxExpertProfileJSONBody
			}
		}
	}

	if request.Method == http.MethodPost && request.URL.Path == "/api/v1/admin/connectors/packages" {
		return MaxArchiveJSONBody
	}
	for _, collection := range []string{"/api/v1/skills", "/api/v1/admin/connectors/cli"} {
		if request.Method == http.MethodPost && request.URL.Path == collection {
			return MaxArchiveJSONBody
		}
		if request.Method == http.MethodPatch {
			id, ok := strings.CutPrefix(request.URL.Path, collection+"/")
			if ok && id != "" && id != "." && id != ".." && !strings.Contains(id, "/") {
				return MaxArchiveJSONBody
			}
		}
	}
	return MaxJSONBody
}

type rawBodyKey struct{}

func CaptureRawBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, int64(JSONBodyLimit(request))+1))
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func WithRawBody(request *http.Request, body []byte) *http.Request {
	copyOfBody := append([]byte(nil), body...)
	return request.WithContext(context.WithValue(request.Context(), rawBodyKey{}, copyOfBody))
}

func RestoreRawBody(request *http.Request) {
	body, _ := request.Context().Value(rawBodyKey{}).([]byte)
	if body != nil {
		request.Body = io.NopCloser(bytes.NewReader(body))
	}
}

func RawBodyFromContext(ctx context.Context) ([]byte, bool) {
	body, ok := ctx.Value(rawBodyKey{}).([]byte)
	return append([]byte(nil), body...), ok
}

func singleResourcePath(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\")
}
