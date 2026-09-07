package server

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/transportmeta"
	"google.golang.org/protobuf/proto"
)

func TestArchiveJSONThroughFilterAndDecoder(t *testing.T) {
	archive := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 80*1024)))
	for _, test := range []struct {
		method string
		path   string
		body   string
		input  proto.Message
	}{
		{http.MethodPost, "/api/v1/skills", `{"source":"upload","archive":"` + archive + `"}`, &workspacev1.CreateSkillRequest{}},
		{http.MethodPatch, "/api/v1/skills/skill-1", `{"archive":"` + archive + `","expected_version":1}`, &workspacev1.UpdateSkillRequest{}},
		{http.MethodPost, "/api/v1/admin/connectors/cli", `{"definition":{"archive":"` + archive + `"}}`, &workspacev1.CreateCLIConnectorDefinitionRequest{}},
		{http.MethodPatch, "/api/v1/admin/connectors/cli/cli-1", `{"definition":{"archive":"` + archive + `"},"expected_version":1}`, &workspacev1.UpdateCLIConnectorDefinitionRequest{}},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.ContentLength = -1
			response := httptest.NewRecorder()
			rawBodyFilter(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if err := decodeStrictJSONRequest(request, test.input); err != nil {
					t.Fatal(err)
				}
				body, ok := transportmeta.RawBodyFromContext(request.Context())
				if !ok || string(body) != test.body {
					t.Fatal("archive body was truncated")
				}
				writer.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestArchiveJSONRejectsUnknownFieldsAndMalformedBase64(t *testing.T) {
	for _, body := range []string{`{"archive":"not base64!"}`, `{"unknown":true}`, `{} {}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/skills", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if err := decodeStrictJSONRequest(request, &workspacev1.CreateSkillRequest{}); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

type endlessBody struct{ read int }

func (body *endlessBody) Read(buffer []byte) (int, error) {
	clear(buffer)
	body.read += len(buffer)
	return len(buffer), nil
}

func TestArchiveJSONRejectsOversizeChunkedBodyWithBoundedRead(t *testing.T) {
	body := &endlessBody{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/skills", body)
	request.ContentLength = -1
	response := httptest.NewRecorder()
	rawBodyFilter(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("oversized request reached handler") })).ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || response.Body.String() != "{\"error\":\"request_body_too_large\"}\n" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if body.read != transportmeta.MaxArchiveJSONBody+1 {
		t.Fatalf("read=%d", body.read)
	}
}

func TestStrictJSONRequestDecoderPreservesRawBody(t *testing.T) {
	body := `{"workflow":{"name":"Daily report","goal":"Summarize changes"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var input workspacev1.CreateWorkflowRequest
	if err := decodeStrictJSONRequest(request, &input); err != nil {
		t.Fatal(err)
	}
	transportmeta.RestoreRawBody(request)
	preserved, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(preserved) != body || input.Workflow.GetName() != "Daily report" {
		t.Fatalf("decoded workflow=%q raw=%q", input.Workflow.GetName(), preserved)
	}
}

func TestStrictJSONRequestDecoderFailsClosed(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "wrong content type", contentType: "text/plain", body: `{}`},
		{name: "unknown field", contentType: "application/json", body: `{"unknown":true}`},
		{name: "multiple documents", contentType: "application/json", body: `{} {}`},
		{name: "empty", contentType: "application/json", body: ``},
		{name: "oversized", contentType: "application/json", body: `{"runtime":"` + strings.Repeat("x", transportmeta.MaxJSONBody) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if err := decodeStrictJSONRequest(request, &workspacev1.CreateWorkflowRequest{}); err == nil {
				t.Fatal("decoder accepted invalid request")
			}
		})
	}
}

func TestPublicErrorEncoderDoesNotExposeCause(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/runtime-images", nil)
	response := httptest.NewRecorder()
	encodePublicError(response, request, errWithSecret("postgres://user:secret@database"))
	if response.Code != http.StatusInternalServerError || response.Body.String() != "{\"error\":\"internal_error\"}\n" {
		t.Fatalf("encoded error = (%d, %q)", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("unsafe error response headers=%v body=%q", response.Header(), response.Body.String())
	}
}

func TestResponseEncoderWritesExplicitCompatibilityMapping(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/v1/runtime-images", nil)
	response := httptest.NewRecorder()
	response.Header().Set("X-Agent-Platform-Internal-Response-Status", "201")
	response.Header().Set("X-Agent-Platform-Internal-Response-Body", `{"id":"runtime-1","version":1}`)
	if err := encodeResponse(response, request, &workspacev1.Workflow{}); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusCreated || response.Body.String() != "{\"id\":\"runtime-1\",\"version\":1}\n" {
		t.Fatalf("mapped response = (%d, %q)", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Agent-Platform-Internal-Response-Body") != "" {
		t.Fatal("internal response body escaped into public headers")
	}
}

type errWithSecret string

func (err errWithSecret) Error() string { return string(err) }
