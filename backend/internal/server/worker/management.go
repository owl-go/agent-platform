package worker

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
)

type ReadinessChecker interface {
	PingContext(context.Context) error
}

func NewManagementServer(address string, readiness ReadinessChecker, state *State) (*kratoshttp.Server, error) {
	if readiness == nil || state == nil {
		return nil, fmt.Errorf("Worker database readiness and loop State are required")
	}
	server := kratoshttp.NewServer(
		kratoshttp.Address(address),
		kratoshttp.Timeout(0),
	)
	router := server.Route("/")
	router.GET("/healthz", func(ctx kratoshttp.Context) error {
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	router.GET("/readyz", func(ctx kratoshttp.Context) error {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if !state.Ready() || readiness.PingContext(checkCtx) != nil {
			return ctx.JSON(http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		}
		return ctx.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})
	router.GET("/metrics", func(ctx kratoshttp.Context) error {
		var metrics strings.Builder
		metrics.WriteString("# TYPE agent_platform_worker_up gauge\nagent_platform_worker_up 1\n")
		metrics.WriteString("# TYPE agent_platform_worker_loop_started gauge\n")
		metrics.WriteString("# TYPE agent_platform_worker_loop_fatal gauge\n")
		for _, status := range state.Statuses() {
			fmt.Fprintf(&metrics, "agent_platform_worker_loop_started{loop=%q} %d\n", status.Name, boolMetric(status.Started))
			fmt.Fprintf(&metrics, "agent_platform_worker_loop_fatal{loop=%q} %d\n", status.Name, boolMetric(status.Fatal))
		}
		return ctx.String(http.StatusOK, metrics.String())
	})
	return server, nil
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}
