package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"
	executionservice "agent-platform/backend/internal/service/execution"
)

type eventReaderFunc func(context.Context, string, int64, int) ([]domain.Event, error)

func (function eventReaderFunc) ListEventsAfter(ctx context.Context, runID string, cursor int64, limit int) ([]domain.Event, error) {
	return function(ctx, runID, cursor, limit)
}

func TestGinRunEventSSEAuthorizesBeforeStreamingHeaders(t *testing.T) {
	handler, err := executionservice.NewRunEventSSE(
		eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
			t.Fatal("event reader called before authorization")
			return nil, nil
		}),
		runAccessFunc(func(context.Context, string, string) error { return errors.New("identity unavailable") }),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Content-Type") == "text/event-stream" {
		t.Fatalf("authorization response=(%d, %q, %q)", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
}

func TestGinRunEventSSEUsesLastEventIDAndReportsInternalReadFailure(t *testing.T) {
	runID := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	reader := eventReaderFunc(func(ctx context.Context, gotRunID string, cursor int64, limit int) ([]domain.Event, error) {
		deadline, ok := ctx.Deadline()
		if gotRunID != runID || cursor != 7 || limit != 100 || !ok || time.Until(deadline) > 5*time.Second {
			t.Fatalf("query=(%q,%d,%d) deadline=%v", gotRunID, cursor, limit, deadline)
		}
		return nil, errors.New("database unavailable")
	})
	handler, err := executionservice.NewRunEventSSE(reader, runAccessFunc(func(context.Context, string, string) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Last-Event-ID", "7")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "event: stream_error\ndata: {\"error\":\"event_stream_failed\"}\n\n" {
		t.Fatalf("stream failure response=(%d, %q)", response.Code, response.Body.String())
	}
}

type runAccessFunc func(context.Context, string, string) error

func (function runAccessFunc) AuthorizeRunRead(ctx context.Context, token, runID string) error {
	return function(ctx, token, runID)
}

func TestGinRunEventSSEPreservesCursorAndFrameContract(t *testing.T) {
	runID := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	calls := 0
	reader := eventReaderFunc(func(_ context.Context, gotRunID string, cursor int64, limit int) ([]domain.Event, error) {
		calls++
		if gotRunID != runID || limit != 100 {
			t.Fatalf("ListEventsAfter(%q, %d, %d)", gotRunID, cursor, limit)
		}
		if calls > 1 {
			return nil, context.Canceled
		}
		if cursor != 4 {
			t.Fatalf("first cursor = %d, want 4", cursor)
		}
		return []domain.Event{{Sequence: 5, Type: "run.running", Payload: json.RawMessage("{\n\"attempt\":1\n}"), CreatedAt: time.Now()}}, nil
	})
	access := runAccessFunc(func(_ context.Context, token, gotRunID string) error {
		if token != "token" || gotRunID != runID {
			t.Fatalf("AuthorizeRunRead(%q, %q)", token, gotRunID)
		}
		return nil
	})
	handler, err := executionservice.NewRunEventSSE(reader, access)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	request, err := http.NewRequest(http.MethodGet, server.URL+"/v1/runs/"+runID+"/events?after=4", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Last-Event-ID", "3")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := "id: 5\nevent: run.running\ndata: {\ndata: \"attempt\":1\ndata: }\n\n"
	if response.StatusCode != http.StatusOK || string(body) != want {
		t.Fatalf("SSE response = (%d, %q), want (%d, %q)", response.StatusCode, body, http.StatusOK, want)
	}
	if response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("X-Accel-Buffering = %q", response.Header.Get("X-Accel-Buffering"))
	}
}
