package workspace

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
)

type assistantPayload struct {
	Name             string       `json:"name"`
	Icon             string       `json:"icon"`
	Introduction     string       `json:"introduction"`
	Scenario         string       `json:"scenario"`
	ServiceGoal      string       `json:"service_goal"`
	AnswerScope      string       `json:"answer_scope"`
	OperatingRules   string       `json:"operating_rules"`
	ResponseStyle    string       `json:"response_style"`
	KnowledgeBaseIDs []string     `json:"knowledge_base_ids"`
	ExpertID         *string      `json:"expert_id"`
	ExpertTeamID     *string      `json:"expert_team_id"`
	DigitalHumanID   *string      `json:"digital_human_id"`
	Share            sharePayload `json:"share"`
	State            string       `json:"state"`
	Version          int64        `json:"version"`
}

type sharePayload struct {
	Enabled         bool     `json:"enabled"`
	Token           string   `json:"token,omitempty"`
	AllowedOrigins  []string `json:"allowed_origins"`
	Width           string   `json:"width"`
	Height          int      `json:"height"`
	FreeTextEnabled bool     `json:"free_text_enabled"`
	DailyCallLimit  int      `json:"daily_call_limit"`
}

type digitalHumanPayload struct {
	Name             string `json:"name"`
	AvatarObjectKey  string `json:"avatar_object_key"`
	Voice            string `json:"voice"`
	Language         string `json:"language"`
	ExpressionStyle  string `json:"expression_style"`
	SceneDescription string `json:"scene_description"`
	State            string `json:"state"`
	Version          int64  `json:"version"`
}

type faqPayload struct {
	Question       string `json:"question"`
	AnswerMarkdown string `json:"answer_markdown"`
	DisplayOrder   int    `json:"display_order"`
	Category       string `json:"category"`
	Tag            string `json:"tag"`
	Icon           string `json:"icon"`
	Enabled        bool   `json:"enabled"`
	Version        int64  `json:"version"`
}

type knowledgeBasePayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type knowledgeDocumentPayload struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
type embeddingProviderPayload struct {
	Endpoint   string `json:"endpoint"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	Enabled    bool   `json:"enabled"`
	APIKey     string `json:"api_key"`
	Version    int64  `json:"version"`
}

func (service *Service) aiApplicationsHandler(writer http.ResponseWriter, request *http.Request) {
	owner, err := service.owner(request.Context())
	if err != nil {
		writeAuthError(writer, http.StatusUnauthorized, "authentication_required")
		return
	}
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "ai-apps" {
		http.NotFound(writer, request)
		return
	}
	if parts[3] == "assistants" {
		service.handleAssistants(writer, request, owner, parts[4:])
		return
	}
	if parts[3] == "digital-humans" {
		service.handleDigitalHumans(writer, request, owner, parts[4:])
		return
	}
	if parts[3] == "knowledge-bases" {
		service.handleKnowledgeBases(writer, request, owner, parts[4:])
		return
	}
	if parts[3] == "embedding-provider" && len(parts) == 4 {
		if _, adminErr := service.administrator(request.Context()); adminErr != nil {
			writeAuthError(writer, http.StatusForbidden, "administrator_required")
			return
		}
		if request.Method == http.MethodGet {
			value, configErr := service.aiapplications.GetEmbeddingConfiguration(request.Context())
			service.writeAIResult(writer, value, configErr)
			return
		}
		if request.Method == http.MethodPatch {
			var payload embeddingProviderPayload
			if !decodeJSON(writer, request, &payload) {
				return
			}
			value, configErr := service.aiapplications.SaveEmbeddingConfiguration(request.Context(), aiapplicationdomain.EmbeddingConfiguration{Endpoint: payload.Endpoint, Model: payload.Model, Dimensions: payload.Dimensions, Enabled: payload.Enabled, Version: payload.Version}, []byte(payload.APIKey))
			service.writeAIResult(writer, value, configErr)
			return
		}
	}
	http.NotFound(writer, request)
}

func (service *Service) handleKnowledgeBases(writer http.ResponseWriter, request *http.Request, owner string, rest []string) {
	if len(rest) == 0 && request.Method == http.MethodGet {
		value, err := service.aiapplications.ListKnowledgeBases(request.Context(), owner)
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 0 && request.Method == http.MethodPost {
		var payload knowledgeBasePayload
		if !decodeJSON(writer, request, &payload) {
			return
		}
		value, err := service.aiapplications.CreateKnowledgeBase(request.Context(), owner, aiapplicationdomain.KnowledgeBase{Name: payload.Name, Description: payload.Description})
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 2 && rest[1] == "documents" && request.Method == http.MethodGet {
		value, err := service.aiapplications.ListKnowledgeDocuments(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 2 && rest[1] == "documents" && request.Method == http.MethodPost {
		var payload knowledgeDocumentPayload
		if !decodeJSON(writer, request, &payload) {
			return
		}
		value, err := service.aiapplications.CreateKnowledgeDocument(request.Context(), owner, rest[0], aiapplicationdomain.KnowledgeDocument{Name: payload.Name, Content: payload.Content})
		service.writeAIResult(writer, value, err)
		return
	}
	http.NotFound(writer, request)
}

func (service *Service) handleAssistants(writer http.ResponseWriter, request *http.Request, owner string, rest []string) {
	if len(rest) == 0 {
		if request.Method == http.MethodGet {
			value, err := service.aiapplications.ListAssistants(request.Context(), owner)
			service.writeAIResult(writer, value, err)
			return
		}
		if request.Method == http.MethodPost {
			var payload assistantPayload
			if !decodeJSON(writer, request, &payload) {
				return
			}
			value, err := service.aiapplications.CreateAssistant(request.Context(), owner, assistantFromPayload(payload))
			service.writeAIResult(writer, value, err)
			return
		}
	}
	if len(rest) >= 2 && rest[1] == "faqs" {
		service.handleFAQs(writer, request, owner, rest[0], rest[2:])
		return
	}
	if len(rest) == 2 && rest[1] == "answer" {
		service.handleAssistantAnswer(writer, request, owner, rest[0])
		return
	}
	if len(rest) == 2 && rest[1] == "share-token" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			Version int64 `json:"version"`
		}
		if !decodeJSON(writer, request, &input) {
			return
		}
		value, err := service.aiapplications.RegenerateShareToken(request.Context(), owner, rest[0], input.Version)
		service.writeAIResult(writer, map[string]any{"assistant": value, "token": value.Share.Token}, err)
		return
	}
	if len(rest) == 2 && rest[1] == "sessions" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		assistant, err := service.aiapplications.GetAssistant(request.Context(), owner, rest[0])
		if err != nil {
			service.writeAIResult(writer, nil, err)
			return
		}
		session, err := service.workspace.Repository().CreateSession(request.Context(), owner, assistant.ExpertID, assistant.ExpertTeamID)
		if err == nil {
			err = service.aiapplications.BindAssistantSession(request.Context(), owner, rest[0], session.ID)
		}
		if err != nil {
			if session.ID != "" {
				_ = service.workspace.Repository().DeleteSession(request.Context(), owner, session.ID)
			}
			service.writeAIResult(writer, nil, err)
			return
		}
		service.writeAIResult(writer, map[string]any{"id": session.ID, "title": session.Title, "expert_id": session.ExpertID, "expert_team_id": session.ExpertTeamID, "assistant_id": rest[0], "assistant_welcome": assistant.Introduction, "created_at": session.CreatedAt, "updated_at": session.UpdatedAt, "version": session.Version}, nil)
		return
	}
	if len(rest) == 2 && rest[1] == "copy" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		value, err := service.aiapplications.CopyAssistant(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 2 && rest[1] == "state" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			State   aiapplicationdomain.ApplicationState `json:"state"`
			Version int64                                `json:"version"`
		}
		if !decodeJSON(writer, request, &input) {
			return
		}
		value, err := service.aiapplications.SetAssistantState(request.Context(), owner, rest[0], input.State, input.Version)
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) != 1 {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodGet:
		value, err := service.aiapplications.GetAssistant(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
	case http.MethodPatch:
		var payload assistantPayload
		if !decodeJSON(writer, request, &payload) {
			return
		}
		value, err := service.aiapplications.UpdateAssistant(request.Context(), owner, rest[0], assistantFromPayload(payload), payload.Version)
		service.writeAIResult(writer, value, err)
	case http.MethodDelete:
		service.writeAIResult(writer, nil, service.aiapplications.DeleteAssistant(request.Context(), owner, rest[0]))
	default:
		writer.Header().Set("Allow", "GET, PATCH, DELETE")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (service *Service) handleAssistantAnswer(writer http.ResponseWriter, request *http.Request, owner, assistantID string) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		Question string `json:"question"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	decision := aiapplicationdomain.DefaultSafetyPolicy().Decide(input.Question)
	_ = service.aiapplications.RecordSafetyAudit(request.Context(), owner, assistantID, "authenticated", decision, "not_charged")
	if decision == aiapplicationdomain.SafetyRefuse {
		service.writeAIResult(writer, map[string]string{"kind": "refusal", "answer": aiapplicationdomain.SafetyRefusal}, nil)
		return
	}
	match, matched, err := service.aiapplications.MatchFAQ(request.Context(), owner, assistantID, input.Question)
	if err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	if matched {
		service.writeAIResult(writer, map[string]any{"kind": "faq", "answer_markdown": match.FAQ.AnswerMarkdown, "faq_id": match.FAQ.ID, "confidence": match.Confidence}, nil)
		return
	}
	assistant, assistantErr := service.aiapplications.GetAssistant(request.Context(), owner, assistantID)
	if assistantErr != nil {
		service.writeAIResult(writer, nil, assistantErr)
		return
	}
	if err := service.credits.RequirePositiveBalance(request.Context(), owner, service.userTimezone(request.Context(), owner)); err != nil {
		service.writeAIResult(writer, nil, err)
		return
	}
	session, sessionErr := service.workspace.Repository().CreateSession(request.Context(), owner, assistant.ExpertID, assistant.ExpertTeamID)
	if sessionErr == nil {
		sessionErr = service.aiapplications.BindAssistantSession(request.Context(), owner, assistantID, session.ID)
	}
	if sessionErr == nil {
		_, assistantMessage, messageErr := service.workspace.Repository().CreateMessagePair(request.Context(), owner, session.ID, input.Question, nil)
		if messageErr != nil {
			sessionErr = messageErr
		} else {
			service.writeAIResult(writer, map[string]any{"kind": "generating", "session_id": session.ID, "message_id": assistantMessage.ID, "state": assistantMessage.State}, nil)
			return
		}
	}
	if session.ID != "" {
		_ = service.workspace.Repository().DeleteSession(request.Context(), owner, session.ID)
	}
	service.writeAIResult(writer, nil, sessionErr)
}

func (service *Service) handleFAQs(writer http.ResponseWriter, request *http.Request, owner, assistantID string, rest []string) {
	if len(rest) == 0 {
		if request.Method == http.MethodGet {
			value, err := service.aiapplications.ListFAQs(request.Context(), owner, assistantID)
			service.writeAIResult(writer, value, err)
			return
		}
		if request.Method == http.MethodPost {
			var payload faqPayload
			if !decodeJSON(writer, request, &payload) {
				return
			}
			value, err := service.aiapplications.CreateFAQ(request.Context(), owner, assistantID, faqFromPayload(payload))
			service.writeAIResult(writer, value, err)
			return
		}
	}
	if len(rest) != 1 {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodPatch:
		var payload faqPayload
		if !decodeJSON(writer, request, &payload) {
			return
		}
		value, err := service.aiapplications.UpdateFAQ(request.Context(), owner, assistantID, rest[0], faqFromPayload(payload), payload.Version)
		service.writeAIResult(writer, value, err)
	case http.MethodDelete:
		service.writeAIResult(writer, nil, service.aiapplications.DeleteFAQ(request.Context(), owner, assistantID, rest[0]))
	default:
		writer.Header().Set("Allow", "PATCH, DELETE")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (service *Service) handleDigitalHumans(writer http.ResponseWriter, request *http.Request, owner string, rest []string) {
	if len(rest) == 0 {
		if request.Method == http.MethodGet {
			value, err := service.aiapplications.ListDigitalHumans(request.Context(), owner)
			service.writeAIResult(writer, value, err)
			return
		}
		if request.Method == http.MethodPost {
			var payload digitalHumanPayload
			if !decodeJSON(writer, request, &payload) {
				return
			}
			value, err := service.aiapplications.CreateDigitalHuman(request.Context(), owner, humanFromPayload(payload))
			service.writeAIResult(writer, value, err)
			return
		}
	}
	if len(rest) == 2 && rest[1] == "copy" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		value, err := service.aiapplications.CopyDigitalHuman(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 2 && rest[1] == "preview" {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		value, err := service.aiapplications.PreviewDigitalHuman(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) == 2 && rest[1] == "state" {
		if request.Method != http.MethodPost {
			writer.Header().Set("Allow", http.MethodPost)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var input struct {
			State   aiapplicationdomain.ApplicationState `json:"state"`
			Version int64                                `json:"version"`
		}
		if !decodeJSON(writer, request, &input) {
			return
		}
		value, err := service.aiapplications.SetDigitalHumanState(request.Context(), owner, rest[0], input.State, input.Version)
		service.writeAIResult(writer, value, err)
		return
	}
	if len(rest) != 1 {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodGet:
		value, err := service.aiapplications.GetDigitalHuman(request.Context(), owner, rest[0])
		service.writeAIResult(writer, value, err)
	case http.MethodPatch:
		var payload digitalHumanPayload
		if !decodeJSON(writer, request, &payload) {
			return
		}
		value, err := service.aiapplications.UpdateDigitalHuman(request.Context(), owner, rest[0], humanFromPayload(payload), payload.Version)
		service.writeAIResult(writer, value, err)
	case http.MethodDelete:
		detach := request.URL.Query().Get("detach") == "true"
		service.writeAIResult(writer, nil, service.aiapplications.DeleteDigitalHumanWithOptions(request.Context(), owner, rest[0], detach))
	default:
		writer.Header().Set("Allow", "GET, PATCH, DELETE")
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeAuthError(writer, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func (service *Service) writeAIResult(writer http.ResponseWriter, value any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		code := "request_failed"
		switch {
		case errors.Is(err, aiapplicationdomain.ErrNotFound):
			status, code = http.StatusNotFound, "resource_not_found"
		case errors.Is(err, aiapplicationdomain.ErrInvalid):
			status, code = http.StatusUnprocessableEntity, "invalid_input"
		case errors.Is(err, aiapplicationdomain.ErrConflict):
			status, code = http.StatusConflict, "resource_conflict"
		case errors.Is(err, aiapplicationdomain.ErrVersionConflict):
			status, code = http.StatusPreconditionFailed, "version_conflict"
		}
		writeAuthError(writer, status, code)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	if value == nil {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	_ = json.NewEncoder(writer).Encode(value)
}

func assistantFromPayload(value assistantPayload) aiapplicationdomain.SmartAssistant {
	return aiapplicationdomain.SmartAssistant{Name: value.Name, Icon: value.Icon, Introduction: value.Introduction, Scenario: value.Scenario, ServiceGoal: value.ServiceGoal, AnswerScope: value.AnswerScope, OperatingRules: value.OperatingRules, ResponseStyle: value.ResponseStyle, KnowledgeBaseIDs: value.KnowledgeBaseIDs, ExpertID: value.ExpertID, ExpertTeamID: value.ExpertTeamID, DigitalHumanID: value.DigitalHumanID, State: aiapplicationdomain.ApplicationState(value.State), Share: aiapplicationdomain.ShareConfiguration{Enabled: value.Share.Enabled, Token: value.Share.Token, AllowedOrigins: value.Share.AllowedOrigins, Width: value.Share.Width, Height: value.Share.Height, FreeTextEnabled: value.Share.FreeTextEnabled, DailyCallLimit: value.Share.DailyCallLimit}}
}
func humanFromPayload(value digitalHumanPayload) aiapplicationdomain.DigitalHuman {
	return aiapplicationdomain.DigitalHuman{Name: value.Name, AvatarObjectKey: value.AvatarObjectKey, Voice: value.Voice, Language: value.Language, ExpressionStyle: value.ExpressionStyle, SceneDescription: value.SceneDescription, State: aiapplicationdomain.ApplicationState(value.State)}
}
func faqFromPayload(value faqPayload) aiapplicationdomain.FAQ {
	return aiapplicationdomain.FAQ{Question: value.Question, AnswerMarkdown: value.AnswerMarkdown, DisplayOrder: value.DisplayOrder, Category: value.Category, Tag: value.Tag, Icon: value.Icon, Enabled: value.Enabled}
}
