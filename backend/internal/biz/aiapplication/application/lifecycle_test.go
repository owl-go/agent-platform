package application_test

import (
	"context"
	"errors"
	"testing"

	"agent-platform/backend/internal/biz/aiapplication/application"
	"agent-platform/backend/internal/biz/aiapplication/domain"
)

type lifecycleRepository struct {
	assistant domain.SmartAssistant
	faqs      []domain.FAQ
}

func (r *lifecycleRepository) ListAssistants(context.Context, string) ([]domain.SmartAssistant, error) {
	return []domain.SmartAssistant{r.assistant}, nil
}
func (r *lifecycleRepository) GetAssistant(context.Context, string, string) (domain.SmartAssistant, error) {
	return r.assistant, nil
}
func (r *lifecycleRepository) GetAssistantByShareTokenHash(context.Context, string) (domain.SmartAssistant, error) {
	return r.assistant, nil
}
func (r *lifecycleRepository) BindAssistantSession(context.Context, string, string, string, []byte) error {
	return nil
}
func (r *lifecycleRepository) CreateAssistant(_ context.Context, owner string, assistant domain.SmartAssistant) (domain.SmartAssistant, error) {
	assistant.ID = "copied"
	assistant.OwnerID = owner
	r.assistant = assistant
	return assistant, nil
}
func (r *lifecycleRepository) UpdateAssistant(_ context.Context, _ string, _ string, assistant domain.SmartAssistant, _ int64) (domain.SmartAssistant, error) {
	r.assistant = assistant
	return assistant, nil
}
func (r *lifecycleRepository) DeleteAssistant(context.Context, string, string) error { return nil }
func (r *lifecycleRepository) ListFAQs(context.Context, string, string) ([]domain.FAQ, error) {
	return r.faqs, nil
}
func (r *lifecycleRepository) CreateFAQ(_ context.Context, _ string, assistantID string, faq domain.FAQ) (domain.FAQ, error) {
	faq.AssistantID = assistantID
	r.faqs = append(r.faqs, faq)
	return faq, nil
}
func (r *lifecycleRepository) UpdateFAQ(context.Context, string, string, string, domain.FAQ, int64) (domain.FAQ, error) {
	return domain.FAQ{}, nil
}
func (r *lifecycleRepository) DeleteFAQ(context.Context, string, string, string) error { return nil }

func TestCopyAssistantDoesNotReuseShareTokenOrState(t *testing.T) {
	repository := &lifecycleRepository{assistant: domain.SmartAssistant{
		ID: "source", Name: "原助手", ServiceGoal: "回答产品问题", State: domain.StateEnabled,
		Share: domain.ShareConfiguration{Enabled: true, Token: "secret-token", TokenHash: "secret-hash", TokenRevision: 4},
	}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}

	copy, err := service.CopyAssistant(context.Background(), "owner-1", "source")
	if err != nil {
		t.Fatalf("CopyAssistant() error = %v", err)
	}
	if copy.ID == "source" || copy.Share.Token != "" || copy.Share.TokenHash != "" || copy.Share.TokenRevision != 0 {
		t.Fatalf("copy reused identity or token: %#v", copy)
	}
	if copy.State != domain.StateDraft {
		t.Fatalf("copy state = %q, want draft", copy.State)
	}
}

func TestSetAssistantStateRejectsIncompleteEnablement(t *testing.T) {
	repository := &lifecycleRepository{assistant: domain.SmartAssistant{ID: "source", Name: "原助手", State: domain.StateDraft}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.SetAssistantState(context.Background(), "owner-1", "source", domain.StateEnabled, 1)
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("SetAssistantState() error = %v, want ErrInvalid", err)
	}
}
