package workspace

import (
	"strings"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

func isAssistantIdentityInquiry(question string) bool {
	switch normalizeAssistantFAQQuestion(question) {
	case "你是谁", "你是什么助手", "介绍一下自己", "介绍一下你自己", "请介绍一下自己", "who are you", "introduce yourself":
		return true
	default:
		return false
	}
}

func assistantIdentityAnswer(assistant aiappdomain.SmartAssistant) string {
	answer := "我是" + assistant.Name + "。"
	if description := strings.TrimSpace(assistant.Description); description != "" {
		answer += "\n\n" + description
	}
	return answer
}
