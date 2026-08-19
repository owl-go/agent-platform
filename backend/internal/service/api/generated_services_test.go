package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	modelcatalogv1 "agent-platform/backend/api/modelcatalog/v1"
	runtimecatalogv1 "agent-platform/backend/api/runtimecatalog/v1"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestGeneratedServiceCanBeCalledWithoutHTTPContext(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	service := &GeneratedServices{dependencies: Dependencies{
		RuntimeImages: runtimeImageReaderStub{image: runtimedomain.RuntimeImage{ID: id, OrganizationID: "org-1", Version: 1}},
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

func (stub runtimeImageReaderStub) Get(context.Context, string, string) (runtimedomain.RuntimeImage, error) {
	return stub.image, nil
}

func (stub runtimeImageReaderStub) List(context.Context, runtimeapplication.ListQuery) (runtimeapplication.Page, error) {
	return runtimeapplication.Page{Items: []runtimedomain.RuntimeImage{stub.image}}, nil
}

type runtimeAccessFunc func(context.Context, string) (identitydomain.Actor, error)

func (function runtimeAccessFunc) AuthorizeRuntimeImageRead(ctx context.Context, token string) (identitydomain.Actor, error) {
	return function(ctx, token)
}

var allowRuntimeImageRead runtimeAccessFunc = func(context.Context, string) (identitydomain.Actor, error) {
	return identitydomain.Actor{UserID: "user-1", OrganizationID: "org-1"}, nil
}

func TestGeneratedRuntimeRoutePreservesLegacyHTTPContract(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	service := &GeneratedServices{dependencies: Dependencies{
		RuntimeImages: runtimeImageReaderStub{image: runtimedomain.RuntimeImage{ID: id, OrganizationID: "org-1"}},
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

func TestRuntimeImageListReturnsPaginationToken(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000001"
	reader := &recordingRuntimeImageReader{image: runtimedomain.RuntimeImage{ID: id}, nextToken: "next-page"}
	service := &GeneratedServices{dependencies: Dependencies{RuntimeImages: reader, RuntimeAccess: allowRuntimeImageRead}}

	response, err := service.ListRuntimeImages(context.Background(), &runtimecatalogv1.ListRuntimeImagesRequest{PageSize: 7, PageToken: "current-page"})
	if err != nil || response.NextPageToken != "next-page" || len(response.Items) != 1 {
		t.Fatalf("ListRuntimeImages() = (%+v, %v)", response, err)
	}
	if reader.query.PageSize != 7 || reader.query.Token != "current-page" {
		t.Fatalf("List query = %+v", reader.query)
	}
	if reader.query.OrganizationID != "org-1" {
		t.Fatalf("List query Organization = %q", reader.query.OrganizationID)
	}
}

func TestModelCatalogListHidesTeamScopedCredentials(t *testing.T) {
	teamID := "team-2"
	reader := modelCatalogReaderStub{credentials: []modeldomain.CredentialProfile{
		{ID: "organization-model", OrganizationID: "org-1", Name: "shared", Kind: modeldomain.ModelCredential},
		{ID: "team-model", OrganizationID: "org-1", TeamID: &teamID, Name: "private", Kind: modeldomain.ModelCredential},
		{ID: "organization-git", OrganizationID: "org-1", Name: "git", Kind: modeldomain.GitSSHCredential},
	}}
	service := &GeneratedServices{dependencies: Dependencies{ModelCatalog: reader, ModelAccess: allowModelCatalogRead}}

	response, err := service.ListCredentialProfiles(context.Background(), &modelcatalogv1.ListCredentialProfilesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Id != "organization-model" {
		t.Fatalf("visible Credential Profiles = %+v", response.Items)
	}
}

type modelCatalogReaderStub struct {
	credentials []modeldomain.CredentialProfile
}

func (stub modelCatalogReaderStub) GetCredential(context.Context, string, string) (modeldomain.CredentialProfile, error) {
	return stub.credentials[0], nil
}

func (stub modelCatalogReaderStub) ListCredentials(context.Context, string) ([]modeldomain.CredentialProfile, error) {
	return stub.credentials, nil
}

func (modelCatalogReaderStub) GetModel(context.Context, string, string) (modeldomain.ConfiguredModel, error) {
	return modeldomain.ConfiguredModel{}, nil
}

func (modelCatalogReaderStub) ListModels(context.Context, string) ([]modeldomain.ConfiguredModel, error) {
	return nil, nil
}

type modelCatalogAccessFunc func(context.Context, string) (identitydomain.Actor, error)

func (function modelCatalogAccessFunc) AuthorizeModelCatalogRead(ctx context.Context, token string) (identitydomain.Actor, error) {
	return function(ctx, token)
}

var allowModelCatalogRead modelCatalogAccessFunc = func(context.Context, string) (identitydomain.Actor, error) {
	return identitydomain.Actor{UserID: "user-1", OrganizationID: "org-1"}, nil
}

type recordingRuntimeImageReader struct {
	image     runtimedomain.RuntimeImage
	nextToken string
	query     runtimeapplication.ListQuery
}

func (reader *recordingRuntimeImageReader) Get(context.Context, string, string) (runtimedomain.RuntimeImage, error) {
	return reader.image, nil
}

func (reader *recordingRuntimeImageReader) List(_ context.Context, query runtimeapplication.ListQuery) (runtimeapplication.Page, error) {
	reader.query = query
	return runtimeapplication.Page{Items: []runtimedomain.RuntimeImage{reader.image}, NextToken: reader.nextToken}, nil
}

func TestGeneratedServicesRejectMissingSecurityGraph(t *testing.T) {
	if _, err := NewGeneratedServices(Dependencies{}); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("NewGeneratedServices() error = %v", err)
	}
}
