package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	executionv1 "agent-platform/backend/api/execution/v1"
	modelcatalogv1 "agent-platform/backend/api/modelcatalog/v1"
	runtimecatalogv1 "agent-platform/backend/api/runtimecatalog/v1"
	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	modeldomain "agent-platform/backend/internal/biz/modelcatalog/domain"
	runtimeapplication "agent-platform/backend/internal/biz/runtimecatalog/application"
	runtimedomain "agent-platform/backend/internal/biz/runtimecatalog/domain"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

func TestRunSearchReturnsStableCursorAndFrozenDiagnostics(t *testing.T) {
	firstID := "00000000-0000-4000-8000-000000000101"
	secondID := "00000000-0000-4000-8000-000000000102"
	createdAt := time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
	searcher := &runSearcherStub{values: []executiondomain.Details{
		{ID: firstID, SessionID: "session-1", CodingTaskID: "task-1", AgentID: "agent-1", AgentReleaseID: "release-1", RuntimeImageID: "runtime-1", RepositoryBindingID: "binding-1", State: executiondomain.Completed, CreatedAt: createdAt, UpdatedAt: createdAt, Version: 1, RepositoryBinding: []byte(`{"name":"Payments"}`), RuntimeImage: []byte(`{"runtime":"codex","image_digest":"registry/codex@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`), ConfiguredModel: []byte(`{"model_id":"gpt-enterprise"}`)},
		{ID: secondID, SessionID: "session-2", State: executiondomain.Completed, CreatedAt: createdAt.Add(-time.Minute), UpdatedAt: createdAt, Version: 1},
	}}
	service := &GeneratedServices{dependencies: Dependencies{RunSearch: searcher, RunSearchAccess: allowTeamRunRead}}
	limit := int32(1)
	direction := "desc"
	response, err := service.ListRuns(context.Background(), &executionv1.ListRunsRequest{TeamId: "team-1", Limit: &limit, SortDirection: &direction})
	if err != nil || len(response.Items) != 1 || response.NextPageToken == "" {
		t.Fatalf("ListRuns() = (%+v, %v)", response, err)
	}
	if response.Items[0].CodingTaskId != "task-1" || response.Items[0].RuntimeImageSnapshot.GetFields()["image_digest"].GetStringValue() == "" {
		t.Fatalf("Run diagnostics = %+v", response.Items[0])
	}
	searcher.values = nil
	response, err = service.ListRuns(context.Background(), &executionv1.ListRunsRequest{TeamId: "team-1", Limit: &limit, SortDirection: &direction, PageToken: &response.NextPageToken})
	if err != nil || searcher.query.CursorCreatedAt == nil || searcher.query.CursorID != firstID {
		t.Fatalf("cursor query = (%+v, %v)", searcher.query, err)
	}
}

type runSearcherStub struct {
	values []executiondomain.Details
	query  executiondomain.SearchQuery
}

func (stub *runSearcherStub) Search(_ context.Context, query executiondomain.SearchQuery) ([]executiondomain.Details, error) {
	stub.query = query
	return stub.values, nil
}

type runSearchAccessFunc func(context.Context, string, string) (identitydomain.Actor, error)

func (function runSearchAccessFunc) AuthorizeTeamRead(ctx context.Context, token, teamID string) (identitydomain.Actor, error) {
	return function(ctx, token, teamID)
}

var allowTeamRunRead runSearchAccessFunc = func(context.Context, string, string) (identitydomain.Actor, error) {
	return identitydomain.Actor{UserID: "user-1", OrganizationID: "org-1"}, nil
}

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
