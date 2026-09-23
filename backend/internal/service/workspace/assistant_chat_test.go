package workspace

import (
	"context"
	"fmt"
	"testing"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
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

func TestAssistantModelSelectionRequiresAvailableOpenAIChatModelAndKey(t *testing.T) {
	for _, test := range []struct {
		name      string
		protocols []string
		available bool
		hasKey    bool
		valid     bool
	}{
		{name: "OpenAI Chat", protocols: []string{"openai_chat", "openai_responses"}, available: true, hasKey: true, valid: true},
		{name: "Responses only", protocols: []string{"openai_responses"}, available: true, hasKey: true},
		{name: "Unavailable model", protocols: []string{"openai_chat"}, hasKey: true},
		{name: "Missing key", protocols: []string{"openai_chat"}, available: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			connections := []workspacedomain.ModelProviderConnection{{ID: "connection-1", CredentialOwnerID: "admin", ProviderType: "openai", Endpoint: "https://example.test/v1", Protocols: test.protocols, HasAPIKey: test.hasKey, Version: 3, Models: []workspacedomain.ProviderModel{{ID: "model-1", ModelID: "chat-model", Available: test.available}}}}
			model, err := selectAssistantModel(connections, "model-1")
			if test.valid {
				if err != nil || model.ProviderModelID != "model-1" || model.Protocol != "openai_chat" || model.ConnectionVersion != 3 {
					t.Fatalf("model = %+v, err = %v", model, err)
				}
			} else if err == nil {
				t.Fatalf("unexpected model: %+v", model)
			}
		})
	}
	if _, err := selectAssistantModel(nil, "other-user-or-missing-model"); err == nil {
		t.Fatal("missing model was accepted")
	}
}
