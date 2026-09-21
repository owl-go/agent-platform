package application_test

import (
	"context"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/application"
	"agent-platform/backend/internal/biz/aiapplication/domain"
)

type answerRepository struct{ faq domain.FAQ }

func (repository answerRepository) ListFAQs(context.Context, string, string) ([]domain.FAQ, error) {
	return []domain.FAQ{repository.faq}, nil
}
func (answerRepository) ListAssistants(context.Context, string) ([]domain.SmartAssistant, error) {
	return nil, nil
}
func (answerRepository) GetAssistant(context.Context, string, string) (domain.SmartAssistant, error) {
	return domain.SmartAssistant{}, nil
}
func (answerRepository) GetAssistantByShareTokenHash(context.Context, string) (domain.SmartAssistant, error) {
	return domain.SmartAssistant{}, nil
}
func (answerRepository) BindAssistantSession(context.Context, string, string, string, []byte) error {
	return nil
}
func (answerRepository) CreateAssistant(context.Context, string, domain.SmartAssistant) (domain.SmartAssistant, error) {
	return domain.SmartAssistant{}, nil
}
func (answerRepository) UpdateAssistant(context.Context, string, string, domain.SmartAssistant, int64) (domain.SmartAssistant, error) {
	return domain.SmartAssistant{}, nil
}
func (answerRepository) DeleteAssistant(context.Context, string, string) error { return nil }
func (answerRepository) ListDigitalHumans(context.Context, string) ([]domain.DigitalHuman, error) {
	return nil, nil
}
func (answerRepository) GetDigitalHuman(context.Context, string, string) (domain.DigitalHuman, error) {
	return domain.DigitalHuman{}, nil
}
func (answerRepository) CreateDigitalHuman(context.Context, string, domain.DigitalHuman) (domain.DigitalHuman, error) {
	return domain.DigitalHuman{}, nil
}
func (answerRepository) UpdateDigitalHuman(context.Context, string, string, domain.DigitalHuman, int64) (domain.DigitalHuman, error) {
	return domain.DigitalHuman{}, nil
}
func (answerRepository) DeleteDigitalHuman(context.Context, string, string) error { return nil }
func (answerRepository) CreateFAQ(context.Context, string, string, domain.FAQ) (domain.FAQ, error) {
	return domain.FAQ{}, nil
}
func (answerRepository) UpdateFAQ(context.Context, string, string, string, domain.FAQ, int64) (domain.FAQ, error) {
	return domain.FAQ{}, nil
}
func (answerRepository) DeleteFAQ(context.Context, string, string, string) error { return nil }

func TestFAQAnswerReturnsMarkdownWithoutModel(t *testing.T) {
	service, err := application.New(answerRepository{faq: domain.FAQ{Question: "如何退款？", AnswerMarkdown: "在订单页申请。", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	faq, matched, err := service.FAQAnswer(context.Background(), "owner", "assistant", "  如何 退款？ ")
	if err != nil || !matched || faq.AnswerMarkdown != "在订单页申请。" {
		t.Fatalf("FAQAnswer() = %+v, %v, %v", faq, matched, err)
	}
}

func TestFAQAnswerSkipsProtectedQuestion(t *testing.T) {
	service, err := application.New(answerRepository{faq: domain.FAQ{Question: "武器", AnswerMarkdown: "unsafe", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, matched, err := service.FAQAnswer(context.Background(), "owner", "assistant", "请介绍武器")
	if err != nil || matched {
		t.Fatalf("FAQAnswer() matched = %v, err = %v, want refusal before FAQ", matched, err)
	}
}

func TestFAQAnswerDoesNotMatchBlankQuestion(t *testing.T) {
	service, err := application.New(answerRepository{faq: domain.FAQ{Question: "常见问题", AnswerMarkdown: "答案", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, matched, err := service.FAQAnswer(context.Background(), "owner", "assistant", "   ")
	if err != nil || matched {
		t.Fatalf("FAQAnswer() matched = %v, err = %v, want no match", matched, err)
	}
}
