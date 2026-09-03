package workspace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (service *Service) streamRunEvents(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 7 {
		http.NotFound(writer, request)
		return
	}
	workflowID, runID := parts[3], parts[5]
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	if _, err := service.workspace.Repository().GetRun(request.Context(), owner, workflowID, runID); err != nil {
		writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		return
	}
	cursor, _ := strconv.ParseInt(request.Header.Get("Last-Event-ID"), 10, 64)
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	ticker := time.NewTicker(150 * time.Millisecond)
	heartbeat := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer heartbeat.Stop()
	for {
		events, err := service.workspace.Repository().ListRunEvents(request.Context(), owner, workflowID, runID, cursor, 100)
		if err != nil {
			writeSSE(writer, cursor, "stream.error", []byte(`{"error":"event_stream_failed"}`))
			flusher.Flush()
			return
		}
		for _, event := range events {
			writeSSE(writer, event.Sequence, event.Type, event.Payload)
			cursor = event.Sequence
			flusher.Flush()
			if strings.HasPrefix(event.Type, "run.") && (event.Type == "run.succeeded" || event.Type == "run.failed" || event.Type == "run.cancelled") {
				return
			}
		}
		select {
		case <-request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(writer, ": heartbeat\n\n")
			flusher.Flush()
		case <-ticker.C:
		}
	}
}

func writeSSE(writer http.ResponseWriter, sequence int64, eventType string, payload []byte) {
	if !json.Valid(payload) {
		payload = []byte(`{}`)
	}
	_, _ = fmt.Fprintf(writer, "id: %d\nevent: %s\ndata: %s\n\n", sequence, eventType, payload)
}
