package application

import (
	"agent-platform/backend/internal/biz/workspace/domain"
	"context"
)

type ConversationRepository interface {
	GetConversationSelection(context.Context, string, domain.ConversationScope) (domain.ConversationSelection, error)
	ResolveConversationSelection(context.Context, string, domain.ConversationScope, domain.ConversationSelectionInput) (domain.ConversationSelection, error)
	CreateSelectedMessagePair(context.Context, string, string, string, []domain.Attachment, string) (domain.Message, domain.Message, error)
	ContinueSelectedRunConversation(context.Context, string, string, string, string, []domain.Attachment, string) (domain.Run, error)
}
