package execution

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"

	"github.com/gin-gonic/gin"
)

type internalEventReaderFunc func(context.Context, string, int64, int) ([]domain.Event, error)

func (function internalEventReaderFunc) ListEventsAfter(ctx context.Context, runID string, cursor int64, limit int) ([]domain.Event, error) {
	return function(ctx, runID, cursor, limit)
}

type internalRunAccessFunc func(context.Context, string, string) error

func (function internalRunAccessFunc) AuthorizeRunRead(ctx context.Context, token, runID string) error {
	return function(ctx, token, runID)
}

type internalRunStateFunc func(context.Context, string) (domain.Details, error)

func (function internalRunStateFunc) Get(ctx context.Context, runID string) (domain.Details, error) {
	return function(ctx, runID)
}

func TestRunEventSSEEmitsHeartbeatAndStopsOnCancellation(t *testing.T) {
	calls := 0
	reader := internalEventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
		calls++
		if calls > 1 {
			return nil, context.Canceled
		}
		return nil, nil
	})
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/v1/runs/:run_id/events", func(ctx *gin.Context) {
		serveRunEventsAtIntervals(ctx, reader, internalRunStateFunc(func(context.Context, string) (domain.Details, error) {
			return domain.Details{State: domain.Running}, nil
		}), internalRunAccessFunc(func(context.Context, string, string) error { return nil }), time.Hour, time.Millisecond)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if !strings.Contains(response.Body.String(), ": keepalive\n\n") || calls != 2 {
		t.Fatalf("heartbeat body=%q calls=%d", response.Body.String(), calls)
	}
}

func TestRunEventSSEDeliversAnEventArrivingAfterEmptyHistory(t *testing.T) {
	calls := 0
	reader := internalEventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return []domain.Event{{Sequence: 1, Type: "run.completed", Payload: []byte(`{}`), CreatedAt: time.Now()}}, nil
	})
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/v1/runs/:run_id/events", func(ctx *gin.Context) {
		serveRunEventsAtIntervals(ctx, reader, internalRunStateFunc(func(context.Context, string) (domain.Details, error) {
			return domain.Details{State: domain.Running}, nil
		}), internalRunAccessFunc(func(context.Context, string, string) error { return nil }), time.Millisecond, time.Hour)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if calls != 2 || !strings.Contains(response.Body.String(), "event: run.completed") {
		t.Fatalf("real-time body=%q calls=%d", response.Body.String(), calls)
	}
}
