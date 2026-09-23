package workspace

import (
	"context"
	"fmt"
	"testing"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

func TestAssistantHistoryUsesOnlyLatestTenCompletedTurns(t *testing.T) {
	turns := make([]aiappdomain.AssistantTurn, 0, 14)
	for index := 1; index <= 12; index++ {
		turns = append(turns, aiappdomain.AssistantTurn{TurnNumber: index, Question: fmt.Sprintf("Q%d", index), Answer: fmt.Sprintf("A%d", index), State: "completed"})
	}
	turns = append(turns, aiappdomain.AssistantTurn{TurnNumber: 13, Question: "unfinished", State: "cancelled"})
	var service Service
	messages, usage, err := service.assistantHistory(context.Background(), "owner", aiappdomain.AssistantConversation{}, "turn", turns)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 20 || messages[0].Content != "Q3" || messages[19].Content != "A12" || usage.InputTokens != 0 {
		t.Fatalf("bounded history = %+v, usage = %+v", messages, usage)
	}
}

func TestAssistantChatProtocolPreference(t *testing.T) {
	if got := selectAssistantProtocol("claude", []string{"openai_chat", "anthropic_messages"}); got != "anthropic_messages" {
		t.Fatalf("Claude protocol = %q", got)
	}
	if got := selectAssistantProtocol("codex", []string{"openai_chat", "openai_responses"}); got != "openai_responses" {
		t.Fatalf("Codex protocol = %q", got)
	}
}
