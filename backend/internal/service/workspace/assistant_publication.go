package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"

	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

const assistantPublicationStatsWindowDays = 30

func (service *Service) assistantPublicationCheck(ctx context.Context, owner string, assistant aiapplicationdomain.SmartAssistant) aiapplicationdomain.PublicationValidation {
	validation := aiapplicationdomain.PublicationValidation{
		AssistantID:      assistant.ID,
		AssistantVersion: assistant.Version,
		CheckedAt:        time.Now().UTC(),
		Checks:           make([]aiapplicationdomain.PublicationCheck, 0, 7),
	}
	add := func(code string, err error, readyDetail, failedDetail string) {
		check := aiapplicationdomain.PublicationCheck{Code: code, Ready: err == nil, Detail: readyDetail}
		if err != nil {
			check.Detail = failedDetail
		}
		validation.Checks = append(validation.Checks, check)
	}

	configuration := assistant
	configuration.Share.Enabled = false
	add("configuration_complete", configuration.ValidateForEnable(), "Required configuration is complete.", "Name, icon, scenario, or visible instructions are incomplete.")
	add("model_available", service.validateAssistantModel(ctx, owner, assistant.ProviderModelID), "The selected model is available.", "The selected model or its credential is unavailable.")

	faqs, faqErr := service.aiapplications.ListFAQs(ctx, owner, assistant.ID)
	if faqErr == nil {
		for _, faq := range faqs {
			if faq.Enabled {
				if err := aiapplicationdomain.DefaultSafetyPolicy().ValidateAnswer(faq.AnswerMarkdown); err != nil {
					faqErr = err
					break
				}
			}
		}
	}
	add("faq_safe", faqErr, "Enabled FAQs passed the publication safety check.", "An enabled FAQ failed the publication safety check.")

	var knowledgeErr error
	for _, id := range assistant.KnowledgeBaseIDs {
		base, err := service.workspace.Repository().GetKnowledgeBase(ctx, owner, id, false, false)
		if err != nil {
			knowledgeErr = err
			break
		}
		if base.ReadyDocumentCount < 1 {
			knowledgeErr = fmt.Errorf("knowledge base has no Ready document")
			break
		}
	}
	add("knowledge_ready", knowledgeErr, "Every selected Knowledge Base has a Ready document.", "A selected Knowledge Base is inaccessible or has no Ready document.")

	var resourceErr error
	if assistant.ExpertID != nil || assistant.ExpertTeamID != nil {
		resourceErr = service.validateExecutionRuntimes(ctx, owner, assistant.ExpertID, assistant.ExpertTeamID)
	}
	add("resources_available", resourceErr, "Referenced resources are available.", "A referenced Expert or Team is unavailable.")

	creditErr := service.credits.RequirePositiveBalance(ctx, owner, service.userTimezone(ctx, owner))
	add("credit_policy_ready", creditErr, "The owner has an explicit usable Credit budget.", "The owner has no usable Credit budget for model calls.")

	var shareErr error
	if assistant.Share.Enabled {
		if len(assistant.Share.AllowedOrigins) == 0 || assistant.Share.DailyCallLimit < 1 || !assistant.Share.DataProcessingAcknowledged {
			shareErr = fmt.Errorf("controlled sharing settings are incomplete")
		}
		for _, origin := range assistant.Share.AllowedOrigins {
			if strings.TrimSpace(origin) == "" {
				shareErr = fmt.Errorf("allowed origin is empty")
			}
		}
	}
	add("share_controls", shareErr, "Sharing is off or has explicit origins, a daily cap, and data acknowledgement.", "Sharing requires explicit origins, a positive daily cap, and data acknowledgement.")

	validation.Ready = true
	for _, check := range validation.Checks {
		if !check.Ready {
			validation.Ready = false
			break
		}
	}
	return validation
}

func (service *Service) recordAssistantPublicationValidation(ctx context.Context, owner string, validation aiapplicationdomain.PublicationValidation) (aiapplicationdomain.SmartAssistant, error) {
	if !validation.Ready {
		return aiapplicationdomain.SmartAssistant{}, fmt.Errorf("%w: assistant publication check failed", aiapplicationdomain.ErrInvalid)
	}
	return service.aiapplications.RecordPublicationValidation(ctx, owner, validation.AssistantID, validation.AssistantVersion, validation.CheckedAt)
}
