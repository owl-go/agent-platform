package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

type Repository interface {
	ListAssistants(context.Context, string) ([]domain.SmartAssistant, error)
	GetAssistant(context.Context, string, string) (domain.SmartAssistant, error)
	GetAssistantByShareTokenHash(context.Context, string) (domain.SmartAssistant, error)
	BindAssistantSession(context.Context, string, string, string, []byte) error
	CreateAssistant(context.Context, string, domain.SmartAssistant) (domain.SmartAssistant, error)
	UpdateAssistant(context.Context, string, string, domain.SmartAssistant, int64) (domain.SmartAssistant, error)
	DeleteAssistant(context.Context, string, string) error
	ListFAQs(context.Context, string, string) ([]domain.FAQ, error)
	CreateFAQ(context.Context, string, string, domain.FAQ) (domain.FAQ, error)
	UpdateFAQ(context.Context, string, string, string, domain.FAQ, int64) (domain.FAQ, error)
	DeleteFAQ(context.Context, string, string, string) error
}

type SafetyAuditRepository interface {
	RecordSafetyAudit(context.Context, string, string, string, domain.SafetyDecision, string) error
}

type PublicationRepository interface {
	RecordPublicationValidation(context.Context, string, string, int64, time.Time) (domain.SmartAssistant, error)
	PublicationStats(context.Context, string, string, time.Time) (domain.PublicationStats, error)
}

func (service *Service) RecordSafetyAudit(ctx context.Context, owner, assistantID, source string, decision domain.SafetyDecision, creditOutcome string) error {
	repository, ok := service.repository.(SafetyAuditRepository)
	if !ok {
		return nil
	}
	return repository.RecordSafetyAudit(ctx, owner, assistantID, source, decision, creditOutcome)
}

func (service *Service) BindAssistantSession(ctx context.Context, owner, assistantID, sessionID string) error {
	assistant, err := service.repository.GetAssistant(ctx, owner, assistantID)
	if err != nil {
		return err
	}
	if assistant.State != domain.StateEnabled {
		return fmt.Errorf("%w: assistant is disabled", domain.ErrInvalid)
	}
	if assistant.LastValidatedAt == nil || assistant.ValidatedVersion != assistant.Version {
		return fmt.Errorf("%w: assistant publication validation is stale", domain.ErrInvalid)
	}
	if err := assistant.ValidateForEnable(); err != nil {
		return err
	}
	snapshot, err := json.Marshal(assistant)
	if err != nil {
		return err
	}
	return service.repository.BindAssistantSession(ctx, owner, assistantID, sessionID, snapshot)
}

type Service struct {
	repository Repository
	knowledge  KnowledgeRepository
	embedder   EmbeddingProvider
}

func New(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("AI Application Repository is required")
	}
	return &Service{repository: repository}, nil
}

func (service *Service) Repository() Repository { return service.repository }

func (service *Service) SetEmbeddingProvider(provider EmbeddingProvider) {
	service.embedder = provider
}

func (service *Service) GetEmbeddingConfiguration(ctx context.Context) (domain.EmbeddingConfiguration, error) {
	repository, ok := service.repository.(EmbeddingConfigurationRepository)
	if !ok {
		return domain.EmbeddingConfiguration{}, fmt.Errorf("embedding configuration repository is unavailable")
	}
	return repository.GetEmbeddingConfiguration(ctx)
}

func (service *Service) SaveEmbeddingConfiguration(ctx context.Context, configuration domain.EmbeddingConfiguration, apiKey []byte) (domain.EmbeddingConfiguration, error) {
	if err := configuration.Validate(); err != nil {
		return domain.EmbeddingConfiguration{}, err
	}
	repository, ok := service.repository.(EmbeddingConfigurationRepository)
	if !ok {
		return domain.EmbeddingConfiguration{}, fmt.Errorf("embedding configuration repository is unavailable")
	}
	return repository.SaveEmbeddingConfiguration(ctx, configuration, apiKey)
}

func (service *Service) CreateExternalConversation(ctx context.Context, conversation domain.ExternalConversation) (domain.ExternalConversation, error) {
	repository, ok := service.repository.(ExternalConversationRepository)
	if !ok {
		return domain.ExternalConversation{}, fmt.Errorf("external conversation repository is unavailable")
	}
	return repository.CreateExternalConversation(ctx, conversation)
}

func (service *Service) GetExternalConversation(ctx context.Context, id, assistantID, visitorHash string) (domain.ExternalConversation, error) {
	repository, ok := service.repository.(ExternalConversationRepository)
	if !ok {
		return domain.ExternalConversation{}, fmt.Errorf("external conversation repository is unavailable")
	}
	return repository.GetExternalConversation(ctx, id, assistantID, visitorHash)
}

func (service *Service) CreateExternalResponse(ctx context.Context, response domain.ExternalResponse) (domain.ExternalResponse, error) {
	repository, ok := service.repository.(ExternalConversationRepository)
	if !ok {
		return domain.ExternalResponse{}, fmt.Errorf("external conversation repository is unavailable")
	}
	return repository.CreateExternalResponse(ctx, response)
}

func (service *Service) GetExternalResponse(ctx context.Context, id, conversationID, visitorHash string) (domain.ExternalResponse, error) {
	repository, ok := service.repository.(ExternalConversationRepository)
	if !ok {
		return domain.ExternalResponse{}, fmt.Errorf("external conversation repository is unavailable")
	}
	conversation, err := repository.GetExternalConversation(ctx, conversationID, "", visitorHash)
	if err != nil {
		return domain.ExternalResponse{}, err
	}
	return repository.GetExternalResponse(ctx, id, conversation.ID, visitorHash)
}

func (service *Service) ConsumeExternalRate(ctx context.Context, scope, key string, now time.Time, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}
	repository, ok := service.repository.(ExternalRateLimitRepository)
	if !ok {
		return false, fmt.Errorf("external rate-limit repository is unavailable")
	}
	return repository.ConsumeExternalRate(ctx, scope, key, now, limit)
}

func (service *Service) ListAssistants(ctx context.Context, owner string) ([]domain.SmartAssistant, error) {
	assistants, err := service.repository.ListAssistants(ctx, owner)
	for i := range assistants {
		assistants[i] = withFreeQuestions(assistants[i])
	}
	return assistants, err
}
func (service *Service) GetAssistant(ctx context.Context, owner, id string) (domain.SmartAssistant, error) {
	assistant, err := service.repository.GetAssistant(ctx, owner, id)
	return withFreeQuestions(assistant), err
}
func (service *Service) CreateAssistant(ctx context.Context, owner string, assistant domain.SmartAssistant) (domain.SmartAssistant, error) {
	assistant = withFreeQuestions(assistant)
	assistant.OwnerID = owner
	if assistant.State == "" {
		assistant.State = domain.StateDraft
	}
	if assistant.Share.Enabled {
		token, err := newShareToken()
		if err != nil {
			return domain.SmartAssistant{}, err
		}
		assistant.Share.Token = token
		assistant.Share.TokenHash = hashShareToken(token)
		assistant.Share.TokenRevision = 1
	}
	if len(assistant.KnowledgeBaseIDs) > 0 && service.knowledge != nil {
		for _, baseID := range assistant.KnowledgeBaseIDs {
			if err := service.validateKnowledgeBinding(ctx, owner, baseID); err != nil {
				return domain.SmartAssistant{}, err
			}
		}
	} else if len(assistant.KnowledgeBaseIDs) > 0 {
		return domain.SmartAssistant{}, fmt.Errorf("%w: knowledge repository is unavailable", domain.ErrInvalid)
	}
	if assistant.Share.Width == "" {
		assistant.Share.Width = "100%"
	}
	if assistant.Share.Height == 0 {
		assistant.Share.Height = 600
	}
	if assistant.State == domain.StateEnabled {
		if err := assistant.ValidateForEnable(); err != nil {
			return domain.SmartAssistant{}, err
		}
	} else if err := assistant.Validate(); err != nil {
		return domain.SmartAssistant{}, err
	}
	return service.repository.CreateAssistant(ctx, owner, assistant)
}

// CopyAssistant creates a new editable draft from visible assistant
// configuration. Share credentials and conversation identity are intentionally
// excluded from the copy.
func (service *Service) CopyAssistant(ctx context.Context, owner, id string) (domain.SmartAssistant, error) {
	source, err := service.GetAssistant(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	copy := source
	copy.ID, copy.OwnerID, copy.CreatedAt, copy.UpdatedAt, copy.Version = "", "", time.Time{}, time.Time{}, 0
	copy.State = domain.StateDraft
	copy.Share.Enabled = false
	copy.Share.Token, copy.Share.TokenHash, copy.Share.TokenRevision = "", "", 0
	created, err := service.repository.CreateAssistant(ctx, owner, copy)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	faqs, err := service.repository.ListFAQs(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	for _, faq := range faqs {
		faq.ID, faq.AssistantID, faq.CreatedAt, faq.UpdatedAt, faq.Version = "", "", time.Time{}, time.Time{}, 0
		if _, err := service.CreateFAQ(ctx, owner, created.ID, faq); err != nil {
			return domain.SmartAssistant{}, err
		}
	}
	return service.GetAssistant(ctx, owner, created.ID)
}

func (service *Service) SetAssistantState(ctx context.Context, owner, id string, state domain.ApplicationState, version int64) (domain.SmartAssistant, error) {
	if state != domain.StateDraft && state != domain.StateEnabled && state != domain.StateDisabled {
		return domain.SmartAssistant{}, fmt.Errorf("%w: unsupported assistant state", domain.ErrInvalid)
	}
	assistant, err := service.GetAssistant(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if state == domain.StateEnabled {
		if err := assistant.ValidateForEnable(); err != nil {
			return domain.SmartAssistant{}, err
		}
		if err := service.validateAssistantResources(ctx, owner, assistant); err != nil {
			return domain.SmartAssistant{}, err
		}
	}
	assistant.State = state
	return service.repository.UpdateAssistant(ctx, owner, id, assistant, version)
}

func newShareToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
func hashShareToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
func (service *Service) UpdateAssistant(ctx context.Context, owner, id string, assistant domain.SmartAssistant, version int64) (domain.SmartAssistant, error) {
	assistant = withFreeQuestions(assistant)
	assistant.OwnerID = owner
	issuedShareToken := ""
	current, err := service.repository.GetAssistant(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if assistant.State == "" {
		assistant.State = current.State
	}
	if assistant.Share.Enabled {
		assistant.Share.TokenHash = current.Share.TokenHash
		assistant.Share.TokenRevision = current.Share.TokenRevision
		if assistant.Share.TokenHash == "" {
			token, tokenErr := newShareToken()
			if tokenErr != nil {
				return domain.SmartAssistant{}, tokenErr
			}
			assistant.Share.Token = token
			issuedShareToken = token
			assistant.Share.TokenHash = hashShareToken(token)
			assistant.Share.TokenRevision++
		}
	} else {
		assistant.Share.Token, assistant.Share.TokenHash, assistant.Share.TokenRevision = "", "", current.Share.TokenRevision
	}
	if len(assistant.KnowledgeBaseIDs) > 0 && service.knowledge != nil {
		for _, baseID := range assistant.KnowledgeBaseIDs {
			if err := service.validateKnowledgeBinding(ctx, owner, baseID); err != nil {
				return domain.SmartAssistant{}, err
			}
		}
	} else if len(assistant.KnowledgeBaseIDs) > 0 {
		return domain.SmartAssistant{}, fmt.Errorf("%w: knowledge repository is unavailable", domain.ErrInvalid)
	}
	if assistant.Share.Width == "" {
		assistant.Share.Width = "100%"
	}
	if assistant.Share.Height == 0 {
		assistant.Share.Height = 600
	}
	if assistant.State == domain.StateEnabled {
		if err := assistant.ValidateForEnable(); err != nil {
			return domain.SmartAssistant{}, err
		}
		if err := service.validateAssistantResources(ctx, owner, assistant); err != nil {
			return domain.SmartAssistant{}, err
		}
	} else if err := assistant.Validate(); err != nil {
		return domain.SmartAssistant{}, err
	}
	updated, err := service.repository.UpdateAssistant(ctx, owner, id, assistant, version)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	updated.Share.Token = issuedShareToken
	return updated, nil
}

func (service *Service) RegenerateShareToken(ctx context.Context, owner, id string, version int64) (domain.SmartAssistant, error) {
	assistant, err := service.GetAssistant(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if !assistant.Share.Enabled {
		return domain.SmartAssistant{}, fmt.Errorf("%w: sharing is disabled", domain.ErrInvalid)
	}
	validationWasCurrent := assistant.LastValidatedAt != nil && assistant.ValidatedVersion == assistant.Version
	token, err := newShareToken()
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	assistant.Share.Token, assistant.Share.TokenHash = token, hashShareToken(token)
	assistant.Share.TokenRevision++
	updated, err := service.repository.UpdateAssistant(ctx, owner, id, assistant, version)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if validationWasCurrent {
		if repository, ok := service.repository.(PublicationRepository); ok {
			updated, err = repository.RecordPublicationValidation(ctx, owner, id, updated.Version, time.Now().UTC())
			if err != nil {
				return domain.SmartAssistant{}, err
			}
		}
	}
	updated.Share.Token = token
	return updated, nil
}

func (service *Service) ResolveSharedAssistant(ctx context.Context, token string) (domain.SmartAssistant, error) {
	if len(token) < 32 {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	assistant, err := service.repository.GetAssistantByShareTokenHash(ctx, hashShareToken(token))
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if !assistant.Share.Enabled || assistant.State != domain.StateEnabled {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	if assistant.LastValidatedAt == nil || assistant.ValidatedVersion != assistant.Version {
		return domain.SmartAssistant{}, domain.ErrNotFound
	}
	return withFreeQuestions(assistant), nil
}

// Keep the legacy JSON fields readable by older clients while ignoring their saved values.
func withFreeQuestions(assistant domain.SmartAssistant) domain.SmartAssistant {
	assistant.Share.FreeTextEnabled = true
	assistant.Share.DailyCallLimit = 0
	return assistant
}

func (service *Service) RecordPublicationValidation(ctx context.Context, owner, id string, version int64, checkedAt time.Time) (domain.SmartAssistant, error) {
	repository, ok := service.repository.(PublicationRepository)
	if !ok {
		return domain.SmartAssistant{}, fmt.Errorf("publication repository is unavailable")
	}
	assistant, err := repository.RecordPublicationValidation(ctx, owner, id, version, checkedAt.UTC())
	return withFreeQuestions(assistant), err
}

func (service *Service) PublicationStats(ctx context.Context, owner, id string, windowDays int) (domain.PublicationStats, error) {
	if windowDays < 1 || windowDays > 90 {
		windowDays = 30
	}
	repository, ok := service.repository.(PublicationRepository)
	if !ok {
		return domain.PublicationStats{}, fmt.Errorf("publication repository is unavailable")
	}
	stats, err := repository.PublicationStats(ctx, owner, id, time.Now().UTC().AddDate(0, 0, -windowDays))
	stats.WindowDays = windowDays
	return stats, err
}

func (service *Service) DeleteAssistant(ctx context.Context, owner, id string) error {
	return service.repository.DeleteAssistant(ctx, owner, id)
}

func (service *Service) validateKnowledgeBinding(ctx context.Context, owner, id string) error {
	if service.knowledge == nil {
		return fmt.Errorf("%w: knowledge repository is unavailable", domain.ErrInvalid)
	}
	base, err := service.knowledge.GetKnowledgeBase(ctx, owner, id)
	if err != nil {
		return err
	}
	if base.State != "" && base.State != domain.KnowledgeReady {
		return fmt.Errorf("%w: knowledge base is unavailable", domain.ErrInvalid)
	}
	return nil
}

func (service *Service) validateAssistantResources(ctx context.Context, owner string, assistant domain.SmartAssistant) error {
	for _, baseID := range assistant.KnowledgeBaseIDs {
		if err := service.validateKnowledgeBinding(ctx, owner, baseID); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) ListFAQs(ctx context.Context, owner, assistantID string) ([]domain.FAQ, error) {
	return service.repository.ListFAQs(ctx, owner, assistantID)
}
func (service *Service) CreateFAQ(ctx context.Context, owner, assistantID string, faq domain.FAQ) (domain.FAQ, error) {
	faq.AssistantID = assistantID
	if err := faq.Validate(); err != nil {
		return domain.FAQ{}, err
	}
	if err := domain.DefaultSafetyPolicy().ValidateAnswer(faq.AnswerMarkdown); err != nil {
		return domain.FAQ{}, err
	}
	return service.repository.CreateFAQ(ctx, owner, assistantID, faq)
}
func (service *Service) UpdateFAQ(ctx context.Context, owner, assistantID, id string, faq domain.FAQ, version int64) (domain.FAQ, error) {
	faq.AssistantID = assistantID
	if err := faq.Validate(); err != nil {
		return domain.FAQ{}, err
	}
	if err := domain.DefaultSafetyPolicy().ValidateAnswer(faq.AnswerMarkdown); err != nil {
		return domain.FAQ{}, err
	}
	return service.repository.UpdateFAQ(ctx, owner, assistantID, id, faq, version)
}
func (service *Service) DeleteFAQ(ctx context.Context, owner, assistantID, id string) error {
	return service.repository.DeleteFAQ(ctx, owner, assistantID, id)
}
