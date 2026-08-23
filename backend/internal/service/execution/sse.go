package execution

import (
	"context"
	"encoding/json"
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

type RunStateReader interface {
	Get(context.Context, string) (executiondomain.Details, error)
}

type RunAccessController interface {
	AuthorizeRunRead(context.Context, string, string) error
}

func NewRunEventSSE(reader EventReader, states RunStateReader, access RunAccessController) (http.Handler, error) {
	if reader == nil || states == nil || access == nil {
		return nil, fmt.Errorf("Run Event Reader, State Reader, and Access Controller are required")
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.GET("/v1/runs/:run_id/events", func(ctx *gin.Context) {
		serveRunEvents(ctx, reader, states, access)
	})
	return engine, nil
}

func serveRunEvents(ctx *gin.Context, reader EventReader, states RunStateReader, access RunAccessController) {
	serveRunEventsAtIntervals(ctx, reader, states, access, time.Second, 15*time.Second)
}

func serveRunEventsAtIntervals(ctx *gin.Context, reader EventReader, states RunStateReader, access RunAccessController, pollInterval, heartbeatInterval time.Duration) {
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

	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		if err := access.AuthorizeRunRead(ctx.Request.Context(), token, runID); err != nil {
			writeStreamError(ctx, authorizationStreamError(err))
			return
		}
		queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
		events, err := reader.ListEventsAfter(queryCtx, runID, cursor, 100)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Request.Context().Err() != nil {
				return
			}
			writeStreamError(ctx, "event_stream_failed")
			return
		}
		previous := cursor
		terminal := false
		for _, event := range events {
			if event.Sequence != previous+1 || terminal {
				writeStreamError(ctx, "event_contract_invalid")
				return
			}
			eventType := strings.NewReplacer("\n", "", "\r", "").Replace(event.Type)
			data, err := json.Marshal(map[string]any{
				"run_id": runID, "sequence": event.Sequence, "event_type": event.Type,
				"payload": json.RawMessage(event.Payload), "created_at": event.CreatedAt.UTC(),
			})
			if err != nil {
				writeStreamError(ctx, "event_contract_invalid")
				return
			}
			if _, err := fmt.Fprintf(ctx.Writer, "id: %d\nevent: %s\n", event.Sequence, eventType); err != nil {
				return
			}
			if _, err := fmt.Fprintf(ctx.Writer, "data: %s\n", data); err != nil {
				return
			}
			if _, err := fmt.Fprint(ctx.Writer, "\n"); err != nil {
				return
			}
			cursor = event.Sequence
			previous = event.Sequence
			terminal = isTerminalEvent(event.Type)
		}
		if len(events) > 0 {
			ctx.Writer.Flush()
			if terminal {
				if len(events) == 100 {
					if err := access.AuthorizeRunRead(ctx.Request.Context(), token, runID); err != nil {
						writeStreamError(ctx, authorizationStreamError(err))
						return
					}
					queryCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
					afterTerminal, err := reader.ListEventsAfter(queryCtx, runID, cursor, 1)
					cancel()
					if err != nil {
						writeStreamError(ctx, "event_stream_failed")
						return
					}
					if len(afterTerminal) != 0 {
						writeStreamError(ctx, "event_contract_invalid")
						return
					}
				}
				return
			}
			continue
		}
		stateCtx, stateCancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
		run, err := states.Get(stateCtx, runID)
		stateCancel()
		if err != nil {
			writeStreamError(ctx, "event_stream_failed")
			return
		}
		if run.State == executiondomain.Completed || run.State == executiondomain.Failed || run.State == executiondomain.Cancelled {
			writeStreamError(ctx, "event_terminal_missing")
			return
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

func isTerminalEvent(eventType string) bool {
	switch eventType {
	case "run.completed", "run.failed", "run.cancelled", "run.killed":
		return true
	default:
		return false
	}
}

func authorizationStreamError(err error) string {
	if errors.Is(err, identitydomain.ErrUnauthenticated) {
		return "invalid_authentication"
	}
	if errors.Is(err, identitydomain.ErrForbidden) {
		return "run_access_denied"
	}
	return "authorization_failed"
}

func writeStreamError(ctx *gin.Context, code string) {
	data, _ := json.Marshal(map[string]string{"error": code})
	_, _ = fmt.Fprintf(ctx.Writer, "event: stream_error\ndata: %s\n\n", data)
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
