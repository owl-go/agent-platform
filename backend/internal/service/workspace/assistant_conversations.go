package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
)

func (service *Service) handleAssistantConversations(writer http.ResponseWriter, request *http.Request, owner, assistantID string, rest []string) {
	ctx := request.Context()
	if len(rest) == 0 {
		switch request.Method {
		case http.MethodGet:
			items, err := service.aiapplications.ListAssistantConversations(ctx, owner, assistantID)
			service.writeAIResult(writer, map[string]any{"items": items}, err)
		case http.MethodPost:
			assistant, err := service.aiapplications.GetAssistant(ctx, owner, assistantID)
			if err != nil {
				service.writeAIResult(writer, nil, err)
				return
			}
			model, err := service.resolveAssistantModel(ctx, owner, assistant.ProviderModelID)
			if err != nil {
				if errors.Is(err, aiappdomain.ErrInvalid) {
					writeAuthError(writer, http.StatusUnprocessableEntity, "assistant_model_unavailable")
					return
				}
				service.writeAIResult(writer, nil, err)
				return
			}
			conversation, err := service.aiapplications.CreateAssistantConversation(ctx, owner, assistantID, model)
			service.writeAIResult(writer, conversation, err)
		default:
			writer.Header().Set("Allow", "GET, POST")
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	conversation, err := service.aiapplications.GetAssistantConversation(ctx, owner, assistantID, rest[0])
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	if len(rest) == 1 && request.Method == http.MethodGet {
		turns, turnErr := service.aiapplications.ListAssistantTurns(ctx, owner, conversation.ID)
		if turnErr != nil {
			service.writeAIResult(writer, nil, turnErr)
			return
		}
		faqs, faqErr := service.aiapplications.ListFAQs(ctx, owner, assistantID)
		if faqErr != nil && !errors.Is(faqErr, aiappdomain.ErrNotFound) {
			service.writeAIResult(writer, nil, faqErr)
			return
		}
		visible := make([]aiappdomain.FAQ, 0, len(faqs))
		for _, faq := range faqs {
			if faq.Enabled {
				visible = append(visible, faq)
			}
		}
		service.writeAIResult(writer, map[string]any{"conversation": conversation, "turns": turns, "faqs": visible}, nil)
		return
	}
	if len(rest) == 2 && rest[1] == "turns" && request.Method == http.MethodPost {
		service.streamAssistantTurn(writer, request, owner, conversation)
		return
	}
	if len(rest) == 4 && rest[1] == "turns" && rest[3] == "cancel" && request.Method == http.MethodPost {
		if err := service.aiapplications.CancelAssistantTurn(ctx, owner, conversation.ID, rest[2]); err != nil {
			service.writeAIResult(writer, nil, err)
			return
		}
		if cancel, ok := service.activeAssistantTurns.Load(rest[2]); ok {
			cancel.(context.CancelFunc)()
		}
		service.writeAIResult(writer, map[string]bool{"cancelled": true}, nil)
		return
	}
	http.NotFound(writer, request)
}

func (service *Service) streamAssistantTurn(writer http.ResponseWriter, request *http.Request, owner string, conversation aiappdomain.AssistantConversation) {
	current, err := service.aiapplications.GetAssistant(request.Context(), owner, conversation.AssistantID)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	if current.State != aiappdomain.StateEnabled {
		service.writeAIResult(writer, nil, aiappdomain.ErrConflict)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	var input struct {
		Question string `json:"question"`
		FAQID    string `json:"faq_id"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	if input.FAQID != "" {
		faqs, faqErr := service.aiapplications.ListFAQs(request.Context(), owner, conversation.AssistantID)
		if faqErr != nil {
			service.writeAIResult(writer, nil, faqErr)
			return
		}
		valid := false
		for _, faq := range faqs {
			if faq.ID == input.FAQID && faq.Enabled && faq.Question == strings.TrimSpace(input.Question) {
				valid = true
				break
			}
		}
		if !valid {
			service.writeAIResult(writer, nil, aiappdomain.ErrInvalid)
			return
		}
	}
	_, ok := writer.(http.Flusher)
	if !ok {
		writeAuthError(writer, http.StatusInternalServerError, "stream_unavailable")
		return
	}
	turn, err := service.aiapplications.BeginAssistantTurn(request.Context(), owner, conversation.ID, input.Question)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	if err := service.releaseInterruptedAssistantCredits(request.Context(), owner, conversation.ID); err != nil {
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(request.Context()), 10*time.Second)
		_, _ = service.aiapplications.FinishAssistantTurn(finishCtx, owner, conversation.ID, turn.ID, "failed", "", "", "", "assistant_failed", 0, 0)
		finishCancel()
		service.writeAIResult(writer, nil, err)
		return
	}
	service.streamAcceptedAssistantTurn(writer, request, owner, conversation, turn, input.FAQID, "authenticated", false)
}

func (service *Service) streamAcceptedAssistantTurn(writer http.ResponseWriter, request *http.Request, owner string, conversation aiappdomain.AssistantConversation, turn aiappdomain.AssistantTurn, faqID, accessSource string, public bool) {
	flusher := writer.(http.Flusher)
	ctx, cancel := context.WithCancel(request.Context())
	service.activeAssistantTurns.Store(turn.ID, context.CancelFunc(cancel))
	defer func() { service.activeAssistantTurns.Delete(turn.ID); cancel() }()
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store, no-transform")
	writer.Header().Set("X-Accel-Buffering", "no")
	if err := writeAssistantEvent(writer, flusher, "thinking", map[string]any{"turn_id": turn.ID, "conversation_id": conversation.ID, "message": "思考中..."}); err != nil {
		finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer finishCancel()
		_, _ = service.aiapplications.FinishAssistantTurn(finishCtx, owner, conversation.ID, turn.ID, "cancelled", "", "", "", "", 0, 0)
		return
	}
	answer, answerErr := service.answerAssistantTurn(ctx, owner, conversation, turn, faqID, accessSource, func(delta string) error {
		return writeAssistantEvent(writer, flusher, "delta", map[string]string{"turn_id": turn.ID, "text": delta})
	})
	state := assistantTurnState(answerErr)
	if ctx.Err() != nil {
		state = "cancelled"
	}
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	failureCode := assistantTurnFailureCode(answerErr)
	completed, finishErr := service.aiapplications.FinishAssistantTurn(finishCtx, owner, conversation.ID, turn.ID, state, answer.source, answer.faqID, answer.text, failureCode, answer.inputTokens, answer.outputTokens)
	if finishErr != nil {
		_ = writeAssistantEvent(writer, flusher, "error", map[string]string{"message": "对话记录保存失败，请刷新重试"})
		return
	}
	if answerErr != nil && !errors.Is(answerErr, context.Canceled) {
		_ = writeAssistantEvent(writer, flusher, "error", map[string]string{"code": failureCode, "message": "对话失败，请重试"})
	}
	if public {
		_ = writeAssistantEvent(writer, flusher, "done", map[string]any{"id": completed.ID, "conversation_id": completed.ConversationID, "answer": completed.Answer, "state": completed.State, "failure_code": completed.Error})
		return
	}
	_ = writeAssistantEvent(writer, flusher, "done", completed)
}

func (service *Service) releaseInterruptedAssistantCredits(ctx context.Context, owner, conversationID string) error {
	turns, err := service.aiapplications.ListAssistantTurns(ctx, owner, conversationID)
	if err != nil {
		return err
	}
	for _, turn := range turns {
		if turn.State != "failed" || turn.Error != "interrupted" {
			continue
		}
		for stage := 1; stage <= 3; stage++ {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			err := service.credits.Abort(cleanupCtx, creditsdomain.Admission{UserID: owner, Source: fmt.Sprintf("assistant-turn-%s:%d", turn.ID, stage)})
			cancel()
			if err != nil {
				return fmt.Errorf("release interrupted Assistant Credits: %w", err)
			}
		}
		if err := service.aiapplications.MarkAssistantInterruptedCreditsReleased(ctx, owner, conversationID, turn.ID); err != nil {
			return err
		}
	}
	return nil
}

func writeAssistantEvent(writer io.Writer, flusher http.Flusher, name string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", name, encoded); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
