package application

import (
	"context"
	"strings"
	"unicode"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

// FAQAnswer resolves the deterministic part of an assistant request. A direct
// FAQ match is returned before any model or retrieval provider is consulted.
func (service *Service) FAQAnswer(ctx context.Context, owner, assistantID, question string) (domain.FAQ, bool, error) {
	if domain.DefaultSafetyPolicy().Decide(question) == domain.SafetyRefuse {
		return domain.FAQ{}, false, nil
	}
	faqs, err := service.repository.ListFAQs(ctx, owner, assistantID)
	if err != nil {
		return domain.FAQ{}, false, err
	}
	normalized := normalizeQuestion(question)
	if normalized == "" {
		return domain.FAQ{}, false, nil
	}
	for _, faq := range faqs {
		if !faq.Enabled {
			continue
		}
		candidate := normalizeQuestion(faq.Question)
		if normalized == candidate || strings.Contains(normalized, candidate) || strings.Contains(candidate, normalized) {
			return faq, true, nil
		}
	}
	return domain.FAQ{}, false, nil
}

func normalizeQuestion(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, strings.ToLower(strings.TrimSpace(value)))
}
