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

func JSONBodyLimit(request *http.Request) int {
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
