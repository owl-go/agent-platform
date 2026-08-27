package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformv1 "agent-platform/backend/api/platform/v1"
	"agent-platform/backend/internal/platformconfig"
	platformservice "agent-platform/backend/internal/service/platform"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type HTTPConfig struct {
	Address           string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
}

type HTTPHandlers struct {
	Business       APIRoutes
	RunSSE         http.Handler
	Authentication kratoshttp.FilterFunc
}

type APIRoutes interface {
	RegisterHTTP(*kratoshttp.Server)
}

func NewHTTPHandlers(business APIRoutes, runSSE http.Handler, authentication kratoshttp.FilterFunc) (HTTPHandlers, error) {
	if business == nil || runSSE == nil || authentication == nil {
		return HTTPHandlers{}, fmt.Errorf("Business API, Run Event SSE, and Authentication Filter are required")
	}
	return HTTPHandlers{Business: business, RunSSE: runSSE, Authentication: authentication}, nil
}

func NewWorkspaceHTTPHandlers(business APIRoutes, authentication kratoshttp.FilterFunc) (HTTPHandlers, error) {
	if business == nil || authentication == nil {
		return HTTPHandlers{}, fmt.Errorf("Agent Workspace API and Authentication Filter are required")
	}
	return HTTPHandlers{Business: business, Authentication: authentication}, nil
}

func NewHTTPServerFromConfig(config platformconfig.Config, platform *platformservice.Service, handlers HTTPHandlers, logger *slog.Logger) (*kratoshttp.Server, error) {
	if handlers.Business == nil || handlers.Authentication == nil || logger == nil {
		return nil, fmt.Errorf("Business API, Authentication Filter, and Logger are required")
	}
	server := newHTTPServer(HTTPConfig{
		Address:           config.API.Address,
		ReadHeaderTimeout: config.API.ReadHeaderTimeout.Value(),
		IdleTimeout:       config.API.IdleTimeout.Value(),
	}, platform, kratoshttp.Filter(apiFilters(logger, handlers.Authentication)...))
	handlers.Business.RegisterHTTP(server)
	if handlers.RunSSE != nil {
		server.Handle("/v1/runs/{run_id}/events", handlers.RunSSE)
	}
	return server, nil
}

func NewHTTPServer(config HTTPConfig, platform *platformservice.Service) *kratoshttp.Server {
	return newHTTPServer(config, platform)
}

func newHTTPServer(config HTTPConfig, platform *platformservice.Service, additional ...kratoshttp.ServerOption) *kratoshttp.Server {
	options := []kratoshttp.ServerOption{
		kratoshttp.Timeout(0),
		kratoshttp.RequestDecoder(decodeStrictJSONRequest),
		kratoshttp.ResponseEncoder(encodeResponse),
		kratoshttp.ErrorEncoder(encodePublicError),
	}
	if config.Address != "" {
		options = append(options, kratoshttp.Address(config.Address))
	}
	options = append(options, additional...)
	server := kratoshttp.NewServer(options...)
	server.ReadHeaderTimeout = config.ReadHeaderTimeout
	server.IdleTimeout = config.IdleTimeout
	platformv1.RegisterHealthServiceHTTPServer(server, platform)
	return server
}

func encodeResponse(writer http.ResponseWriter, _ *http.Request, value any) error {
	if body := writer.Header().Get("X-Agent-Platform-Internal-Response-Body"); body != "" {
		status, err := strconv.Atoi(writer.Header().Get("X-Agent-Platform-Internal-Response-Status"))
		if err != nil || status < 100 || status > 599 {
			return fmt.Errorf("invalid mapped response status")
		}
		writer.Header().Del("X-Agent-Platform-Internal-Response-Body")
		writer.Header().Del("X-Agent-Platform-Internal-Response-Status")
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		writer.WriteHeader(status)
		if _, err := bytes.NewBufferString(body).WriteTo(writer); err != nil {
			return err
		}
		if !strings.HasSuffix(body, "\n") {
			_, err = writer.Write([]byte("\n"))
			return err
		}
		return nil
	}
	status := http.StatusOK
	if rawStatus := writer.Header().Get("X-Agent-Platform-Internal-Response-Status"); rawStatus != "" {
		parsed, err := strconv.Atoi(rawStatus)
		if err != nil || parsed < 100 || parsed > 599 {
			return fmt.Errorf("invalid mapped response status")
		}
		status = parsed
		writer.Header().Del("X-Agent-Platform-Internal-Response-Status")
	}
	switch response := value.(type) {
	case *platformv1.ReadyResponse:
		if response.Status == "unavailable" {
			status = http.StatusServiceUnavailable
		}
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	return json.NewEncoder(writer).Encode(value)
}
