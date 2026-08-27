package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/transportmeta"
)

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
