package application

import (
	"context"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

// AssistantConversationRepository persists every accepted turn. Only the
// bounded context passed to a model is pruned; the audit transcript is not.
type AssistantConversationRepository interface {
	CreateAssistantConversation(context.Context, domain.AssistantConversation) (domain.AssistantConversation, error)
	ListAssistantConversations(context.Context, string, string) ([]domain.AssistantConversation, error)
	GetAssistantConversation(context.Context, string, string) (domain.AssistantConversation, error)
	ListAssistantTurns(context.Context, string, string) ([]domain.AssistantTurn, error)
	BeginAssistantTurn(context.Context, string, string, string) (domain.AssistantTurn, error)
	SaveAssistantTurnProgress(context.Context, string, string, string, string) error
	FinishAssistantTurn(context.Context, string, string, string, string, string, string, string, int64, int64) (domain.AssistantTurn, error)
	CancelAssistantTurn(context.Context, string, string, string) error
	SaveAssistantSummary(context.Context, string, string, int, string) error
	MarkAssistantInterruptedCreditsReleased(context.Context, string, string, string) error
}

func (service *Service) conversations() (AssistantConversationRepository, error) {
	repository, ok := service.repository.(AssistantConversationRepository)
	if !ok {
		return nil, fmt.Errorf("Assistant Conversation repository is unavailable")
	}
	return repository, nil
}

func (service *Service) CreateAssistantConversation(ctx context.Context, owner, assistantID string, model domain.AssistantModel) (domain.AssistantConversation, error) {
	assistant, err := service.GetAssistant(ctx, owner, assistantID)
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	if assistant.State != domain.StateEnabled {
		return domain.AssistantConversation{}, fmt.Errorf("%w: Assistant is not enabled", domain.ErrConflict)
	}
	if err := assistant.ValidateForEnable(); err != nil {
		return domain.AssistantConversation{}, err
	}
	repository, err := service.conversations()
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	return repository.CreateAssistantConversation(ctx, domain.AssistantConversation{OwnerID: owner, AssistantID: assistant.ID, AssistantName: assistant.Name, Welcome: assistant.Introduction, AssistantSnapshot: assistant, ModelSnapshot: model})
}

func (service *Service) ListAssistantConversations(ctx context.Context, owner, assistantID string) ([]domain.AssistantConversation, error) {
	repository, err := service.conversations()
	if err != nil {
		return nil, err
	}
	return repository.ListAssistantConversations(ctx, owner, assistantID)
}

func (service *Service) GetAssistantConversation(ctx context.Context, owner, assistantID, conversationID string) (domain.AssistantConversation, error) {
	repository, err := service.conversations()
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	conversation, err := repository.GetAssistantConversation(ctx, owner, conversationID)
	if err != nil {
		return domain.AssistantConversation{}, err
	}
	if conversation.AssistantID != assistantID {
		return domain.AssistantConversation{}, domain.ErrNotFound
	}
	return conversation, nil
}

func (service *Service) ListAssistantTurns(ctx context.Context, owner, conversationID string) ([]domain.AssistantTurn, error) {
	repository, err := service.conversations()
	if err != nil {
		return nil, err
	}
	return repository.ListAssistantTurns(ctx, owner, conversationID)
}

func (service *Service) BeginAssistantTurn(ctx context.Context, owner, conversationID, question string) (domain.AssistantTurn, error) {
	question = strings.TrimSpace(question)
	if question == "" || len([]rune(question)) > 4000 {
		return domain.AssistantTurn{}, fmt.Errorf("%w: question must contain 1-4000 characters", domain.ErrInvalid)
	}
	repository, err := service.conversations()
	if err != nil {
		return domain.AssistantTurn{}, err
	}
	return repository.BeginAssistantTurn(ctx, owner, conversationID, question)
}

func (service *Service) FinishAssistantTurn(ctx context.Context, owner, conversationID, turnID, state, source, faqID, answer string, inputTokens, outputTokens int64) (domain.AssistantTurn, error) {
	repository, err := service.conversations()
	if err != nil {
		return domain.AssistantTurn{}, err
	}
	return repository.FinishAssistantTurn(ctx, owner, conversationID, turnID, state, source, faqID, answer, inputTokens, outputTokens)
}

func (service *Service) SaveAssistantTurnProgress(ctx context.Context, owner, conversationID, turnID, answer string) error {
	repository, err := service.conversations()
	if err != nil {
		return err
	}
	return repository.SaveAssistantTurnProgress(ctx, owner, conversationID, turnID, answer)
}

func (service *Service) CancelAssistantTurn(ctx context.Context, owner, conversationID, turnID string) error {
	repository, err := service.conversations()
	if err != nil {
		return err
	}
	return repository.CancelAssistantTurn(ctx, owner, conversationID, turnID)
}

func (service *Service) SaveAssistantSummary(ctx context.Context, owner, conversationID string, throughTurn int, summary string) error {
	repository, err := service.conversations()
	if err != nil {
		return err
	}
	return repository.SaveAssistantSummary(ctx, owner, conversationID, throughTurn, summary)
}

func (service *Service) MarkAssistantInterruptedCreditsReleased(ctx context.Context, owner, conversationID, turnID string) error {
	repository, err := service.conversations()
	if err != nil {
		return err
	}
	return repository.MarkAssistantInterruptedCreditsReleased(ctx, owner, conversationID, turnID)
}
