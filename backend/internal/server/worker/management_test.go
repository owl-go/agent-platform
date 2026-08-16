package worker_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	workerserver "agent-platform/backend/internal/server/worker"
)

type managementReadinessFunc func(context.Context) error

func (function managementReadinessFunc) PingContext(ctx context.Context) error { return function(ctx) }

func TestManagementServerReportsDatabaseReadinessWithoutDeepSideEffects(t *testing.T) {
	checker := managementReadinessFunc(func(context.Context) error { return errors.New("database unavailable") })
	state := workerserver.NewState()
	loop, err := workerserver.NewLoopWithState("reconcile", time.Hour, func(context.Context) (bool, error) { return false, nil }, state)
	if err != nil {
		t.Fatal(err)
	}
	loopDone := make(chan error, 1)
	go func() { loopDone <- loop.Start(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for !state.Ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	t.Cleanup(func() {
		_ = loop.Stop(context.Background())
		<-loopDone
	})
	server, err := workerserver.NewManagementServer("", checker, state)
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(server)
	defer upstream.Close()

	for _, test := range []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/healthz", wantStatus: http.StatusOK, wantBody: `"status":"ok"`},
		{path: "/readyz", wantStatus: http.StatusServiceUnavailable, wantBody: `"status":"unavailable"`},
		{path: "/metrics", wantStatus: http.StatusOK, wantBody: "agent_platform_worker_up 1"},
	} {
		response, err := upstream.Client().Get(upstream.URL + test.path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != test.wantStatus || !strings.Contains(string(body), test.wantBody) {
			t.Fatalf("GET %s = (%d, %q)", test.path, response.StatusCode, body)
		}
	}
}

func TestManagementReadinessTracksStartupAndFatalLoopState(t *testing.T) {
	checker := managementReadinessFunc(func(context.Context) error { return nil })
	state := workerserver.NewState()
	loop, err := workerserver.NewLoopWithState("execution", time.Hour, func(context.Context) (bool, error) {
		return false, workerserver.Fatal(errors.New("fatal"))
	}, state)
	if err != nil {
		t.Fatal(err)
	}
	server, err := workerserver.NewManagementServer("", checker, state)
	if err != nil {
		t.Fatal(err)
	}

	assertReadyStatus := func(want int) {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != want {
			t.Fatalf("readiness status=%d body=%s, want %d", response.Code, response.Body.String(), want)
		}
	}
	assertReadyStatus(http.StatusServiceUnavailable)
	if err := loop.Start(context.Background()); err == nil {
		t.Fatal("fatal loop returned nil")
	}
	assertReadyStatus(http.StatusServiceUnavailable)
}
