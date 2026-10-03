package workspace

import (
	"strings"

	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

// Match only a complete greeting, so a greeting prefix cannot bypass scope
// classification of an additional request.
func isAssistantGreeting(question string) bool {
	switch normalizeAssistantFAQQuestion(question) {
	case "你好", "您好", "你好呀", "你好啊", "您好呀", "您好啊", "嗨", "哈喽", "早上好", "上午好", "中午好", "下午好", "晚上好", "hello", "hi", "hey", "good morning", "good afternoon", "good evening":
		return true
	default:
		return false
	}
}

func assistantGreetingAnswer(assistant aiappdomain.SmartAssistant) string {
	if welcome := strings.TrimSpace(assistant.Introduction); welcome != "" {
		return welcome
	}
	return "您好！我是" + assistant.Name + "。请问有什么可以帮您？"
}
