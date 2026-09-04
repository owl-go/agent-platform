package workspace

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type sessionMessageSnapshot struct {
	State             string                    `json:"state"`
	Content           string                    `json:"content"`
	Error             string                    `json:"error,omitempty"`
	ProgressStage     string                    `json:"progress_stage,omitempty"`
	ElapsedMS         int64                     `json:"elapsed_ms"`
	ExpertStages      []domain.ExpertStage      `json:"expert_stages,omitempty"`
	CreditConsumption *domain.CreditConsumption `json:"credit_consumption,omitempty"`
}

func (service *Service) streamSessionMessage(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 7 || parts[2] != "sessions" || parts[4] != "messages" || parts[6] != "events" {
		http.NotFound(writer, request)
		return
	}
	messageID, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil || messageID <= 0 {
		http.NotFound(writer, request)
		return
	}
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	sessionID := parts[3]
	message, err := service.workspace.Repository().GetMessage(request.Context(), owner, sessionID, messageID)
	if err != nil || message.Role != "assistant" {
		writeAuthError(writer, http.StatusNotFound, "resource_not_found")
		return
	}

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
	sequence := int64(0)
	last := ""
	for {
		current, err := service.workspace.Repository().GetMessage(request.Context(), owner, sessionID, messageID)
		if err != nil {
			writeSSE(writer, sequence, "stream.error", []byte(`{"error":"message_stream_failed"}`))
			flusher.Flush()
			return
		}
		payload, _ := json.Marshal(snapshotOf(current))
		fingerprint := string(payload)
		if fingerprint != last {
			sequence++
			writeSSE(writer, sequence, "message.snapshot", payload)
			flusher.Flush()
			last = fingerprint
		}
		if terminalMessageState(current.State) {
			return
		}
		select {
		case <-request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = writer.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		case <-ticker.C:
		}
	}
}

func snapshotOf(message domain.Message) sessionMessageSnapshot {
	return sessionMessageSnapshot{State: message.State, Content: message.Content, Error: message.Error, ProgressStage: message.ProgressStage, ElapsedMS: message.ElapsedMS, ExpertStages: message.ExpertStages, CreditConsumption: message.CreditConsumption}
}

func terminalMessageState(state string) bool {
	return state == "completed" || state == "failed" || state == "cancelled"
}
