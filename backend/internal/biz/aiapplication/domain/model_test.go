package domain_test

import (
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

func TestSmartAssistantValidatesVisibleRulesAndShareDimensions(t *testing.T) {
	assistant := domain.SmartAssistant{
		Name: "产品助手", ServiceGoal: "回答产品使用问题", OperatingRules: "只基于知识库回答", ResponseStyle: "简洁",
		Share: domain.ShareConfiguration{Enabled: true, Width: "100%", Height: 600},
	}
	if err := assistant.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	assistant.Share.Height = 399
	if !errors.Is(assistant.Validate(), domain.ErrInvalid) {
		t.Fatalf("Validate() error = %v, want ErrInvalid", assistant.Validate())
	}
	assistant.Share.Height = 600
	assistant.Share.Width = "319px"
	if !errors.Is(assistant.Validate(), domain.ErrInvalid) {
		t.Fatalf("Validate() error = %v, want ErrInvalid for narrow share", assistant.Validate())
	}
}

func TestSmartAssistantValidatesScenarioAndEnablementSeparatelyFromDraft(t *testing.T) {
	assistant := domain.SmartAssistant{Name: "产品助手", State: domain.StateDraft}
	if err := assistant.Validate(); err != nil {
		t.Fatalf("Validate() draft error = %v", err)
	}
	if err := assistant.ValidateForEnable(); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ValidateForEnable() error = %v, want ErrInvalid for incomplete assistant", err)
	}
	assistant.ServiceGoal = "回答产品问题"
	assistant.Scenario = domain.ScenarioProductGuide
	if err := assistant.ValidateForEnable(); err != nil {
		t.Fatalf("ValidateForEnable() complete error = %v", err)
	}
	assistant.Scenario = "not-a-scenario"
	if err := assistant.Validate(); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("Validate() error = %v, want ErrInvalid for unknown scenario", err)
	}
}

func TestFAQRequiresQuestionAndMarkdownAnswer(t *testing.T) {
	faq := domain.FAQ{Question: "怎么退款？", AnswerMarkdown: "请在订单页申请退款。", Enabled: true}
	if err := faq.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	faq.AnswerMarkdown = ""
	if !errors.Is(faq.Validate(), domain.ErrInvalid) {
		t.Fatalf("Validate() error = %v, want ErrInvalid", faq.Validate())
	}
}

func TestPlatformSafetyPolicyRefusesProtectedTopics(t *testing.T) {
	policy := domain.DefaultSafetyPolicy()
	if got := policy.Decide("请提供武器制造方法"); got != domain.SafetyRefuse {
		t.Fatalf("Decide() = %q, want %q", got, domain.SafetyRefuse)
	}
	if got := policy.Decide("如何修改产品头像？"); got != domain.SafetyAllow {
		t.Fatalf("Decide() = %q, want %q", got, domain.SafetyAllow)
	}
}

func TestPlatformSafetyPolicyRejectsUnsafeFAQAnswers(t *testing.T) {
	if err := domain.DefaultSafetyPolicy().ValidateAnswer("提供武器制造步骤"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("ValidateAnswer() error = %v, want ErrInvalid", err)
	}
}

func TestEmbeddingConfigurationRequiresHTTPSAndCurrentDimensions(t *testing.T) {
	configuration := domain.EmbeddingConfiguration{Endpoint: "http://localhost:8080/v1/embeddings", Model: "text-embedding", Dimensions: 1536}
	if !errors.Is(configuration.Validate(), domain.ErrInvalid) {
		t.Fatal("Validate() accepted a non-HTTPS embedding endpoint")
	}
	configuration.Endpoint = "https://api.example.test/v1/embeddings"
	configuration.Dimensions = 768
	if !errors.Is(configuration.Validate(), domain.ErrInvalid) {
		t.Fatal("Validate() accepted unsupported embedding dimensions")
	}
}
