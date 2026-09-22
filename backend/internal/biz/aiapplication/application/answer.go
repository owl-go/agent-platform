package application

import (
	"context"
	"strings"
	"unicode"

	"agent-platform/backend/internal/biz/aiapplication/domain"
)

const FAQMatchThreshold = 0.65

type FAQMatch struct {
	FAQ        domain.FAQ
	Confidence float32
}

// FAQAnswer resolves the deterministic part of an assistant request. A direct
// FAQ match is returned before any model or retrieval provider is consulted.
func (service *Service) FAQAnswer(ctx context.Context, owner, assistantID, question string) (domain.FAQ, bool, error) {
	match, matched, err := service.MatchFAQ(ctx, owner, assistantID, question)
	return match.FAQ, matched, err
}

// MatchFAQ applies deterministic normalization first and a bounded lexical
// overlap fallback when an exact question is uncertain. It returns a stable
// identity and confidence without rewriting the stored Markdown answer.
func (service *Service) MatchFAQ(ctx context.Context, owner, assistantID, question string) (FAQMatch, bool, error) {
	if domain.DefaultSafetyPolicy().Decide(question) == domain.SafetyRefuse {
		return FAQMatch{}, false, nil
	}
	faqs, err := service.repository.ListFAQs(ctx, owner, assistantID)
	if err != nil {
		return FAQMatch{}, false, err
	}
	normalized := normalizeQuestion(question)
	if normalized == "" {
		return FAQMatch{}, false, nil
	}
	for _, faq := range faqs {
		if !faq.Enabled {
			continue
		}
		candidate := normalizeQuestion(faq.Question)
		if normalized == candidate {
			return FAQMatch{FAQ: faq, Confidence: 1}, true, nil
		}
		if strings.Contains(normalized, candidate) || strings.Contains(candidate, normalized) {
			shorter, longer := len([]rune(normalized)), len([]rune(candidate))
			if shorter > longer {
				shorter, longer = longer, shorter
			}
			confidence := float32(shorter) / float32(longer)
			if confidence >= FAQMatchThreshold {
				return FAQMatch{FAQ: faq, Confidence: confidence}, true, nil
			}
		}
		confidence := lexicalFAQConfidence(normalized, candidate)
		if confidence >= FAQMatchThreshold {
			return FAQMatch{FAQ: faq, Confidence: confidence}, true, nil
		}
	}
	return FAQMatch{}, false, nil
}

func normalizeQuestion(value string) string {
	value = strings.NewReplacer("怎么", "如何", "怎样", "如何").Replace(value)
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return r
	}, strings.ToLower(strings.TrimSpace(value)))
}

func lexicalFAQConfidence(question, candidate string) float32 {
	questionRunes := []rune(question)
	candidateRunes := []rune(candidate)
	if len(candidateRunes) < 2 {
		return 0
	}
	questionSet := make(map[rune]struct{}, len(questionRunes))
	for _, value := range questionRunes {
		questionSet[value] = struct{}{}
	}
	overlap := 0
	seen := make(map[rune]struct{}, len(candidateRunes))
	for _, value := range candidateRunes {
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		if _, exists := questionSet[value]; exists {
			overlap++
		}
	}
	return float32(overlap) / float32(len(seen))
}
