package server

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"agent-platform/backend/internal/transportmeta"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/google/uuid"
)

var routeIDPattern = regexp.MustCompile(`(?i)/[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)

const defaultUnaryTimeout = 30 * time.Second

type httpMetrics struct {
	requests atomic.Uint64
	failures atomic.Uint64
}

func apiFilters(logger *slog.Logger, authentication kratoshttp.FilterFunc) []kratoshttp.FilterFunc {
	metrics := &httpMetrics{}
	return []kratoshttp.FilterFunc{
		recoveryFilter(logger),
		securityHeadersFilter,
		requestIDFilter,
		traceSeamFilter,
		metricsFilter(metrics),
		accessLogFilter(logger),
		authentication,
		unaryTimeoutFilter(defaultUnaryTimeout),
		rawBodyFilter,
	}
}

func securityHeadersFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}

func unaryTimeoutFilter(timeout time.Duration) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if strings.HasPrefix(request.URL.Path, "/v1/runs/") && strings.HasSuffix(request.URL.Path, "/events") {
				next.ServeHTTP(writer, request)
				return
			}
			ctx, cancel := context.WithTimeout(request.Context(), timeout)
			defer cancel()
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}
}

func rawBodyFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead || request.Method == http.MethodOptions {
			next.ServeHTTP(writer, request)
			return
		}
		body, err := transportmeta.CaptureRawBody(request)
		if err != nil || len(body) > transportmeta.MaxJSONBody {
			writePublicError(writer, http.StatusBadRequest, "invalid_request_body")
			return
		}
		next.ServeHTTP(writer, transportmeta.WithRawBody(request, body))
	})
}

func recoveryFilter(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("http panic recovered", "request_id", request.Header.Get("X-Request-ID"), "panic_type", fmt.Sprintf("%T", recovered), "stack", string(debug.Stack()))
					writePublicError(writer, http.StatusInternalServerError, "internal_error")
				}
			}()
			next.ServeHTTP(writer, request)
		})
	}
}

func requestIDFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if len(requestID) == 0 || len(requestID) > 128 || strings.ContainsAny(requestID, "\r\n") {
			requestID = uuid.NewString()
		}
		request.Header.Set("X-Request-ID", requestID)
		writer.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(writer, request)
	})
}

func traceSeamFilter(next http.Handler) http.Handler { return next }

func metricsFilter(metrics *httpMetrics) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			metrics.requests.Add(1)
			status := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(status, request)
			if status.status >= http.StatusInternalServerError {
				metrics.failures.Add(1)
			}
		})
	}
}

func accessLogFilter(logger *slog.Logger) kratoshttp.FilterFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			started := time.Now()
			status := &statusWriter{ResponseWriter: writer, status: http.StatusOK}
			next.ServeHTTP(status, request)
			logger.Info("http request",
				"method", request.Method,
				"route", routeIDPattern.ReplaceAllString(request.URL.Path, "/{id}"),
				"status", status.status,
				"duration_ms", time.Since(started).Milliseconds(),
				"request_id", request.Header.Get("X-Request-ID"),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (writer *statusWriter) WriteHeader(status int) {
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Flush()                      { _ = http.NewResponseController(writer.ResponseWriter).Flush() }
func (writer *statusWriter) Unwrap() http.ResponseWriter { return writer.ResponseWriter }
func (writer *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(writer.ResponseWriter).Hijack()
}

func writePublicError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = fmt.Fprintf(writer, "{\"error\":%q}\n", code)
}
