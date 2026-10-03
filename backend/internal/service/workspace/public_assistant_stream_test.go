package workspace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	aiapp "agent-platform/backend/internal/biz/aiapplication/application"
	domain "agent-platform/backend/internal/biz/aiapplication/domain"
	creditsapp "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspaceapp "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
)

const publicTestToken = "public-test-token-with-more-than-32-characters"

type publicStreamRepository struct {
	currentAssistantRepository
	aiapp.AssistantConversationRepository
	conversation              domain.AssistantConversation
	turn                      domain.AssistantTurn
	history                   []domain.AssistantTurn
	rateAllowed, dailyAllowed bool
	dailyCalls                int
}

func (r *publicStreamRepository) GetAssistantByShareTokenHash(_ context.Context, hash string) (domain.SmartAssistant, error) {
	if hash != publicHash(publicTestToken) {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	return r.assistant, nil
}
func (r *publicStreamRepository) CreateAssistantConversation(_ context.Context, value domain.AssistantConversation) (domain.AssistantConversation, error) {
	value.ID = "visitor-conversation"
	r.conversation = value
	return value, nil
}
func (r *publicStreamRepository) GetPublicAssistantConversation(_ context.Context, owner, assistantID, id, visitor string, revision int64) (domain.AssistantConversation, error) {
	c := r.conversation
	if owner != c.OwnerID || assistantID != c.AssistantID || id != c.ID || visitor != c.VisitorHash || revision != c.ShareTokenRevision {
		return domain.AssistantConversation{}, domain.ErrNotFound
	}
	return c, nil
}
func (r *publicStreamRepository) BeginAssistantTurn(_ context.Context, owner, id, question string) (domain.AssistantTurn, error) {
	if owner != r.assistant.OwnerID || id != r.conversation.ID {
		return domain.AssistantTurn{}, domain.ErrNotFound
	}
	if r.turn.ID != "" {
		r.history = append(r.history, r.turn)
	}
	r.turn = domain.AssistantTurn{ID: "visitor-turn", ConversationID: id, Question: question, State: "generating", TurnNumber: len(r.history) + 1}
	return r.turn, nil
}
func (r *publicStreamRepository) ListAssistantTurns(context.Context, string, string) ([]domain.AssistantTurn, error) {
	values := append([]domain.AssistantTurn(nil), r.history...)
	if r.turn.ID != "" {
		values = append(values, r.turn)
	}
	return values, nil
}
func (r *publicStreamRepository) SaveAssistantTurnProgress(_ context.Context, _, _, _, answer string) error {
	r.turn.Answer = answer
	return nil
}
func (r *publicStreamRepository) FinishAssistantTurn(_ context.Context, _, _, _, state, source, faqID, answer, failure string, input, output int64) (domain.AssistantTurn, error) {
	r.turn.State = state
	r.turn.Source = source
	r.turn.FAQID = faqID
	r.turn.Answer = answer
	r.turn.Error = failure
	r.turn.InputTokens = input
	r.turn.OutputTokens = output
	return r.turn, nil
}
func (r *publicStreamRepository) ConsumeExternalRate(context.Context, string, string, time.Time, int) (bool, error) {
	return r.rateAllowed, nil
}
func (r *publicStreamRepository) ConsumeShareCall(context.Context, string, time.Time, int) (bool, error) {
	r.dailyCalls++
	return r.dailyAllowed, nil
}
func (r *publicStreamRepository) RecordSafetyAudit(context.Context, string, string, string, domain.SafetyDecision, string) error {
	return nil
}

type publicStreamModel struct {
	probe  func()
	cancel context.CancelFunc
	fail   bool
	calls  int
}

func (m *publicStreamModel) Generate(ctx context.Context, request aiapp.ChatRequest, delta func(string) error) (aiapp.ChatResult, error) {
	m.calls++
	if !request.Stream {
		return aiapp.ChatResult{Text: `{"decision":"continue","faq_id":"","question":"分析客户"}`, UsageKnown: true}, nil
	}
	if err := delta("**继续跟进**"); err != nil {
		return aiapp.ChatResult{}, err
	}
	if m.probe != nil {
		m.probe()
	}
	if m.cancel != nil {
		m.cancel()
		return aiapp.ChatResult{Text: "**继续跟进**", UsageKnown: true}, ctx.Err()
	}
	if m.fail {
		return aiapp.ChatResult{Text: "**继续跟进**", UsageKnown: true}, &aiapp.ChatError{Code: aiapp.ChatFailureUnavailable, Message: "test upstream failure"}
	}
	if err := delta("，维护关系。"); err != nil {
		return aiapp.ChatResult{}, err
	}
	return aiapp.ChatResult{Text: "**继续跟进**，维护关系。", UsageKnown: true}, nil
}

func publicStreamService(t *testing.T) (*Service, *publicStreamRepository, *publicStreamModel) {
	t.Helper()
	now := time.Now()
	r := &publicStreamRepository{rateAllowed: true, dailyAllowed: true, currentAssistantRepository: currentAssistantRepository{assistant: domain.SmartAssistant{ID: "assistant", OwnerID: "owner", Name: "项目分析助手", Icon: "sparkles", Introduction: "欢迎提问", ProviderModelID: "model", State: domain.StateEnabled, Version: 4, ValidatedVersion: 4, LastValidatedAt: &now, Share: domain.ShareConfiguration{Enabled: true, TokenRevision: 2, AllowedOrigins: []string{"https://customer.example.test"}, Width: "640px", Height: 600, FreeTextEnabled: true, DailyCallLimit: 100, DataProcessingAcknowledged: true}}, faqs: []domain.FAQ{{ID: "faq", Question: "运行引擎是什么？", AnswerMarkdown: "固定答案", Enabled: true}}}}
	application, err := aiapp.New(r)
	if err != nil {
		t.Fatal(err)
	}
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	credential, err := box.Encrypt([]byte("test-secret"), "model-provider:admin")
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := workspaceapp.New(&planModelRepository{credential: credential, connection: workspacedomain.ModelProviderConnection{ID: "connection", CredentialOwnerID: "admin", ProviderType: "openai", Version: 1, HasAPIKey: true, Protocols: []string{"openai_responses"}, Models: []workspacedomain.ProviderModel{{ID: "model", ModelID: "test-model", Available: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	credits, err := creditsapp.New(&planCreditRepository{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	model := &publicStreamModel{}
	return &Service{aiapplications: application, workspace: workspace, credits: credits, box: box, assistantChatModel: model}, r, model
}
func publicStreamRequest(question, conversation, faq string) *http.Request {
	body, _ := json.Marshal(map[string]string{"question": question, "conversation_id": conversation, "faq_id": faq})
	request := httptest.NewRequest(http.MethodPost, "http://platform.example.test/api/v1/public/assistants/"+publicTestToken+"/turns", strings.NewReader(string(body)))
	request.AddCookie(&http.Cookie{Name: visitorCookieName, Value: "test-visitor-cookie"})
	return request
}

func TestPublicAssistantStreamsBeforeCompletion(t *testing.T) {
	service, repository, model := publicStreamService(t)
	writer := httptest.NewRecorder()
	model.probe = func() {
		if !writer.Flushed || !strings.Contains(writer.Body.String(), "event: delta") || strings.Contains(writer.Body.String(), "event: done") {
			t.Fatal("first answer chunk was buffered until completion")
		}
	}
	request := publicStreamRequest("分析客户", "", "")
	// The iframe's fetch originates from the platform, while frame-ancestors limits its parent.
	request.Header.Set("Origin", "http://platform.example.test")
	service.publicAssistantHandler(writer, request)
	if writer.Code != http.StatusOK || !strings.Contains(writer.Body.String(), "event: done") {
		t.Fatalf("stream=%d %s", writer.Code, writer.Body.String())
	}
	if repository.turn.State != "completed" || repository.turn.Answer != "**继续跟进**，维护关系。" || repository.dailyCalls != 0 {
		t.Fatalf("turn=%#v calls=%d", repository.turn, repository.dailyCalls)
	}
	for _, private := range []string{"test-secret", "model_id", "connection_id", "input_tokens", "owner_id", "prompt"} {
		if strings.Contains(writer.Body.String(), private) {
			t.Fatalf("public stream exposed %q", private)
		}
	}
}

func TestPublicAssistantStreamControls(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*publicStreamRepository, *http.Request)
		status int
	}{
		{"revoked share", func(r *publicStreamRepository, _ *http.Request) { r.assistant.Share.Enabled = false }, 404},
		{"stale publication", func(r *publicStreamRepository, _ *http.Request) { r.assistant.ValidatedVersion = 3 }, 404},
		{"unlisted origin", func(_ *publicStreamRepository, q *http.Request) {
			q.Header.Set("Origin", "https://unrelated.example.test")
		}, 403},
		{"visitor rate", func(r *publicStreamRepository, _ *http.Request) { r.rateAllowed = false }, 429},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, r, model := publicStreamService(t)
			request := publicStreamRequest("分析客户", "", "")
			test.change(r, request)
			writer := httptest.NewRecorder()
			service.publicAssistantHandler(writer, request)
			if writer.Code != test.status || model.calls != 0 || r.turn.ID != "" {
				t.Fatalf("status=%d calls=%d turn=%q", writer.Code, model.calls, r.turn.ID)
			}
		})
	}
}

func TestPublicAssistantFAQWorksWithoutModelCalls(t *testing.T) {
	service, r, model := publicStreamService(t)
	r.assistant.Share.FreeTextEnabled = false
	writer := httptest.NewRecorder()
	service.publicAssistantHandler(writer, publicStreamRequest("运行引擎是什么？", "", "faq"))
	if writer.Code != 200 || r.turn.Answer != "固定答案" || model.calls != 0 || r.dailyCalls != 0 {
		t.Fatalf("FAQ status=%d answer=%q model=%d daily=%d", writer.Code, r.turn.Answer, model.calls, r.dailyCalls)
	}
}

func TestPublicAssistantAlwaysAllowsFreeQuestionsWithLegacySharingSettings(t *testing.T) {
	for _, question := range []string{"你好呀", "分析客户"} {
		t.Run(question, func(t *testing.T) {
			service, r, model := publicStreamService(t)
			r.assistant.Share.FreeTextEnabled = false
			r.assistant.Share.DailyCallLimit = 2
			r.dailyAllowed = false // A previously exhausted daily counter must not be consulted.
			for i := 0; i < 3; i++ {
				conversation := ""
				if i > 0 {
					conversation = r.conversation.ID
				}
				writer := httptest.NewRecorder()
				service.publicAssistantHandler(writer, publicStreamRequest(question, conversation, ""))
				if writer.Code != 200 || r.turn.State != "completed" || r.turn.TurnNumber != i+1 || r.dailyCalls != 0 || !strings.Contains(writer.Body.String(), "event: done") {
					t.Fatalf("free question %d: status=%d turn=%#v daily=%d", i, writer.Code, r.turn, r.dailyCalls)
				}
				if question == "你好呀" && (r.turn.Answer != "欢迎提问" || r.turn.Source != "configuration" || model.calls != 0) {
					t.Fatal("greeting did not use the configured welcome without model calls")
				}
			}
		})
	}
}

func TestPublicAssistantLegacyMetadataAndAnswerAllowFreeQuestions(t *testing.T) {
	service, r, model := publicStreamService(t)
	r.assistant.Share.FreeTextEnabled = false
	r.assistant.Share.DailyCallLimit = 2
	r.dailyAllowed = false
	metadata := httptest.NewRecorder()
	service.publicAssistantHandler(metadata, httptest.NewRequest(http.MethodGet, "http://platform.example.test/api/v1/public/assistants/"+publicTestToken, nil))
	var profile struct {
		FreeTextEnabled bool `json:"free_text_enabled"`
	}
	if err := json.Unmarshal(metadata.Body.Bytes(), &profile); err != nil || metadata.Code != 200 || !profile.FreeTextEnabled {
		t.Fatalf("metadata=%d %s err=%v", metadata.Code, metadata.Body.String(), err)
	}
	request := publicStreamRequest("运行引擎是什么？", "", "")
	request.URL.Path = "/api/v1/public/assistants/" + publicTestToken + "/answer"
	writer := httptest.NewRecorder()
	service.publicAssistantHandler(writer, request)
	if writer.Code != 200 || !strings.Contains(writer.Body.String(), "faq") || model.calls != 0 || r.dailyCalls != 0 {
		t.Fatalf("legacy answer=%d %s calls=%d daily=%d", writer.Code, writer.Body.String(), model.calls, r.dailyCalls)
	}
}

func TestPublicAssistantCannotResumeAnotherVisitorOrTokenRevision(t *testing.T) {
	for _, revision := range []int64{1, 2} {
		t.Run(map[int64]string{1: "old token revision", 2: "another visitor"}[revision], func(t *testing.T) {
			service, r, model := publicStreamService(t)
			visitor := publicHash("test-visitor-cookie")
			if revision == 2 {
				visitor = "another-visitor"
			}
			r.conversation = domain.AssistantConversation{ID: "other", OwnerID: "owner", AssistantID: "assistant", VisitorHash: visitor, ShareTokenRevision: revision}
			writer := httptest.NewRecorder()
			service.publicAssistantHandler(writer, publicStreamRequest("分析客户", "other", ""))
			if writer.Code != 404 || model.calls != 0 || r.turn.ID != "" {
				t.Fatalf("resumed another visitor/revision: status=%d", writer.Code)
			}
		})
	}
}

func TestPublicAssistantContinuesVisitorConversation(t *testing.T) {
	service, r, _ := publicStreamService(t)
	service.publicAssistantHandler(httptest.NewRecorder(), publicStreamRequest("分析客户", "", ""))
	first := r.turn
	writer := httptest.NewRecorder()
	service.publicAssistantHandler(writer, publicStreamRequest("继续分析", r.conversation.ID, ""))
	if writer.Code != 200 || r.turn.State != "completed" || r.turn.TurnNumber != 2 || len(r.history) != 1 || r.history[0].Answer != first.Answer || r.dailyCalls != 0 {
		t.Fatalf("continuation status=%d turn=%#v history=%#v daily=%d", writer.Code, r.turn, r.history, r.dailyCalls)
	}
}

func TestPublicAssistantRejectsMismatchedFAQID(t *testing.T) {
	service, r, model := publicStreamService(t)
	r.assistant.Share.FreeTextEnabled = false
	writer := httptest.NewRecorder()
	service.publicAssistantHandler(writer, publicStreamRequest("分析客户", "", "faq"))
	if writer.Code != 422 || model.calls != 0 || r.dailyCalls != 0 || r.turn.ID != "" {
		t.Fatalf("forged FAQ status=%d model=%d daily=%d turn=%#v", writer.Code, model.calls, r.dailyCalls, r.turn)
	}
}

func TestPublicAssistantStreamPreservesFailedAndCancelledPartialAnswer(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed", true: "cancelled"}[cancelled], func(t *testing.T) {
			service, r, model := publicStreamService(t)
			request := publicStreamRequest("分析客户", "", "")
			ctx, cancel := context.WithCancel(request.Context())
			defer cancel()
			request = request.WithContext(ctx)
			if cancelled {
				model.cancel = cancel
			} else {
				model.fail = true
			}
			writer := httptest.NewRecorder()
			service.publicAssistantHandler(writer, request)
			want := map[bool]string{false: "failed", true: "cancelled"}[cancelled]
			if r.turn.State != want || r.turn.Answer != "**继续跟进**" || !strings.Contains(writer.Body.String(), "event: done") {
				t.Fatalf("partial answer: %#v stream=%s", r.turn, writer.Body.String())
			}
		})
	}
}

func TestPublicAssistantEmbedLoadsSharedChatEntryWithConfiguredDimensions(t *testing.T) {
	service, r, _ := publicStreamService(t)
	r.assistant.Name = "助手<script>alert(1)</script>"
	writer := httptest.NewRecorder()
	service.publicAssistantEmbed(writer, httptest.NewRequest("GET", "/embed/assistant/"+publicTestToken, nil))
	body := writer.Body.String()
	if writer.Code != 200 || !strings.Contains(body, `src="/assets/assistant-embed.js"`) || !strings.Contains(body, `data-width="640px" data-height="600"`) || strings.Contains(body, `id="form"`) || strings.Contains(body, "助手<script>") {
		t.Fatalf("unexpected embed shell: %s", body)
	}
	if !strings.Contains(writer.Header().Get("Content-Security-Policy"), "frame-ancestors https://customer.example.test") {
		t.Fatal("configured frame restrictions missing")
	}
}

type publicationCreditRepository struct{ creditsapp.Repository }

func (publicationCreditRepository) Balance(context.Context, string, string, time.Time) (creditsdomain.Balance, error) {
	return creditsdomain.Balance{Available: 100}, nil
}

func TestAssistantPublicationDoesNotRequireQuestionRestrictions(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.ShareConfiguration)
		ready  bool
	}{
		{"no daily cap", func(s *domain.ShareConfiguration) { s.DailyCallLimit = 0 }, true},
		{"legacy restrictions", func(s *domain.ShareConfiguration) { s.FreeTextEnabled = false; s.DailyCallLimit = 2 }, true},
		{"missing origins", func(s *domain.ShareConfiguration) { s.AllowedOrigins = nil }, false},
		{"missing acknowledgement", func(s *domain.ShareConfiguration) { s.DataProcessingAcknowledged = false }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, r, _ := publicStreamService(t)
			test.change(&r.assistant.Share)
			credits, err := creditsapp.New(publicationCreditRepository{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			service.credits = credits
			validation := service.assistantPublicationCheck(context.Background(), "owner", r.assistant)
			for _, check := range validation.Checks {
				if check.Code == "share_controls" {
					if check.Ready != test.ready {
						t.Fatalf("share check=%#v", check)
					}
					return
				}
			}
			t.Fatal("publication omitted the share check")
		})
	}
}
