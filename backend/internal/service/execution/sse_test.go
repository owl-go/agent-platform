package execution_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/execution/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	executionservice "agent-platform/backend/internal/service/execution"
)

type eventReaderFunc func(context.Context, string, int64, int) ([]domain.Event, error)

func (function eventReaderFunc) ListEventsAfter(ctx context.Context, runID string, cursor int64, limit int) ([]domain.Event, error) {
	return function(ctx, runID, cursor, limit)
}

type runStateReaderFunc func(context.Context, string) (domain.Details, error)

func (function runStateReaderFunc) Get(ctx context.Context, runID string) (domain.Details, error) {
	return function(ctx, runID)
}

var runningStateReader runStateReaderFunc = func(context.Context, string) (domain.Details, error) {
	return domain.Details{State: domain.Running}, nil
}

func TestGinRunEventSSEAuthorizesBeforeStreamingHeaders(t *testing.T) {
	handler, err := executionservice.NewRunEventSSE(
		eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
			t.Fatal("event reader called before authorization")
			return nil, nil
		}),
		runningStateReader,
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
	handler, err := executionservice.NewRunEventSSE(reader, runningStateReader, runAccessFunc(func(context.Context, string, string) error { return nil }))
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
	createdAt := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
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
		return []domain.Event{{Sequence: 5, Type: "run.running", Payload: json.RawMessage(`{"attempt":1}`), CreatedAt: createdAt}}, nil
	})
	access := runAccessFunc(func(_ context.Context, token, gotRunID string) error {
		if token != "token" || gotRunID != runID {
			t.Fatalf("AuthorizeRunRead(%q, %q)", token, gotRunID)
		}
		return nil
	})
	handler, err := executionservice.NewRunEventSSE(reader, runningStateReader, access)
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
	want := "id: 5\nevent: run.running\ndata: {\"created_at\":\"2026-08-23T08:00:00Z\",\"event_type\":\"run.running\",\"payload\":{\"attempt\":1},\"run_id\":\"6ba7b810-9dad-11d1-80b4-00c04fd430c8\",\"sequence\":5}\n\n"
	if response.StatusCode != http.StatusOK || string(body) != want {
		t.Fatalf("SSE response = (%d, %q), want (%d, %q)", response.StatusCode, body, http.StatusOK, want)
	}
	if response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("X-Accel-Buffering = %q", response.Header.Get("X-Accel-Buffering"))
	}
}

func TestGinRunEventSSERejectsSequenceGapsAndStopsAtTerminal(t *testing.T) {
	runID := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	tests := []struct {
		name   string
		events []domain.Event
		want   string
	}{
		{name: "gap", events: []domain.Event{{Sequence: 2, Type: "run.running", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, want: "event: stream_error"},
		{name: "terminal", events: []domain.Event{{Sequence: 1, Type: "run.completed", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, want: "event: run.completed"},
		{name: "cancelled terminal", events: []domain.Event{{Sequence: 1, Type: "run.cancelled", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, want: "event: run.cancelled"},
		{name: "rollback", events: []domain.Event{{Sequence: 1, Type: "run.running", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}, {Sequence: 1, Type: "command.started", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, want: "event: stream_error"},
		{name: "after terminal", events: []domain.Event{{Sequence: 1, Type: "run.completed", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}, {Sequence: 2, Type: "run.completed", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, want: "event: stream_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, err := executionservice.NewRunEventSSE(eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
				return test.events, nil
			}), runningStateReader, runAccessFunc(func(context.Context, string, string) error { return nil }))
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/v1/runs/"+runID+"/events", nil)
			request.Header.Set("Authorization", "Bearer token")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("SSE body = %q, want %q", response.Body.String(), test.want)
			}
		})
	}
}

func TestGinRunEventSSERejectsEventsAfterTerminalAcrossPages(t *testing.T) {
	events := make([]domain.Event, 100)
	for index := range events {
		events[index] = domain.Event{Sequence: int64(index + 1), Type: "run.running", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}
	}
	events[99].Type = "run.completed"
	reader := eventReaderFunc(func(_ context.Context, _ string, cursor int64, _ int) ([]domain.Event, error) {
		if cursor == 0 {
			return events, nil
		}
		return []domain.Event{{Sequence: 101, Type: "run.completed", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, nil
	})
	authorizations := 0
	handler, err := executionservice.NewRunEventSSE(reader, runningStateReader, runAccessFunc(func(context.Context, string, string) error {
		authorizations++
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if authorizations != 3 || !strings.Contains(response.Body.String(), `"error":"event_contract_invalid"`) {
		t.Fatalf("SSE did not reject event after paginated terminal: %q", response.Body.String())
	}
}

func TestGinRunEventSSEReportsMissingTerminalForFinishedRun(t *testing.T) {
	handler, err := executionservice.NewRunEventSSE(
		eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) { return nil, nil }),
		runStateReaderFunc(func(context.Context, string) (domain.Details, error) {
			return domain.Details{State: domain.Completed}, nil
		}),
		runAccessFunc(func(context.Context, string, string) error { return nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), `"error":"event_terminal_missing"`) {
		t.Fatalf("missing terminal response = %q", response.Body.String())
	}
}

func TestGinRunEventSSEReauthorizesBeforeEveryEventBatch(t *testing.T) {
	authorizations := 0
	reads := 0
	handler, err := executionservice.NewRunEventSSE(
		eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
			reads++
			return []domain.Event{{Sequence: 1, Type: "run.running", Payload: json.RawMessage(`{}`), CreatedAt: time.Now()}}, nil
		}),
		runningStateReader,
		runAccessFunc(func(context.Context, string, string) error {
			authorizations++
			if authorizations >= 3 {
				return identitydomain.ErrForbidden
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if reads != 1 || authorizations != 3 || !strings.Contains(response.Body.String(), `"error":"run_access_denied"`) {
		t.Fatalf("reauthorization reads=%d authorizations=%d body=%q", reads, authorizations, response.Body.String())
	}
}

func TestGinRunEventSSEMapsAuthenticationAndAuthorizationFailures(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		access     error
		wantStatus int
		wantCode   string
	}{
		{name: "missing bearer", wantStatus: http.StatusUnauthorized, wantCode: "authentication_required"},
		{name: "expired identity", header: "Bearer expired", access: identitydomain.ErrUnauthenticated, wantStatus: http.StatusUnauthorized, wantCode: "invalid_authentication"},
		{name: "forbidden Run", header: "Bearer token", access: identitydomain.ErrForbidden, wantStatus: http.StatusForbidden, wantCode: "run_access_denied"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, err := executionservice.NewRunEventSSE(eventReaderFunc(func(context.Context, string, int64, int) ([]domain.Event, error) {
				t.Fatal("event reader must not run")
				return nil, nil
			}), runningStateReader, runAccessFunc(func(context.Context, string, string) error { return test.access }))
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/v1/runs/6ba7b810-9dad-11d1-80b4-00c04fd430c8/events", nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("response=(%d,%q), want (%d,%q)", response.Code, response.Body.String(), test.wantStatus, test.wantCode)
			}
		})
	}
}
