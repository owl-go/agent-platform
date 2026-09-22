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
	human     domain.DigitalHuman
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
func (r *lifecycleRepository) ListDigitalHumans(context.Context, string) ([]domain.DigitalHuman, error) {
	return nil, nil
}
func (r *lifecycleRepository) GetDigitalHuman(context.Context, string, string) (domain.DigitalHuman, error) {
	return r.human, nil
}
func (r *lifecycleRepository) CreateDigitalHuman(_ context.Context, owner string, human domain.DigitalHuman) (domain.DigitalHuman, error) {
	human.ID = "copied-human"
	human.OwnerID = owner
	r.human = human
	return human, nil
}
func (r *lifecycleRepository) UpdateDigitalHuman(_ context.Context, _ string, _ string, human domain.DigitalHuman, _ int64) (domain.DigitalHuman, error) {
	r.human = human
	return human, nil
}
func (r *lifecycleRepository) DeleteDigitalHuman(context.Context, string, string) error { return nil }
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

func TestDisabledDigitalHumanCannotBeBoundToAssistant(t *testing.T) {
	humanID := "human-1"
	repository := &lifecycleRepository{human: domain.DigitalHuman{ID: humanID, Name: "停用数字人", State: domain.StateDisabled}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CreateAssistant(context.Background(), "owner-1", domain.SmartAssistant{Name: "助手", ServiceGoal: "服务", DigitalHumanID: &humanID})
	if !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("CreateAssistant() error = %v, want ErrInvalid", err)
	}
}

func TestDeleteReferencedDigitalHumanReturnsConflict(t *testing.T) {
	repository := &lifecycleRepository{
		assistant: domain.SmartAssistant{ID: "assistant-1", Name: "助手", DigitalHumanID: stringPtr("human-1")},
		human:     domain.DigitalHuman{ID: "human-1", Name: "数字人", State: domain.StateEnabled},
	}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteDigitalHuman(context.Background(), "owner-1", "human-1"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("DeleteDigitalHuman() error = %v, want ErrConflict", err)
	}
}

func TestCopyDigitalHumanCreatesIndependentEnabledIdentity(t *testing.T) {
	repository := &lifecycleRepository{human: domain.DigitalHuman{ID: "human-1", Name: "数字人", State: domain.StateDisabled}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	copy, err := service.CopyDigitalHuman(context.Background(), "owner-1", "human-1")
	if err != nil {
		t.Fatalf("CopyDigitalHuman() error = %v", err)
	}
	if copy.ID == "human-1" || copy.State != domain.StateEnabled {
		t.Fatalf("copy = %#v, want independent enabled identity", copy)
	}
}

func stringPtr(value string) *string { return &value }
