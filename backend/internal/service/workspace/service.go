package workspace

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	aiapplication "agent-platform/backend/internal/biz/aiapplication/application"
	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	aicreationapplication "agent-platform/backend/internal/biz/aicreation/application"
	aicreationdomain "agent-platform/backend/internal/biz/aicreation/domain"
	creditsapplication "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/feishucli"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"agent-platform/backend/internal/knowledgebase/retrieval"
	"agent-platform/backend/internal/objectstore"
	"agent-platform/backend/internal/platformconfig"
	"agent-platform/backend/internal/productanalytics"
	"agent-platform/backend/internal/secretcrypto"
	"agent-platform/backend/internal/skillstore"
	"agent-platform/backend/internal/workspacefs"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	kratoshttp "github.com/go-kratos/kratos/v3/transport/http"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	workspacev1.UnimplementedAgentWorkspaceServiceServer
	accounts                 *accountapplication.Service
	credits                  *creditsapplication.Service
	aicreation               *aicreationapplication.Service
	aiapplications           *aiapplication.Service
	assistantChatModel       aiapplication.ChatModel
	activeAssistantTurns     sync.Map
	workspace                *workspaceapplication.Service
	box                      *secretcrypto.Box
	files                    *workspacefs.Store
	skills                   *skillstore.Store
	objects                  objectstore.Provider
	knowledgeSearch          retrieval.Searcher
	config                   platformconfig.Config
	analytics                productanalytics.Observer
	feishu                   feishuApplicationRegistrar
	removeNativeSessionState func(string, string, string) error
	cloneGitSource           func(context.Context, string, workspacefs.GitCloneOptions) error
}

func (service *Service) RegisterHTTP(server *kratoshttp.Server) {
	workspacev1.RegisterAgentWorkspaceServiceHTTPServer(server, service)
	server.Handle("/api/v1/sessions/{session_id}/messages/{message_id}/events", http.HandlerFunc(service.streamSessionMessage))
	server.Handle("/api/v1/resource-creation-actions/", http.HandlerFunc(service.decideResourceCreationAction))
	server.Handle("/api/v1/sessions/{session_id}/artifacts/{artifact_id}/download", http.HandlerFunc(service.downloadSessionArtifact))
	server.Handle("/api/v1/workflows/{workflow_id}/runs/{run_id}/events", http.HandlerFunc(service.streamRunEvents))
	server.Handle("/api/v1/workflows/{workflow_id}/artifacts/{artifact_id}/download", http.HandlerFunc(service.downloadWorkflowArtifact))
	server.Handle("/api/v1/workflows/{workflow_id}/workspace/download", http.HandlerFunc(service.downloadWorkspaceFile))
	server.Handle("/api/v1/attachments/upload", http.HandlerFunc(service.uploadAttachment))
	server.Handle("/api/v1/attachments/{attachment_id}/download", http.HandlerFunc(service.downloadAttachment))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/upload", http.HandlerFunc(service.uploadKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/search", http.HandlerFunc(service.searchKnowledgeBase))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/import", http.HandlerFunc(service.importKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/{document_id}/retry", http.HandlerFunc(service.retryKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/{document_id}/regenerate", http.HandlerFunc(service.regenerateKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/restore", http.HandlerFunc(service.restoreKnowledgeBase))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/categories/{category_id}/restore", http.HandlerFunc(service.restoreKnowledgeCategory))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/{document_id}", http.HandlerFunc(service.mutateKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/{document_id}/restore", http.HandlerFunc(service.restoreKnowledgeDocument))
	server.Handle("/api/v1/knowledge-bases/{knowledge_base_id}/documents/{document_id}/download", http.HandlerFunc(service.downloadKnowledgeDocument))
	server.Handle("/api/v1/ai-creation/image-generations/{record_id}/images/{position}", http.HandlerFunc(service.downloadGeneratedImage))
	server.Handle("/api/v1/ai-creation/reference-images", http.HandlerFunc(service.uploadReferenceImage))
	server.Handle("/api/v1/ai-creation/reference-images/{upload_id}", http.HandlerFunc(service.deleteReferenceImage))
	server.Handle("/api/v1/ai-creation/image-generations/{record_id}/download", http.HandlerFunc(service.downloadGeneratedImages))
	server.Handle("/api/v1/ai-creation/image-generations/{record_id}/events", http.HandlerFunc(service.streamImageGeneration))
	server.Handle("/api/v1/ai-apps/assistants", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/icon", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/faqs", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/faqs/{faq_id}", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/answer", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/share-token", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/sessions", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/conversations", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/conversations/{conversation_id}", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/conversations/{conversation_id}/turns", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/conversations/{conversation_id}/turns/{turn_id}/cancel", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/copy", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/assistants/{assistant_id}/state", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/knowledge-bases", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/knowledge-bases/{knowledge_base_id}/documents", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/embedding-provider", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/digital-humans", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/digital-humans/{digital_human_id}", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/digital-humans/{digital_human_id}/copy", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/digital-humans/{digital_human_id}/state", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/ai-apps/digital-humans/{digital_human_id}/preview", http.HandlerFunc(service.aiApplicationsHandler))
	server.Handle("/api/v1/public/assistants/{share_token}", http.HandlerFunc(service.publicAssistantHandler))
	server.Handle("/api/v1/public/assistants/{share_token}/answer", http.HandlerFunc(service.publicAssistantHandler))
	server.Handle("/api/v1/public/assistants/{share_token}/conversations/{conversation_id}/responses/{response_id}", http.HandlerFunc(service.publicAssistantHandler))
	server.Handle("/embed/assistant/{share_token}", http.HandlerFunc(service.publicAssistantEmbed))
}

func New(accounts *accountapplication.Service, credits *creditsapplication.Service, aicreation *aicreationapplication.Service, aiapplications *aiapplication.Service, chatModel aiapplication.ChatModel, workspace *workspaceapplication.Service, box *secretcrypto.Box, files *workspacefs.Store, skills *skillstore.Store, objects objectstore.Provider, analytics productanalytics.Observer, config platformconfig.Config) (*Service, error) {
	if accounts == nil || credits == nil || aicreation == nil || aiapplications == nil || chatModel == nil || workspace == nil || box == nil || files == nil || skills == nil || objects == nil || analytics == nil {
		return nil, fmt.Errorf("Account, Credits, AI Creation, AI Applications, Agent Workspace, encryption, Workspace File, Skill, Object Store, and product analytics services are required")
	}
	service := &Service{accounts: accounts, credits: credits, aicreation: aicreation, aiapplications: aiapplications, assistantChatModel: chatModel, workspace: workspace, box: box, files: files, skills: skills, objects: objects, config: config, analytics: analytics, feishu: feishucli.NewRegistrar(nil), removeNativeSessionState: workspacefs.RemoveNativeSessionState, cloneGitSource: files.Clone}
	if strings.TrimSpace(config.AnythingLLM.Endpoint) != "" {
		provider, err := anythingllm.NewClient(config.AnythingLLM.Endpoint, config.AnythingLLM.APIKey, config.AnythingLLM.Timeout.Value())
		if err != nil {
			return nil, err
		}
		service.knowledgeSearch, err = retrieval.New(workspace.Repository(), provider)
		if err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (service *Service) EnableProductAnalytics(observer productanalytics.Observer) {
	if observer != nil {
		service.analytics = observer
	}
}

func (service *Service) productAnalytics() productanalytics.Observer {
	if service.analytics == nil {
		return productanalytics.Nop{}
	}
	return service.analytics
}

func (service *Service) owner(ctx context.Context) (string, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return "", publicError(err)
	}
	return principal.UserID, nil
}

func (service *Service) validateExecutionRuntimes(ctx context.Context, owner string, expertID, teamID *string) error {
	check := func(expert workspacedomain.Expert) error {
		availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{expert})
		if err != nil {
			return err
		}
		if !availability[expert.ID].Available {
			return fmt.Errorf("%w: selected Expert execution profile is unavailable", workspacedomain.ErrInvalid)
		}
		return nil
	}
	if expertID != nil {
		expert, err := service.workspace.Repository().GetExpert(ctx, owner, *expertID)
		if err != nil {
			return err
		}
		return check(expert)
	}
	if teamID != nil {
		team, err := service.workspace.Repository().GetExpertTeam(ctx, owner, *teamID)
		if err != nil {
			return err
		}
		for _, expert := range team.Experts {
			if err := check(expert); err != nil {
				return err
			}
		}
		return nil
	}
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return err
	}
	runtime, exists := service.config.Worker.Runtimes[string(settings.DefaultRuntimeEngine)]
	if !exists || !runtime.Available {
		return fmt.Errorf("%w: default Runtime Engine is unavailable", workspacedomain.ErrInvalid)
	}
	return nil
}

func (service *Service) validateExpertInputAvailability(ctx context.Context, input workspacedomain.ExpertInput) error {
	expert := workspacedomain.Expert{ID: "candidate", Introduction: input.Introduction, CoreCapability: input.CoreCapability, OperatingProcedure: input.OperatingProcedure, OutputStandard: input.OutputStandard, ExecutionInstruction: input.ExecutionInstruction, ProviderModelID: input.ProviderModelID, RuntimeEngine: input.RuntimeEngine}
	availability, err := service.expertAvailability(ctx, []workspacedomain.Expert{expert})
	if err != nil {
		return err
	}
	if !availability[expert.ID].Available {
		return fmt.Errorf("%w: Expert execution profile is unavailable", workspacedomain.ErrInvalid)
	}
	return nil
}

func publicError(err error) error {
	var providerFailure *aicreationapplication.ProviderFailure
	switch {
	case errors.Is(err, accountdomain.ErrUnauthenticated):
		return kratoserrors.New(http.StatusUnauthorized, "authentication_required", "authentication required")
	case errors.Is(err, accountdomain.ErrForbidden):
		return kratoserrors.New(http.StatusForbidden, "access_denied", "access denied")
	case errors.Is(err, accountdomain.ErrNotFound), errors.Is(err, workspacedomain.ErrNotFound):
		return kratoserrors.New(http.StatusNotFound, "resource_not_found", "resource not found")
	case errors.Is(err, accountdomain.ErrConflict), errors.Is(err, workspacedomain.ErrConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "version_conflict", "resource version changed")
	case errors.Is(err, workspacedomain.ErrQueueFull):
		return kratoserrors.New(http.StatusTooManyRequests, "queue_full", "workflow queue is full")
	case errors.Is(err, workspacedomain.ErrWorkflowCredentialUnavailable):
		return kratoserrors.New(http.StatusUnprocessableEntity, "workflow_credential_secret_unavailable", "Workflow API Secret is unavailable; regenerate the credential")
	case errors.Is(err, workspacedomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, creditsdomain.ErrInsufficientCredits):
		public := kratoserrors.New(http.StatusTooManyRequests, "insufficient_credits", "Credit Balance is not positive")
		var insufficient *creditsdomain.InsufficientCreditsError
		if errors.As(err, &insufficient) {
			public = public.WithMetadata(map[string]string{
				"balance_hundredths": fmt.Sprintf("%d", insufficient.Balance),
				"next_allocation_at": insufficient.NextAllocationAt.UTC().Format(time.RFC3339Nano),
			})
		}
		return public
	case errors.Is(err, creditsdomain.ErrCodeUnavailable):
		return kratoserrors.New(http.StatusUnprocessableEntity, "redemption_code_unavailable", "Redemption Code is unavailable")
	case errors.Is(err, creditsdomain.ErrRedemptionDisabled):
		return kratoserrors.New(http.StatusForbidden, "redemption_codes_disabled", "Redemption Codes are disabled")
	case errors.Is(err, creditsdomain.ErrConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "credit_conflict", "Credits state changed")
	case errors.Is(err, creditsdomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, aicreationdomain.ErrNotFound):
		return kratoserrors.New(http.StatusNotFound, "resource_not_found", "resource not found")
	case errors.Is(err, aicreationdomain.ErrConflict):
		return kratoserrors.New(http.StatusConflict, "active_image_generation_conflict", "an Image Generation Record is already active")
	case errors.Is(err, aicreationdomain.ErrVersionConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "version_conflict", "resource version changed")
	case errors.Is(err, aicreationapplication.ErrInsufficientCredits):
		return kratoserrors.New(http.StatusTooManyRequests, "insufficient_credits", "Available Credit is insufficient")
	case errors.Is(err, aicreationdomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.Is(err, aiapplicationdomain.ErrNotFound):
		return kratoserrors.New(http.StatusNotFound, "resource_not_found", "resource not found")
	case errors.Is(err, aiapplicationdomain.ErrConflict):
		return kratoserrors.New(http.StatusConflict, "resource_conflict", "resource conflicts with current state")
	case errors.Is(err, aiapplicationdomain.ErrVersionConflict):
		return kratoserrors.New(http.StatusPreconditionFailed, "version_conflict", "resource version changed")
	case errors.Is(err, aiapplicationdomain.ErrInvalid):
		return kratoserrors.New(http.StatusUnprocessableEntity, "invalid_input", err.Error())
	case errors.As(err, &providerFailure):
		status := http.StatusUnprocessableEntity
		if providerFailure.Code == "image_provider_unavailable" || providerFailure.Code == "image_provider_rate_limited" || providerFailure.Code == "image_outcome_unknown" {
			status = http.StatusServiceUnavailable
		}
		return kratoserrors.New(status, providerFailure.Code, providerFailure.Code)
	default:
		return kratoserrors.New(http.StatusInternalServerError, "request_failed", "request failed")
	}
}

func randomCredential(prefix string, size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSecret(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(hash), err
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
