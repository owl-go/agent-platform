package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimecatalogv1 "agent-platform/backend/api/runtimecatalog/v1"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestGeneratedServiceCanBeCalledWithoutHTTPContext(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	service := &GeneratedServices{dependencies: Dependencies{
		RuntimeImages: runtimeImageReaderStub{image: runtimedomain.RuntimeImage{ID: id, Version: 1}},
		RuntimeAccess: allowRuntimeImageRead,
	}}
	response, err := service.GetRuntimeImage(context.Background(), &runtimecatalogv1.GetRuntimeImageRequest{RuntimeImageId: id})
	if err != nil {
		t.Fatal(err)
	}
	if response.Id != id || response.Version != 1 {
		t.Fatalf("response = %+v", response)
	}
}

type runtimeImageReaderStub struct{ image runtimedomain.RuntimeImage }

func (stub runtimeImageReaderStub) Get(context.Context, string) (runtimedomain.RuntimeImage, error) {
	return stub.image, nil
}

func (stub runtimeImageReaderStub) List(context.Context) ([]runtimedomain.RuntimeImage, error) {
	return []runtimedomain.RuntimeImage{stub.image}, nil
}

type runtimeAccessFunc func(context.Context, string) error

func (function runtimeAccessFunc) AuthorizeRuntimeImageRead(ctx context.Context, token string) error {
	return function(ctx, token)
}

var allowRuntimeImageRead runtimeAccessFunc = func(context.Context, string) error { return nil }

func TestGeneratedRuntimeRoutePreservesLegacyHTTPContract(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	service := &GeneratedServices{dependencies: Dependencies{
		RuntimeImages: runtimeImageReaderStub{image: runtimedomain.RuntimeImage{ID: id}},
		RuntimeAccess: allowRuntimeImageRead,
	}}
	server := kratoshttp.NewServer(kratoshttp.ResponseEncoder(func(writer http.ResponseWriter, _ *http.Request, _ any) error {
		body := writer.Header().Get(internalResponseBodyHeader)
		writer.Header().Del(internalResponseBodyHeader)
		writer.Header().Del(internalResponseStatusHeader)
		_, err := writer.Write([]byte(body))
		return err
	}))
	service.RegisterHTTP(server)

	request := httptest.NewRequest(http.MethodGet, "/v1/runtime-images/"+id, nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), id) {
		t.Fatalf("GET generated Runtime Image route = (%d, %q)", response.Code, response.Body.String())
	}
}

func TestGeneratedServicesRejectMissingSecurityGraph(t *testing.T) {
	if _, err := NewGeneratedServices(Dependencies{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("NewGeneratedServices() error = %v", err)
	}
}
