package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/retrieval"
)

type assistantKnowledgeSearcher struct {
	hits  []retrieval.Hit
	err   error
	calls int
}

func (searcher *assistantKnowledgeSearcher) Search(_ context.Context, owner, baseID string, generation int64, query string, limit, _ int) ([]retrieval.Hit, error) {
	searcher.calls++
	if owner != "owner" || baseID != "base" || generation != 0 || query != "question" || limit != 5 {
		return nil, fmt.Errorf("unexpected search arguments: %s %s %d %s %d", owner, baseID, generation, query, limit)
	}
	return searcher.hits, searcher.err
}

func TestAssistantRetrievalUsesVerifiedKnowledgeSearcher(t *testing.T) {
	searcher := &assistantKnowledgeSearcher{hits: []retrieval.Hit{{Text: "grounded answer", Source: workspacedomain.KnowledgeSearchSource{DocumentName: "guide.txt", CategoryName: "manual", RevisionID: "revision"}}}}
	service := &Service{knowledgeSearch: searcher}
	text, grounded, err := service.retrieveAssistantKnowledge(context.Background(), "owner", []string{"base"}, "question")
	if err != nil || !grounded || !strings.Contains(text, "manual/guide.txt") || !strings.Contains(text, "grounded answer") || searcher.calls != 1 {
		t.Fatalf("Assistant retrieval = %q, %v, %v, calls=%d", text, grounded, err, searcher.calls)
	}
	searcher.err = workspacedomain.ErrNotFound
	if _, _, err := service.retrieveAssistantKnowledge(context.Background(), "owner", []string{"base"}, "question"); !errors.Is(err, workspacedomain.ErrNotFound) {
		t.Fatalf("inaccessible Knowledge Base silently fell back: %v", err)
	}
	service.knowledgeSearch = nil
	if _, _, err := service.retrieveAssistantKnowledge(context.Background(), "owner", []string{"base"}, "question"); err == nil {
		t.Fatal("missing AnythingLLM silently fell back to PostgreSQL")
	}
}

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
