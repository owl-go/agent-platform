package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	aiapp "agent-platform/backend/internal/biz/aiapplication/application"
	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/knowledgebase/retrieval"
)

func TestAssistantTurnFailureCodeKeepsOnlySafeModelCategory(t *testing.T) {
	providerFailure := &aiapp.ChatError{Code: aiapp.ChatFailureAuthentication, Message: "upstream rejected credential"}
	if code := assistantTurnFailureCode(providerFailure); code != aiapp.ChatFailureAuthentication {
		t.Fatalf("provider failure code = %q", code)
	}
	if code := assistantTurnFailureCode(errors.New("private internal detail")); code != "assistant_failed" {
		t.Fatalf("internal failure code = %q", code)
	}
}

type assistantModelRepository struct {
	workspaceapplication.Repository
	connections []workspacedomain.ModelProviderConnection
}

func (repository *assistantModelRepository) ListModelProviderConnections(context.Context) ([]workspacedomain.ModelProviderConnection, error) {
	return repository.connections, nil
}

type currentAssistantRepository struct {
	aiapp.Repository
	assistant aiappdomain.SmartAssistant
	faqs      []aiappdomain.FAQ
}

func (repository *currentAssistantRepository) GetAssistant(_ context.Context, owner, id string) (aiappdomain.SmartAssistant, error) {
	if owner != repository.assistant.OwnerID || id != repository.assistant.ID {
		return aiappdomain.SmartAssistant{}, aiappdomain.ErrNotFound
	}
	return repository.assistant, nil
}

func (repository *currentAssistantRepository) ListFAQs(_ context.Context, owner, assistantID string) ([]aiappdomain.FAQ, error) {
	if owner != repository.assistant.OwnerID || assistantID != repository.assistant.ID {
		return nil, aiappdomain.ErrNotFound
	}
	return append([]aiappdomain.FAQ(nil), repository.faqs...), nil
}

type assistantKnowledgeSearcher struct {
	hits  []retrieval.Hit
	err   error
	calls int
	query string
}

func (searcher *assistantKnowledgeSearcher) Search(_ context.Context, owner, baseID string, generation int64, query string, limit, _ int) ([]retrieval.Hit, error) {
	searcher.calls++
	expectedQuery := searcher.query
	if expectedQuery == "" {
		expectedQuery = "question"
	}
	if owner != "owner" || baseID != "base" || generation != 0 || query != expectedQuery || limit != 5 {
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
		t.Fatal("missing retrieval provider silently continued")
	}
}

func TestAssistantHistoryUsesOnlyLatestTenCompletedTurns(t *testing.T) {
	turns := make([]aiappdomain.AssistantTurn, 0, 14)
	for index := 1; index <= 12; index++ {
		turns = append(turns, aiappdomain.AssistantTurn{TurnNumber: index, Question: fmt.Sprintf("Q%d", index), Answer: fmt.Sprintf("A%d", index), State: "completed"})
	}
	turns = append(turns, aiappdomain.AssistantTurn{TurnNumber: 13, Question: "unfinished", State: "cancelled"})
	var service Service
	messages, usage, err := service.assistantHistory(context.Background(), "owner", aiappdomain.AssistantConversation{}, aiappdomain.AssistantModel{}, "turn", turns)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 20 || messages[0].Content != "Q3" || messages[19].Content != "A12" || usage.InputTokens != 0 {
		t.Fatalf("bounded history = %+v, usage = %+v", messages, usage)
	}
}

func TestAssistantModelSelectionRequiresAvailableOpenAIResponsesModelAndKey(t *testing.T) {
	for _, test := range []struct {
		name      string
		protocols []string
		available bool
		hasKey    bool
		valid     bool
	}{
		{name: "OpenAI Responses", protocols: []string{"openai_responses", "openai_chat"}, available: true, hasKey: true, valid: true},
		{name: "Chat only", protocols: []string{"openai_chat"}, available: true, hasKey: true},
		{name: "Unavailable model", protocols: []string{"openai_responses"}, hasKey: true},
		{name: "Missing key", protocols: []string{"openai_responses"}, available: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			connections := []workspacedomain.ModelProviderConnection{{ID: "connection-1", CredentialOwnerID: "admin", ProviderType: "openai", Endpoint: "https://example.test/v1", Protocols: test.protocols, HasAPIKey: test.hasKey, Version: 3, Models: []workspacedomain.ProviderModel{{ID: "model-1", ModelID: "chat-model", Available: test.available}}}}
			model, err := selectAssistantModel(connections, "model-1")
			if test.valid {
				if err != nil || model.ProviderModelID != "model-1" || model.Protocol != "openai_responses" || model.ConnectionVersion != 3 {
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

func TestAssistantTurnLoadsCurrentAssistantConfiguration(t *testing.T) {
	repository := &currentAssistantRepository{assistant: aiappdomain.SmartAssistant{
		ID: "assistant-1", OwnerID: "owner", Name: "current name", Prompt: "current prompt",
		PreprocessPrompt: "current preprocess", ResponseStyle: "current style",
		KnowledgeBaseIDs: []string{"current-knowledge"}, ProviderModelID: "current-model",
	}}
	application, err := aiapp.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{aiapplications: application}
	conversation := aiappdomain.AssistantConversation{
		AssistantID: "assistant-1",
		AssistantSnapshot: aiappdomain.SmartAssistant{
			ID: "assistant-1", OwnerID: "owner", Name: "stale name", Prompt: "stale prompt",
			PreprocessPrompt: "stale preprocess", ResponseStyle: "stale style",
			KnowledgeBaseIDs: []string{"stale-knowledge"}, ProviderModelID: "stale-model",
		},
	}

	assistant, err := service.loadAssistantTurnConfiguration(context.Background(), "owner", conversation)
	if err != nil {
		t.Fatal(err)
	}
	if assistant.Name != "current name" || assistant.Prompt != "current prompt" || assistant.PreprocessPrompt != "current preprocess" || assistant.ResponseStyle != "current style" || assistant.ProviderModelID != "current-model" || len(assistant.KnowledgeBaseIDs) != 1 || assistant.KnowledgeBaseIDs[0] != "current-knowledge" {
		t.Fatalf("Assistant turn configuration = %+v, want current Assistant configuration", assistant)
	}
}

func TestAssistantScopeInquiryMatchesConfiguredCapabilityFAQ(t *testing.T) {
	faqs := []aiappdomain.FAQ{
		{ID: "model", Question: "运行引擎是什么？", AnswerMarkdown: "internal model details", Enabled: true},
		{ID: "capabilities", Question: "这个agent可以做什么", AnswerMarkdown: "可以创建会话，也可以创建工作流", Enabled: true},
	}
	for _, question := range []string{"你可以回答什么问题", "处理什么业务范围", "能处理哪些业务？"} {
		faq, ok := matchAssistantScopeFAQ(faqs, question)
		if !ok || faq.ID != "capabilities" {
			t.Fatalf("scope inquiry %q matched FAQ %+v, ok=%v", question, faq, ok)
		}
	}
	if faq, ok := matchAssistantScopeFAQ(faqs, "怎么停止了"); ok {
		t.Fatalf("unrelated question matched FAQ %+v", faq)
	}
}

func TestAssistantScopeFallbackDoesNotExposePlaceholderConfiguration(t *testing.T) {
	assistant := aiappdomain.SmartAssistant{Prompt: "回答范围：\n- 【业务范围一】\n- 【业务范围二】"}
	if answer := assistantScopeFallback(assistant); !strings.Contains(answer, "尚未配置具体业务范围") {
		t.Fatalf("placeholder scope answer = %q", answer)
	}
	assistant.Description = "处理账户开通和售后政策问题"
	if answer := assistantScopeFallback(assistant); !strings.Contains(answer, assistant.Description) {
		t.Fatalf("configured scope answer = %q", answer)
	}
}

func TestAssistantScopeInquiryReturnsCurrentCapabilityFAQWithoutModel(t *testing.T) {
	repository := &currentAssistantRepository{
		assistant: aiappdomain.SmartAssistant{ID: "assistant-1", OwnerID: "owner", Prompt: "strict scope prompt"},
		faqs: []aiappdomain.FAQ{{
			ID: "capabilities", AssistantID: "assistant-1", Question: "这个agent可以做什么",
			AnswerMarkdown: "可以创建会话，也可以创建工作流", Enabled: true,
		}},
	}
	application, err := aiapp.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{aiapplications: application}
	answer, err := service.answerAssistantTurn(
		context.Background(), "owner",
		aiappdomain.AssistantConversation{ID: "conversation-1", AssistantID: "assistant-1"},
		aiappdomain.AssistantTurn{ID: "turn-1", Question: "你可以回答什么问题"},
		"", "authenticated", func(string) error { return nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if answer.text != "可以创建会话，也可以创建工作流" || answer.source != "faq" || answer.faqID != "capabilities" || answer.inputTokens != 0 || answer.outputTokens != 0 {
		t.Fatalf("scope answer = %+v", answer)
	}
}

func TestAssistantTypedFAQReturnsStoredAnswerBeforeScopeClassification(t *testing.T) {
	repository := &currentAssistantRepository{
		assistant: aiappdomain.SmartAssistant{ID: "assistant-1", OwnerID: "owner", ProviderModelID: "unavailable-model", PreprocessPrompt: "询问运行框架或技术实现必须判定为 out_of_scope"},
		faqs:      []aiappdomain.FAQ{{ID: "engine", Question: "运行引擎是什么？", AnswerMarkdown: "这个项目使用已配置的运行引擎。", Enabled: true}},
	}
	application, err := aiapp.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	// Even with an unavailable model, a stored FAQ must still be answerable. This
	// reproduces typed questions being routed to model/scope classification.
	service := &Service{aiapplications: application, workspace: mustAssistantWorkspace(t)}
	answer, err := service.answerAssistantTurn(context.Background(), "owner", aiappdomain.AssistantConversation{ID: "conversation", AssistantID: "assistant-1"}, aiappdomain.AssistantTurn{ID: "turn", Question: "运行引擎是什么？"}, "", "authenticated", func(string) error { return nil })
	if err != nil || answer.text != repository.faqs[0].AnswerMarkdown || answer.source != "faq" || answer.faqID != "engine" || answer.inputTokens != 0 || answer.outputTokens != 0 {
		t.Fatalf("typed FAQ did not return its stored answer: answer=%+v, err=%v", answer, err)
	}
}

func mustAssistantWorkspace(t *testing.T) *workspaceapplication.Service {
	t.Helper()
	application, err := workspaceapplication.New(&assistantModelRepository{})
	if err != nil {
		t.Fatal(err)
	}
	return application
}

func TestAssistantTurnResolvesCurrentProviderConfiguration(t *testing.T) {
	repository := &assistantModelRepository{connections: []workspacedomain.ModelProviderConnection{{
		ID: "connection-current", CredentialOwnerID: "admin", ProviderType: "openai",
		Endpoint: "https://current.example.test/v1", Protocols: []string{"openai_responses"}, HasAPIKey: true, Version: 16,
		Models: []workspacedomain.ProviderModel{{ID: "model-1", ModelID: "gpt-current", Available: true}},
	}}}
	application, err := workspaceapplication.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{workspace: application}
	assistant := aiappdomain.SmartAssistant{ProviderModelID: "model-1"}

	model, err := service.resolveAssistantTurnModel(context.Background(), "owner", assistant)
	if err != nil {
		t.Fatal(err)
	}
	if model.ConnectionID != "connection-current" || model.CredentialOwnerID != "admin" || model.Endpoint != "https://current.example.test/v1" || model.ModelID != "gpt-current" || model.ConnectionVersion != 16 {
		t.Fatalf("Assistant turn model = %+v, want current Provider configuration", model)
	}
}
