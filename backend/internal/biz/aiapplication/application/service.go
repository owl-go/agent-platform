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
	ListDigitalHumans(context.Context, string) ([]domain.DigitalHuman, error)
	GetDigitalHuman(context.Context, string, string) (domain.DigitalHuman, error)
	CreateDigitalHuman(context.Context, string, domain.DigitalHuman) (domain.DigitalHuman, error)
	UpdateDigitalHuman(context.Context, string, string, domain.DigitalHuman, int64) (domain.DigitalHuman, error)
	DeleteDigitalHuman(context.Context, string, string) error
	ListFAQs(context.Context, string, string) ([]domain.FAQ, error)
	CreateFAQ(context.Context, string, string, domain.FAQ) (domain.FAQ, error)
	UpdateFAQ(context.Context, string, string, string, domain.FAQ, int64) (domain.FAQ, error)
	DeleteFAQ(context.Context, string, string, string) error
}

type ShareUsageRepository interface {
	ConsumeShareCall(context.Context, string, time.Time, int) (bool, error)
}

func (service *Service) BindAssistantSession(ctx context.Context, owner, assistantID, sessionID string) error {
	assistant, err := service.repository.GetAssistant(ctx, owner, assistantID)
	if err != nil {
		return err
	}
	if assistant.State != domain.StateEnabled {
		return fmt.Errorf("%w: assistant is disabled", domain.ErrInvalid)
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
	return service.repository.ListAssistants(ctx, owner)
}
func (service *Service) GetAssistant(ctx context.Context, owner, id string) (domain.SmartAssistant, error) {
	return service.repository.GetAssistant(ctx, owner, id)
}
func (service *Service) CreateAssistant(ctx context.Context, owner string, assistant domain.SmartAssistant) (domain.SmartAssistant, error) {
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
	if assistant.DigitalHumanID != nil {
		if _, err := service.repository.GetDigitalHuman(ctx, owner, *assistant.DigitalHumanID); err != nil {
			return domain.SmartAssistant{}, err
		}
	}
	if len(assistant.KnowledgeBaseIDs) > 0 && service.knowledge != nil {
		for _, baseID := range assistant.KnowledgeBaseIDs {
			if _, err := service.knowledge.GetKnowledgeBase(ctx, owner, baseID); err != nil {
				return domain.SmartAssistant{}, err
			}
		}
	}
	if assistant.Share.Width == "" {
		assistant.Share.Width = "100%"
	}
	if assistant.Share.Height == 0 {
		assistant.Share.Height = 600
	}
	if err := assistant.Validate(); err != nil {
		return domain.SmartAssistant{}, err
	}
	return service.repository.CreateAssistant(ctx, owner, assistant)
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
	assistant.OwnerID = owner
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
			assistant.Share.TokenHash = hashShareToken(token)
			assistant.Share.TokenRevision++
		}
	} else {
		assistant.Share.Token, assistant.Share.TokenHash, assistant.Share.TokenRevision = "", "", current.Share.TokenRevision
	}
	if assistant.DigitalHumanID != nil {
		if _, err := service.repository.GetDigitalHuman(ctx, owner, *assistant.DigitalHumanID); err != nil {
			return domain.SmartAssistant{}, err
		}
	}
	if len(assistant.KnowledgeBaseIDs) > 0 && service.knowledge != nil {
		for _, baseID := range assistant.KnowledgeBaseIDs {
			if _, err := service.knowledge.GetKnowledgeBase(ctx, owner, baseID); err != nil {
				return domain.SmartAssistant{}, err
			}
		}
	}
	if assistant.Share.Width == "" {
		assistant.Share.Width = "100%"
	}
	if assistant.Share.Height == 0 {
		assistant.Share.Height = 600
	}
	if err := assistant.Validate(); err != nil {
		return domain.SmartAssistant{}, err
	}
	return service.repository.UpdateAssistant(ctx, owner, id, assistant, version)
}

func (service *Service) RegenerateShareToken(ctx context.Context, owner, id string, version int64) (domain.SmartAssistant, error) {
	assistant, err := service.repository.GetAssistant(ctx, owner, id)
	if err != nil {
		return domain.SmartAssistant{}, err
	}
	if !assistant.Share.Enabled {
		return domain.SmartAssistant{}, fmt.Errorf("%w: sharing is disabled", domain.ErrInvalid)
	}
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
	return assistant, nil
}

func (service *Service) ConsumeSharedAssistantCall(ctx context.Context, assistantID string, dailyLimit int) (bool, error) {
	if dailyLimit <= 0 {
		return true, nil
	}
	repository, ok := service.repository.(ShareUsageRepository)
	if !ok {
		return true, nil
	}
	return repository.ConsumeShareCall(ctx, assistantID, time.Now().UTC(), dailyLimit)
}
func (service *Service) DeleteAssistant(ctx context.Context, owner, id string) error {
	return service.repository.DeleteAssistant(ctx, owner, id)
}

func (service *Service) ListDigitalHumans(ctx context.Context, owner string) ([]domain.DigitalHuman, error) {
	return service.repository.ListDigitalHumans(ctx, owner)
}
func (service *Service) GetDigitalHuman(ctx context.Context, owner, id string) (domain.DigitalHuman, error) {
	return service.repository.GetDigitalHuman(ctx, owner, id)
}
func (service *Service) CreateDigitalHuman(ctx context.Context, owner string, human domain.DigitalHuman) (domain.DigitalHuman, error) {
	human.OwnerID = owner
	if err := human.Validate(); err != nil {
		return domain.DigitalHuman{}, err
	}
	return service.repository.CreateDigitalHuman(ctx, owner, human)
}
func (service *Service) UpdateDigitalHuman(ctx context.Context, owner, id string, human domain.DigitalHuman, version int64) (domain.DigitalHuman, error) {
	human.OwnerID = owner
	if err := human.Validate(); err != nil {
		return domain.DigitalHuman{}, err
	}
	return service.repository.UpdateDigitalHuman(ctx, owner, id, human, version)
}
func (service *Service) DeleteDigitalHuman(ctx context.Context, owner, id string) error {
	return service.repository.DeleteDigitalHuman(ctx, owner, id)
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
