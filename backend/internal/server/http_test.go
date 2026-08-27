package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/platformconfig"
	platformserver "agent-platform/backend/internal/server"
	platformservice "agent-platform/backend/internal/service/platform"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type readinessFunc func(context.Context) error

func (function readinessFunc) PingContext(ctx context.Context) error { return function(ctx) }

type routesFunc func(*kratoshttp.Server)

func (function routesFunc) RegisterHTTP(server *kratoshttp.Server) { function(server) }

type workspaceStub struct {
	workspacev1.UnimplementedAgentWorkspaceServiceServer
	created *workspacev1.CreateWorkflowRequest
}

func (stub *workspaceStub) CreateWorkflow(_ context.Context, request *workspacev1.CreateWorkflowRequest) (*workspacev1.Workflow, error) {
	stub.created = request
	return &workspacev1.Workflow{Name: request.Workflow.GetName()}, nil
}

func TestHealthAndReadinessAreServedByKratos(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		readiness  readinessFunc
		wantStatus int
		wantBody   string
	}{
		{name: "health", path: "/healthz", readiness: func(context.Context) error { return errors.New("offline") }, wantStatus: http.StatusOK, wantBody: "{\"status\":\"ok\"}\n"},
		{name: "ready", path: "/readyz", readiness: func(context.Context) error { return nil }, wantStatus: http.StatusOK, wantBody: "{\"status\":\"ready\"}\n"},
		{name: "not ready", path: "/readyz", readiness: func(context.Context) error { return errors.New("offline") }, wantStatus: http.StatusServiceUnavailable, wantBody: "{\"status\":\"unavailable\"}\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, err := platformservice.New(test.readiness)
			if err != nil {
				t.Fatal(err)
			}
			server := platformserver.NewHTTPServer(platformserver.HTTPConfig{}, service)
			upstream := httptest.NewServer(server)
			defer upstream.Close()

			response, err := upstream.Client().Get(upstream.URL + test.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != test.wantStatus || string(body) != test.wantBody {
				t.Fatalf("GET %s = (%d, %q), want (%d, %q)", test.path, response.StatusCode, body, test.wantStatus, test.wantBody)
			}
		})
	}
}

func TestHTTPServerMountsGinSSEBeforeBusinessPrefix(t *testing.T) {
	service, err := platformservice.New(readinessFunc(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	business := routesFunc(func(server *kratoshttp.Server) {
		server.Handle("/v1/runs", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
	})
	sse := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	})
	handlers, err := platformserver.NewHTTPHandlers(business, sse, func(next http.Handler) http.Handler { return next })
	if err != nil {
		t.Fatal(err)
	}
	config := platformconfig.Config{API: platformconfig.APIConfig{
		ReadHeaderTimeout: platformconfig.Duration(time.Second),
		IdleTimeout:       platformconfig.Duration(time.Second),
	}}
	server, err := platformserver.NewHTTPServerFromConfig(config, service, handlers, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/v1/runs/00000000-0000-4000-8000-000000000001/events", wantStatus: http.StatusOK},
		{path: "/v1/runs", wantStatus: http.StatusTeapot},
		{path: "/outside", wantStatus: http.StatusNotFound},
	} {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != test.wantStatus {
			t.Fatalf("GET %s status=%d, want %d", test.path, response.Code, test.wantStatus)
		}
	}
}

func TestHTTPServerRejectsMissingProtectedHandlers(t *testing.T) {
	service, err := platformservice.New(readinessFunc(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platformserver.NewHTTPServerFromConfig(platformconfig.Config{}, service, platformserver.HTTPHandlers{}, slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil {
		t.Fatal("NewHTTPServerFromConfig accepted missing protected handlers")
	}
}

func TestGeneratedWriteRouteUsesStrictProtoJSONBinding(t *testing.T) {
	service, err := platformservice.New(readinessFunc(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	stub := &workspaceStub{}
	business := routesFunc(func(server *kratoshttp.Server) {
		workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, stub)
	})
	handlers, err := platformserver.NewHTTPHandlers(business, http.NotFoundHandler(), func(next http.Handler) http.Handler { return next })
	if err != nil {
		t.Fatal(err)
	}
	server, err := platformserver.NewHTTPServerFromConfig(platformconfig.Config{}, service, handlers, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"workflow":{"name":"Daily report","goal":"Summarize changes"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST status=%d body=%q", response.Code, response.Body.String())
	}
	if stub.created == nil || stub.created.Workflow.GetName() != "Daily report" {
		t.Fatalf("created request workflow=%q", stub.created.GetWorkflow().GetName())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(`{"workflow":{"name":"Daily report","goal":"Summarize"},"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || response.Body.String() != "{\"error\":\"invalid_request_body\"}\n" {
		t.Fatalf("unknown field response=(%d, %q)", response.Code, response.Body.String())
	}

	for _, test := range []struct {
		name        string
		body        string
		contentType string
		wantCode    string
	}{
		{name: "missing content type", body: `{}`, wantCode: "application_json_required"},
		{name: "wrong content type", body: `{}`, contentType: "text/plain", wantCode: "application_json_required"},
		{name: "multiple documents", body: `{} {}`, contentType: "application/json", wantCode: "invalid_request_body"},
		{name: "oversized", body: `{"runtime":"` + strings.Repeat("x", 65*1024) + `"}`, contentType: "application/json", wantCode: "invalid_request_body"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			wantBody := "{\"error\":\"" + test.wantCode + "\"}\n"
			if response.Code != http.StatusBadRequest || response.Body.String() != wantBody {
				t.Fatalf("response=(%d, %q), want (400, %q)", response.Code, response.Body.String(), wantBody)
			}
		})
	}
}

func TestAuthenticationRunsBeforeGeneratedBodyBinding(t *testing.T) {
	service, err := platformservice.New(readinessFunc(func(context.Context) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	stub := &workspaceStub{}
	business := routesFunc(func(server *kratoshttp.Server) {
		workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, stub)
	})
	authentication := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusUnauthorized)
		})
	}
	handlers, err := platformserver.NewHTTPHandlers(business, http.NotFoundHandler(), authentication)
	if err != nil {
		t.Fatal(err)
	}
	server, err := platformserver.NewHTTPServerFromConfig(platformconfig.Config{}, service, handlers, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/workflows", strings.NewReader(`{"unknown":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || stub.created != nil {
		t.Fatalf("response status=%d created=%v", response.Code, stub.created != nil)
	}
}
