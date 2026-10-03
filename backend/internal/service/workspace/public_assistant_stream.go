package workspace

import (
	"context"
	"net/http"
	"strings"
	"time"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

func (service *Service) streamPublicAssistantTurn(writer http.ResponseWriter, request *http.Request, assistant aiappdomain.SmartAssistant, token, visitorHash string) {
	if _, ok := writer.(http.Flusher); !ok {
		writeAuthError(writer, http.StatusInternalServerError, "stream_unavailable")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	var input struct {
		Question       string `json:"question"`
		FAQID          string `json:"faq_id"`
		ConversationID string `json:"conversation_id"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	input.Question = strings.TrimSpace(input.Question)
	if input.Question == "" || len([]rune(input.Question)) > 4000 {
		service.writeAIResult(writer, nil, aiappdomain.ErrInvalid)
		return
	}
	if aiappdomain.DefaultSafetyPolicy().Decide(input.Question) == aiappdomain.SafetyRefuse {
		_ = service.aiapplications.RecordSafetyAudit(request.Context(), assistant.OwnerID, assistant.ID, "public", aiappdomain.SafetyRefuse, "not_charged")
		writePublicJSON(writer, map[string]string{"kind": "refusal", "answer": aiappdomain.SafetyRefusal}, http.StatusOK)
		return
	}
	faqs, err := service.aiapplications.ListFAQs(request.Context(), assistant.OwnerID, assistant.ID)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	faq, matched := matchAssistantExactFAQ(faqs, input.Question)
	if input.FAQID != "" {
		matched = false
		for _, candidate := range faqs {
			if candidate.Enabled && candidate.ID == input.FAQID && candidate.Question == input.Question {
				faq, matched = candidate, true
				break
			}
		}
		if !matched {
			service.writeAIResult(writer, nil, aiappdomain.ErrInvalid)
			return
		}
	}
	if matched {
		input.FAQID = faq.ID
	}
	if !matched && !assistant.Share.FreeTextEnabled {
		writeAuthError(writer, http.StatusForbidden, "free_text_disabled")
		return
	}
	var conversation aiappdomain.AssistantConversation
	if input.ConversationID != "" {
		conversation, err = service.aiapplications.GetPublicAssistantConversation(request.Context(), assistant.OwnerID, assistant.ID, input.ConversationID, visitorHash, assistant.Share.TokenRevision)
		if err != nil {
			service.writeAIResult(writer, nil, err)
			return
		}
	}
	if !service.consumePublicRate(writer, request, token, visitorHash) {
		return
	}
	if !matched {
		allowed, usageErr := service.aiapplications.ConsumeSharedAssistantCall(request.Context(), assistant.ID, assistant.Share.DailyCallLimit)
		if usageErr != nil {
			service.writeAIResult(writer, nil, usageErr)
			return
		}
		if !allowed {
			writeAuthError(writer, http.StatusTooManyRequests, "daily_call_limit_exceeded")
			return
		}
	}
	if input.ConversationID == "" {
		model, modelErr := service.resolveAssistantModel(request.Context(), assistant.OwnerID, assistant.ProviderModelID)
		if modelErr != nil {
			service.writeAssistantModelError(writer, modelErr)
			return
		}
		conversation, err = service.aiapplications.CreatePublicAssistantConversation(request.Context(), assistant.OwnerID, assistant.ID, visitorHash, assistant.Share.TokenRevision, model)
	}
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	turn, err := service.aiapplications.BeginAssistantTurn(request.Context(), assistant.OwnerID, conversation.ID, input.Question)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	if err = service.releaseInterruptedAssistantCredits(request.Context(), assistant.OwnerID, conversation.ID); err != nil {
		finishCtx, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 10*time.Second)
		defer cancel()
		_, _ = service.aiapplications.FinishAssistantTurn(finishCtx, assistant.OwnerID, conversation.ID, turn.ID, "failed", "", "", "", "assistant_failed", 0, 0)
		service.writeAIResult(writer, nil, err)
		return
	}
	service.streamAcceptedAssistantTurn(writer, request, assistant.OwnerID, conversation, turn, input.FAQID, "public", true)
}
