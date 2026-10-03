package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
func (r *lifecycleRepository) RecordPublicationValidation(_ context.Context, _, _ string, version int64, checkedAt time.Time) (domain.SmartAssistant, error) {
	r.assistant.LastValidatedAt = &checkedAt
	r.assistant.ValidatedVersion = version
	return r.assistant, nil
}
func (r *lifecycleRepository) PublicationStats(context.Context, string, string, time.Time) (domain.PublicationStats, error) {
	return domain.PublicationStats{ExternalConversations: 2, FreeTextCalls: 3, CreditConsumedHundredths: 125}, nil
}

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
func TestSharedAssistantRequiresValidationForCurrentRevision(t *testing.T) {
	now := time.Now().UTC()
	repository := &lifecycleRepository{assistant: domain.SmartAssistant{
		ID: "assistant-1", Name: "助手", Icon: "sparkles", State: domain.StateEnabled, Version: 4,
		Share: domain.ShareConfiguration{Enabled: true, TokenHash: "stored-by-repository", AllowedOrigins: []string{"https://support.example.test"}, DailyCallLimit: 100, DataProcessingAcknowledged: true},
	}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveSharedAssistant(context.Background(), "share-token-long-enough-to-resolve-123"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ResolveSharedAssistant() stale validation error = %v, want ErrNotFound", err)
	}
	repository.assistant.LastValidatedAt = &now
	repository.assistant.ValidatedVersion = 4
	if _, err := service.ResolveSharedAssistant(context.Background(), "share-token-long-enough-to-resolve-123"); err != nil {
		t.Fatalf("ResolveSharedAssistant() current validation error = %v", err)
	}
}

func TestPublicationStatsUsesBoundedWindow(t *testing.T) {
	repository := &lifecycleRepository{assistant: domain.SmartAssistant{ID: "assistant-1"}}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := service.PublicationStats(context.Background(), "owner-1", "assistant-1", 30)
	if err != nil || stats.WindowDays != 30 || stats.CreditConsumedHundredths != 125 {
		t.Fatalf("PublicationStats() = %#v, %v", stats, err)
	}
}

func TestAssistantSharingIgnoresLegacyQuestionRestrictions(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	legacy := domain.SmartAssistant{
		ID: "assistant", Name: "助手", OwnerID: "owner", State: domain.StateEnabled,
		Version: 4, ValidatedVersion: 4, LastValidatedAt: &now,
		Share: domain.ShareConfiguration{Enabled: true, TokenHash: "stored-hash", TokenRevision: 2, AllowedOrigins: []string{"https://support.example.test"}, FreeTextEnabled: false, DailyCallLimit: 2, DataProcessingAcknowledged: true},
	}
	repository := &lifecycleRepository{assistant: legacy}
	service, err := application.New(repository)
	if err != nil {
		t.Fatal(err)
	}
	assertFree := func(assistant domain.SmartAssistant, err error) {
		t.Helper()
		if err != nil || !assistant.Share.FreeTextEnabled || assistant.Share.DailyCallLimit != 0 || assistant.Share.EmbedType != "fullscreen" {
			t.Fatalf("sharing=%#v err=%v", assistant.Share, err)
		}
	}
	assertFree(service.GetAssistant(ctx, "owner", "assistant"))
	assertFree(service.ResolveSharedAssistant(ctx, "share-token-long-enough-to-resolve-123"))
	list, err := service.ListAssistants(ctx, "owner")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%#v err=%v", list, err)
	}
	assertFree(list[0], nil)
	if repository.assistant.Share.TokenRevision != 2 || repository.assistant.Version != 4 || repository.assistant.Share.FreeTextEnabled {
		t.Fatal("reading legacy settings mutated saved configuration or revoked the share")
	}
	legacy.State = domain.StateDraft
	assertFree(service.UpdateAssistant(ctx, "owner", "assistant", legacy, 4))
	if !repository.assistant.Share.FreeTextEnabled || repository.assistant.Share.DailyCallLimit != 0 {
		t.Fatal("update persisted legacy restrictions")
	}
	assertFree(service.CreateAssistant(ctx, "owner", legacy))
	if !repository.assistant.Share.FreeTextEnabled || repository.assistant.Share.DailyCallLimit != 0 {
		t.Fatal("creation persisted legacy restrictions")
	}
}
