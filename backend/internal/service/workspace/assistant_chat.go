package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aiapp "agent-platform/backend/internal/biz/aiapplication/application"
	aiappdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	creditsapp "agent-platform/backend/internal/biz/credits/application"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

const assistantScopeRefusal = "对不起，我暂时无法回答此类问题"

type assistantAnswer struct {
	text, source, faqID       string
	inputTokens, outputTokens int64
}

func (service *Service) resolveAssistantModel(ctx context.Context, owner, modelID string) (aiappdomain.AssistantModel, error) {
	if modelID == "" {
		// Assistants created before model selection was added retain their
		// Personal Settings fallback until an explicit model is saved.
		settings, err := service.workspace.Repository().GetSettings(ctx, owner)
		if err != nil {
			return aiappdomain.AssistantModel{}, err
		}
		modelID = settings.RuntimeModelDefaults[settings.DefaultRuntimeEngine]
	}
	if modelID == "" {
		return aiappdomain.AssistantModel{}, fmt.Errorf("%w: select an openai_chat Provider Model", aiappdomain.ErrInvalid)
	}
	connections, err := service.workspace.Repository().ListModelProviderConnections(ctx)
	if err != nil {
		return aiappdomain.AssistantModel{}, err
	}
	return selectAssistantModel(connections, modelID)
}

func (service *Service) resolveAssistantTurnModel(ctx context.Context, owner string, conversation aiappdomain.AssistantConversation) (aiappdomain.AssistantModel, error) {
	modelID := conversation.ModelSnapshot.ProviderModelID
	if modelID == "" {
		modelID = conversation.AssistantSnapshot.ProviderModelID
	}
	return service.resolveAssistantModel(ctx, owner, modelID)
}

func selectAssistantModel(connections []workspacedomain.ModelProviderConnection, modelID string) (aiappdomain.AssistantModel, error) {
	for _, connection := range connections {
		for _, model := range connection.Models {
			if model.ID != modelID {
				continue
			}
			if model.Available && connection.HasAPIKey {
				for _, protocol := range connection.Protocols {
					if protocol == "openai_chat" {
						return aiappdomain.AssistantModel{ProviderModelID: model.ID, ConnectionID: connection.ID, CredentialOwnerID: connection.CredentialOwnerID, ProviderType: connection.ProviderType, Protocol: "openai_chat", ModelID: model.ModelID, Endpoint: connection.Endpoint, ConnectionVersion: connection.Version}, nil
					}
				}
			}
			return aiappdomain.AssistantModel{}, fmt.Errorf("%w: selected Provider Model must be available with an API key and openai_chat protocol", aiappdomain.ErrInvalid)
		}
	}
	return aiappdomain.AssistantModel{}, fmt.Errorf("%w: selected Provider Model is unavailable", aiappdomain.ErrInvalid)
}

func (service *Service) runAssistantModel(ctx context.Context, owner, turnID string, stage int, model aiappdomain.AssistantModel, messages []aiapp.ChatMessage, stream bool, onDelta func(string) error) (aiapp.ChatResult, error) {
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return aiapp.ChatResult{}, err
	}
	admission, err := service.credits.Admit(ctx, creditsapp.AdmissionRequest{UserID: owner, ExecutionID: "assistant-turn-" + turnID, StagePosition: stage, Timezone: settings.Timezone, ProviderType: model.ProviderType, Protocol: model.Protocol, ModelID: model.ModelID})
	if err != nil {
		return aiapp.ChatResult{}, err
	}
	ciphertext, err := service.workspace.Repository().GetModelProviderAPIKey(ctx, model.CredentialOwnerID, model.ConnectionID)
	if err != nil {
		_ = service.credits.Abort(context.WithoutCancel(ctx), admission)
		return aiapp.ChatResult{}, err
	}
	key, err := service.box.Decrypt(ciphertext, "model-provider:"+model.CredentialOwnerID)
	if err != nil {
		_ = service.credits.Abort(context.WithoutCancel(ctx), admission)
		return aiapp.ChatResult{}, err
	}
	result, callErr := service.assistantChatModel.Generate(ctx, aiapp.ChatRequest{Endpoint: model.Endpoint, Protocol: model.Protocol, ModelID: model.ModelID, APIKey: key, Messages: messages, Stream: stream}, onDelta)
	clear(key)
	settlementCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if callErr != nil && result.Text == "" {
		if abortErr := service.credits.Abort(settlementCtx, admission); abortErr != nil {
			return result, fmt.Errorf("Assistant model failed and Credit reservation could not be released: %w", abortErr)
		}
		return result, callErr
	}
	_, settleErr := service.credits.Settle(settlementCtx, creditsapp.SettlementRequest{Admission: admission, Usage: creditsdomain.Usage{InputTokens: result.InputTokens, OutputTokens: result.OutputTokens, Known: result.UsageKnown}})
	if settleErr != nil {
		return result, fmt.Errorf("settle Assistant model Credits: %w", settleErr)
	}
	return result, callErr
}

func (service *Service) answerAssistantTurn(ctx context.Context, owner string, conversation aiappdomain.AssistantConversation, turn aiappdomain.AssistantTurn, requestedFAQID, auditSource string, emit func(string) error) (assistantAnswer, error) {
	result := assistantAnswer{}
	assistant := conversation.AssistantSnapshot
	if aiappdomain.DefaultSafetyPolicy().Decide(turn.Question) == aiappdomain.SafetyRefuse {
		_ = service.aiapplications.RecordSafetyAudit(ctx, owner, assistant.ID, auditSource, aiappdomain.SafetyRefuse, "not_charged")
		result.text, result.source = aiappdomain.SafetyRefusal, "safety"
		return result, nil
	}
	_ = service.aiapplications.RecordSafetyAudit(ctx, owner, assistant.ID, auditSource, aiappdomain.SafetyAllow, "pending")
	faqs, err := service.aiapplications.ListFAQs(ctx, owner, assistant.ID)
	if err != nil {
		return result, err
	}
	enabled := make([]aiappdomain.FAQ, 0, len(faqs))
	for _, faq := range faqs {
		if faq.Enabled {
			enabled = append(enabled, faq)
		}
	}
	if requestedFAQID != "" {
		for _, faq := range enabled {
			if faq.ID == requestedFAQID && faq.Question == turn.Question {
				result.text, result.source, result.faqID = faq.AnswerMarkdown, "faq", faq.ID
				return result, nil
			}
		}
		return result, fmt.Errorf("%w: FAQ is unavailable", aiappdomain.ErrInvalid)
	}
	faqChoices := make([]map[string]string, 0, len(enabled))
	for _, faq := range enabled {
		faqChoices = append(faqChoices, map[string]string{"id": faq.ID, "question": faq.Question})
	}
	choices, _ := json.Marshal(faqChoices)
	model, err := service.resolveAssistantTurnModel(ctx, owner, conversation)
	if err != nil {
		return result, err
	}
	preprocessInstruction := "判断用户的问题是否在此智能助手的服务范围，或是否等价于一条常见问题。只返回 JSON：{\"decision\":\"faq|out_of_scope|continue\",\"faq_id\":\"\",\"question\":\"整理后的问题\"}。faq_id 只能来自提供的列表；没有充分依据就选择 continue。不得把范围外问题判为 FAQ。"
	if assistant.PreprocessPrompt != "" {
		preprocessInstruction += "\n用户配置的预处理提示词：" + assistant.PreprocessPrompt
	}
	preprocess := []aiapp.ChatMessage{{Role: "system", Content: preprocessInstruction}, {Role: "user", Content: "助手简介：" + assistant.Description + "\n助手提示词：" + assistant.Prompt + "\n常见问题：" + string(choices) + "\n用户问题：" + turn.Question}}
	classified, err := service.runAssistantModel(ctx, owner, turn.ID, 1, model, preprocess, false, nil)
	result.inputTokens += classified.InputTokens
	result.outputTokens += classified.OutputTokens
	if err != nil {
		return result, err
	}
	var decision struct {
		Decision string `json:"decision"`
		FAQID    string `json:"faq_id"`
		Question string `json:"question"`
	}
	classification := strings.TrimSpace(classified.Text)
	classification = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(classification, "```json"), "```"), "```")
	if err := json.Unmarshal([]byte(strings.TrimSpace(classification)), &decision); err != nil {
		return result, fmt.Errorf("Assistant preprocessing returned invalid classification: %w", err)
	}
	switch decision.Decision {
	case "faq":
		for _, faq := range enabled {
			if faq.ID == decision.FAQID {
				result.text, result.source, result.faqID = faq.AnswerMarkdown, "faq", faq.ID
				return result, nil
			}
		}
		return result, fmt.Errorf("Assistant preprocessing returned an unknown FAQ")
	case "out_of_scope":
		result.text, result.source = assistantScopeRefusal, "scope"
		return result, nil
	case "continue":
	default:
		return result, fmt.Errorf("Assistant preprocessing returned an unknown decision")
	}
	question := strings.TrimSpace(decision.Question)
	if question == "" || len([]rune(question)) > 4000 {
		question = turn.Question
	}
	knowledge, grounded, err := service.retrieveAssistantKnowledge(ctx, owner, assistant.KnowledgeBaseIDs, question)
	if err != nil {
		return result, err
	}
	result.source = "model"
	if grounded {
		result.source = "knowledge"
	}
	history, err := service.aiapplications.ListAssistantTurns(ctx, owner, conversation.ID)
	if err != nil {
		return result, err
	}
	contextMessages, summaryUsage, err := service.assistantHistory(ctx, owner, conversation, model, turn.ID, history)
	result.inputTokens += summaryUsage.InputTokens
	result.outputTokens += summaryUsage.OutputTokens
	if err != nil {
		return result, err
	}
	system := "你是智能助手“" + conversation.AssistantName + "”。遵循以下助手提示词：\n" + assistant.Prompt + "\n回答风格：" + assistant.ResponseStyle
	if knowledge != "" {
		system += "\n只在相关时使用以下知识库结果；若与问题不符可忽略：" + knowledge
	}
	messages := append([]aiapp.ChatMessage{{Role: "system", Content: system}}, contextMessages...)
	messages = append(messages, aiapp.ChatMessage{Role: "user", Content: question})
	generated, generateErr := service.runAssistantModel(ctx, owner, turn.ID, 3, model, messages, true, func(delta string) error {
		result.text += delta
		if err := service.aiapplications.SaveAssistantTurnProgress(ctx, owner, conversation.ID, turn.ID, result.text); err != nil {
			return err
		}
		return emit(delta)
	})
	result.inputTokens += generated.InputTokens
	result.outputTokens += generated.OutputTokens
	if generated.Text != "" {
		result.text = generated.Text
	}
	return result, generateErr
}

// retrieveAssistantKnowledge shares the same provider-neutral retrieval and
// active revision verification used by Knowledge Base search and Workflow Runs.
func (service *Service) retrieveAssistantKnowledge(ctx context.Context, owner string, baseIDs []string, question string) (string, bool, error) {
	if len(baseIDs) == 0 {
		return "", false, nil
	}
	if service.knowledgeSearch == nil {
		return "", false, fmt.Errorf("Knowledge retrieval is configured but no retrieval provider is available")
	}
	var excerpts []string
	seen := make(map[string]struct{})
	for _, baseID := range baseIDs {
		remaining := 5 - len(excerpts)
		if remaining <= 0 {
			break
		}
		hits, err := service.knowledgeSearch.Search(ctx, owner, baseID, 0, question, remaining, 6000)
		if err != nil {
			return "", false, fmt.Errorf("retrieve Knowledge Base %s: %w", baseID, err)
		}
		for _, hit := range hits {
			text := truncateRunes(strings.TrimSpace(hit.Text), 1300)
			if text == "" {
				continue
			}
			key := hit.Source.RevisionID + "\x00" + text
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			source := hit.Source.DocumentName
			if hit.Source.CategoryName != "" {
				source = hit.Source.CategoryName + "/" + source
			}
			excerpts = append(excerpts, "[来源 "+source+"，修订 "+hit.Source.RevisionID+"] "+text)
		}
	}
	if len(excerpts) == 0 {
		return "", false, nil
	}
	return "\n" + strings.Join(excerpts, "\n"), true, nil
}

func (service *Service) assistantHistory(ctx context.Context, owner string, conversation aiappdomain.AssistantConversation, model aiappdomain.AssistantModel, turnID string, turns []aiappdomain.AssistantTurn) ([]aiapp.ChatMessage, aiapp.ChatResult, error) {
	completed := make([]aiappdomain.AssistantTurn, 0, 10)
	for _, turn := range turns {
		if turn.State == "completed" && turn.TurnNumber > conversation.SummaryThroughTurn {
			completed = append(completed, turn)
		}
	}
	if len(completed) > 10 {
		completed = completed[len(completed)-10:]
	}
	summary := conversation.Summary
	var usage aiapp.ChatResult
	length := len([]rune(summary))
	for _, turn := range completed {
		length += len([]rune(turn.Question)) + len([]rune(turn.Answer))
	}
	if length > 12000 && len(completed) > 2 {
		older := completed[:len(completed)-2]
		material := "现有摘要：" + truncateRunes(summary, 2000)
		for _, turn := range older {
			material += "\n用户：" + truncateRunes(turn.Question, 800) + "\n助手：" + truncateRunes(turn.Answer, 1200)
		}
		compressed, err := service.runAssistantModel(ctx, owner, turnID, 2, model, []aiapp.ChatMessage{{Role: "system", Content: "把对话信息压缩为不超过1000字的事实摘要，保留用户偏好和未解决问题，不添加未出现的事实。"}, {Role: "user", Content: material}}, false, nil)
		usage = compressed
		if err != nil {
			return nil, usage, err
		}
		summary = truncateRunes(compressed.Text, 2000)
		if err := service.aiapplications.SaveAssistantSummary(ctx, owner, conversation.ID, older[len(older)-1].TurnNumber, summary); err != nil {
			return nil, usage, err
		}
		completed = completed[len(completed)-2:]
	}
	messages := make([]aiapp.ChatMessage, 0, 1+len(completed)*2)
	if summary != "" {
		messages = append(messages, aiapp.ChatMessage{Role: "system", Content: "此前对话摘要：" + summary})
	}
	for _, turn := range completed {
		messages = append(messages, aiapp.ChatMessage{Role: "user", Content: turn.Question}, aiapp.ChatMessage{Role: "assistant", Content: turn.Answer})
	}
	return messages, usage, nil
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func assistantTurnState(err error) string {
	if err == nil {
		return "completed"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "failed"
}
