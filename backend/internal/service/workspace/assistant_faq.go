package workspace

import (
	"strings"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

// matchAssistantExactFAQ handles typed FAQ questions without a model call.
// Semantic paraphrases remain the classifier's responsibility; substring and
// keyword matching could mistake a different or extended request for an FAQ.
func matchAssistantExactFAQ(faqs []aiappdomain.FAQ, question string) (aiappdomain.FAQ, bool) {
	normalized := normalizeAssistantFAQQuestion(question)
	if normalized == "" {
		return aiappdomain.FAQ{}, false
	}
	var matched aiappdomain.FAQ
	found := false
	for _, faq := range faqs {
		if !faq.Enabled || normalizeAssistantFAQQuestion(faq.Question) != normalized {
			continue
		}
		if found {
			return aiappdomain.FAQ{}, false
		}
		matched, found = faq, true
	}
	return matched, found
}

func normalizeAssistantFAQQuestion(question string) string {
	question = strings.TrimSpace(question)
	question = strings.TrimRight(question, "?？!！。.")
	return strings.ToLower(strings.Join(strings.Fields(question), " "))
}
