package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

// New share conversations use the same direct model pipeline as the owner's
// Assistant chat. The visitor hash and Share Token revision are checked before
// every public read or write; private conversation routes exclude these rows.
func (service *Service) enqueueDirectPublicAnswer(ctx context.Context, assistant aiappdomain.SmartAssistant, visitorHash, conversationID, question string) (aiappdomain.AssistantConversation, aiappdomain.AssistantTurn, error) {
	var conversation aiappdomain.AssistantConversation
	var err error
	if conversationID == "" {
		model, modelErr := service.resolveAssistantModel(ctx, assistant.OwnerID, assistant.ProviderModelID)
		if modelErr != nil {
			return conversation, aiappdomain.AssistantTurn{}, modelErr
		}
		conversation, err = service.aiapplications.CreatePublicAssistantConversation(ctx, assistant.OwnerID, assistant.ID, visitorHash, assistant.Share.TokenRevision, model)
	} else {
		conversation, err = service.aiapplications.GetPublicAssistantConversation(ctx, assistant.OwnerID, assistant.ID, conversationID, visitorHash, assistant.Share.TokenRevision)
	}
	if err != nil {
		return conversation, aiappdomain.AssistantTurn{}, err
	}
	turn, err := service.aiapplications.BeginAssistantTurn(ctx, assistant.OwnerID, conversation.ID, question)
	if err != nil {
		return conversation, turn, err
	}
	if err := service.releaseInterruptedAssistantCredits(ctx, assistant.OwnerID, conversation.ID); err != nil {
		_, _ = service.aiapplications.FinishAssistantTurn(context.WithoutCancel(ctx), assistant.OwnerID, conversation.ID, turn.ID, "failed", "", "", "", 0, 0)
		return conversation, turn, err
	}
	go service.finishDirectPublicAnswer(context.WithoutCancel(ctx), conversation, turn)
	return conversation, turn, nil
}

func (service *Service) finishDirectPublicAnswer(parent context.Context, conversation aiappdomain.AssistantConversation, turn aiappdomain.AssistantTurn) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	answer, answerErr := service.answerAssistantTurn(ctx, conversation.OwnerID, conversation, turn, "", "public", func(string) error { return nil })
	state := assistantTurnState(answerErr)
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer finishCancel()
	_, _ = service.aiapplications.FinishAssistantTurn(finishCtx, conversation.OwnerID, conversation.ID, turn.ID, state, answer.source, answer.faqID, answer.text, answer.inputTokens, answer.outputTokens)
}

func (service *Service) directPublicResponse(ctx context.Context, assistant aiappdomain.SmartAssistant, visitorHash, conversationID, turnID string) (map[string]any, error) {
	conversation, err := service.aiapplications.GetPublicAssistantConversation(ctx, assistant.OwnerID, assistant.ID, conversationID, visitorHash, assistant.Share.TokenRevision)
	if err != nil {
		return nil, err
	}
	turns, err := service.aiapplications.ListAssistantTurns(ctx, conversation.OwnerID, conversation.ID)
	if err != nil {
		return nil, err
	}
	for _, turn := range turns {
		if turn.ID == turnID {
			return map[string]any{"kind": responseKind(turn.State), "response_id": turn.ID, "conversation_id": conversation.ID, "state": turn.State, "answer": turn.Answer}, nil
		}
	}
	return nil, aiappdomain.ErrNotFound
}

func (service *Service) publicAnswer(ctx context.Context, assistant aiappdomain.SmartAssistant, visitorHash, conversationID, question string) (map[string]any, error) {
	if conversationID != "" {
		_, err := service.aiapplications.GetPublicAssistantConversation(ctx, assistant.OwnerID, assistant.ID, conversationID, visitorHash, assistant.Share.TokenRevision)
		if err != nil {
			if !errors.Is(err, aiappdomain.ErrNotFound) {
				return nil, err
			}
			// Conversations started before the direct model path retain their
			// original Session and provider snapshot for historical continuity.
			legacy, legacyErr := service.aiapplications.GetExternalConversation(ctx, conversationID, assistant.ID, visitorHash)
			if legacyErr != nil {
				return nil, legacyErr
			}
			if legacy.ShareTokenRevision != assistant.Share.TokenRevision {
				return nil, aiappdomain.ErrNotFound
			}
			_ = service.aiapplications.RecordSafetyAudit(ctx, assistant.OwnerID, assistant.ID, "public", aiappdomain.SafetyAllow, "pending")
			response, enqueueErr := service.enqueueExternalAnswer(ctx, assistant, visitorHash, conversationID, question)
			if enqueueErr != nil {
				return nil, enqueueErr
			}
			return map[string]any{"kind": "generating", "response_id": response.ID, "conversation_id": response.ConversationID, "state": response.State}, nil
		}
	}
	conversation, turn, err := service.enqueueDirectPublicAnswer(ctx, assistant, visitorHash, conversationID, question)
	if err != nil {
		return nil, fmt.Errorf("enqueue public Assistant answer: %w", err)
	}
	return map[string]any{"kind": "generating", "response_id": turn.ID, "conversation_id": conversation.ID, "state": turn.State}, nil
}
