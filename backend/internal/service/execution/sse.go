package execution

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	executiondomain "agent-platform/backend/internal/biz/execution/domain"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type EventReader interface {
	ListEventsAfter(context.Context, string, int64, int) ([]executiondomain.Event, error)
}

type RunAccessController interface {
	AuthorizeRunRead(context.Context, string, string) error
}

func NewRunEventSSE(reader EventReader, access RunAccessController) (http.Handler, error) {
	if reader == nil || access == nil {
		return nil, fmt.Errorf("Run Event Reader and Access Controller are required")
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/v1/runs/:run_id/events", func(ctx *gin.Context) {
		serveRunEvents(ctx, reader, access)
	})
	return engine, nil
}

func serveRunEvents(ctx *gin.Context, reader EventReader, access RunAccessController) {
	runID := ctx.Param("run_id")
	if _, err := uuid.Parse(runID); err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_run_id"})
		return
	}
	token, ok := bearerToken(ctx.Request)
	if !ok {
		ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return
	}
	switch err := access.AuthorizeRunRead(ctx.Request.Context(), token, runID); {
	case err == nil:
	case errors.Is(err, identitydomain.ErrUnauthenticated):
		ctx.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_authentication"})
		return
	case errors.Is(err, identitydomain.ErrForbidden):
		ctx.JSON(http.StatusForbidden, map[string]string{"error": "run_access_denied"})
		return
	default:
		ctx.JSON(http.StatusServiceUnavailable, map[string]string{"error": "authorization_failed"})
		return
	}
	cursor, err := eventCursor(ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_event_cursor"})
		return
	}

	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache, no-store")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")
	ctx.Status(http.StatusOK)
	ctx.Writer.Flush()

	poll := time.NewTicker(time.Second)
	defer poll.Stop()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
		events, err := reader.ListEventsAfter(queryCtx, runID, cursor, 100)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Request.Context().Err() != nil {
				return
			}
			writeStreamError(ctx)
			return
		}
		previous := cursor
		for _, event := range events {
			if event.Sequence <= previous {
				writeStreamError(ctx)
				return
			}
			eventType := strings.NewReplacer("\n", "", "\r", "").Replace(event.Type)
			if _, err := fmt.Fprintf(ctx.Writer, "id: %d\nevent: %s\n", event.Sequence, eventType); err != nil {
				return
			}
			for _, line := range strings.Split(string(event.Payload), "\n") {
				if _, err := fmt.Fprintf(ctx.Writer, "data: %s\n", line); err != nil {
					return
				}
			}
			if _, err := fmt.Fprint(ctx.Writer, "\n"); err != nil {
				return
			}
			cursor = event.Sequence
			previous = event.Sequence
		}
		if len(events) > 0 {
			ctx.Writer.Flush()
			continue
		}
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-poll.C:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(ctx.Writer, ": keepalive\n\n"); err != nil {
				return
			}
			ctx.Writer.Flush()
		}
	}
}

func writeStreamError(ctx *gin.Context) {
	_, _ = fmt.Fprint(ctx.Writer, "event: stream_error\ndata: {\"error\":\"event_stream_failed\"}\n\n")
	ctx.Writer.Flush()
}

func eventCursor(request *http.Request) (int64, error) {
	value := request.URL.Query().Get("after")
	if value == "" {
		value = request.Header.Get("Last-Event-ID")
	}
	if value == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		return 0, fmt.Errorf("invalid Run Event cursor")
	}
	return cursor, nil
}

func bearerToken(request *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(request.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
