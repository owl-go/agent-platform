package transportmeta

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

const MaxJSONBody = 64 * 1024

type rawBodyKey struct{}

func CaptureRawBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, MaxJSONBody+1))
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
