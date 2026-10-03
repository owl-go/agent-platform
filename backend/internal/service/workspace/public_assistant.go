package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

const visitorCookieName = "agent_workspace_visitor"

const (
	publicShareTokenRateLimit = 300
	publicVisitorRateLimit    = 60
	publicIPRateLimit         = 120
)

type externalWorkspaceRepository interface {
	CreateExternalSession(context.Context, string, *string, *string) (workspacedomain.Session, error)
	DeleteExternalSession(context.Context, string, string) error
}

func (service *Service) publicAssistantHandler(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "public" || parts[3] != "assistants" {
		http.NotFound(writer, request)
		return
	}
	token := parts[4]
	assistant, err := service.aiapplications.ResolveSharedAssistant(request.Context(), token)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	if !publicRequestOriginAllowed(request, assistant.Share.AllowedOrigins) {
		writeAuthError(writer, http.StatusForbidden, "origin_not_allowed")
		return
	}
	if origin := request.Header.Get("Origin"); origin != "" {
		writer.Header().Set("Access-Control-Allow-Origin", origin)
		writer.Header().Set("Access-Control-Allow-Credentials", "true")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		writer.Header().Add("Vary", "Origin")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
	}
	visitorHash, visitorCookie, visitorErr := publicVisitor(request)
	if visitorErr != nil {
		writeAuthError(writer, http.StatusInternalServerError, "request_failed")
		return
	}
	if visitorCookie != nil {
		http.SetCookie(writer, visitorCookie)
	}
	if len(parts) == 6 && parts[5] == "turns" && request.Method == http.MethodPost {
		service.streamPublicAssistantTurn(writer, request, assistant, token, visitorHash)
		return
	}
	if len(parts) == 6 && parts[5] == "icon" && request.Method == http.MethodGet {
		service.downloadAssistantIcon(writer, request, assistant.OwnerID, assistant.ID)
		return
	}
	if len(parts) == 9 && parts[5] == "conversations" && parts[7] == "responses" && request.Method == http.MethodGet {
		direct, directErr := service.directPublicResponse(request.Context(), assistant, visitorHash, parts[6], parts[8])
		if directErr == nil {
			writePublicJSON(writer, direct, http.StatusOK)
			return
		}
		if !errors.Is(directErr, aiapplicationdomain.ErrNotFound) {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		legacy, legacyErr := service.aiapplications.GetExternalConversation(request.Context(), parts[6], assistant.ID, visitorHash)
		if legacyErr != nil || legacy.ShareTokenRevision != assistant.Share.TokenRevision {
			http.NotFound(writer, request)
			return
		}
		response, responseErr := service.aiapplications.GetExternalResponse(request.Context(), parts[8], parts[6], visitorHash)
		if responseErr != nil {
			http.NotFound(writer, request)
			return
		}
		writePublicJSON(writer, map[string]any{"kind": responseKind(response.State), "response_id": response.ID, "conversation_id": response.ConversationID, "state": response.State, "answer": response.Content, "error": response.Error}, http.StatusOK)
		return
	}
	if len(parts) == 5 && request.Method == http.MethodGet {
		faqs, faqErr := service.aiapplications.ListFAQs(request.Context(), assistant.OwnerID, assistant.ID)
		if faqErr != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		public := make([]map[string]any, 0, len(faqs))
		for _, faq := range faqs {
			if faq.Enabled {
				public = append(public, map[string]any{"id": faq.ID, "question": faq.Question, "answer_markdown": faq.AnswerMarkdown, "category": faq.Category, "tag": faq.Tag, "icon": faq.Icon})
			}
		}
		writePublicJSON(writer, map[string]any{"id": assistant.ID, "name": assistant.Name, "introduction": assistant.Introduction, "scenario": assistant.Scenario, "faqs": public, "free_text_enabled": assistant.Share.FreeTextEnabled, "width": assistant.Share.Width, "height": assistant.Share.Height}, http.StatusOK)
		return
	}
	if len(parts) == 6 && parts[5] == "answer" && request.Method == http.MethodPost {
		if !assistant.Share.FreeTextEnabled {
			writePublicJSON(writer, map[string]string{"kind": "unavailable", "answer": "暂时无法回答此类问题"}, http.StatusForbidden)
			return
		}
		var input struct {
			Question       string `json:"question"`
			ConversationID string `json:"conversation_id"`
		}
		if json.NewDecoder(request.Body).Decode(&input) != nil {
			writeAuthError(writer, http.StatusBadRequest, "invalid_json")
			return
		}
		decision := aiapplicationdomain.DefaultSafetyPolicy().Decide(input.Question)
		if decision == aiapplicationdomain.SafetyRefuse {
			_ = service.aiapplications.RecordSafetyAudit(request.Context(), assistant.OwnerID, assistant.ID, "public", decision, "not_charged")
			writePublicJSON(writer, map[string]string{"kind": "refusal", "answer": aiapplicationdomain.SafetyRefusal}, http.StatusOK)
			return
		}
		if !service.consumePublicRate(writer, request, token, visitorHash) {
			return
		}
		match, matched, faqErr := service.aiapplications.MatchFAQ(request.Context(), assistant.OwnerID, assistant.ID, input.Question)
		if faqErr != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		if matched {
			_ = service.aiapplications.RecordSafetyAudit(request.Context(), assistant.OwnerID, assistant.ID, "public", aiapplicationdomain.SafetyAllow, "not_charged")
			writePublicJSON(writer, map[string]any{"kind": "faq", "answer_markdown": match.FAQ.AnswerMarkdown, "faq_id": match.FAQ.ID, "confidence": match.Confidence}, http.StatusOK)
			return
		}
		allowed, usageErr := service.aiapplications.ConsumeSharedAssistantCall(request.Context(), assistant.ID, assistant.Share.DailyCallLimit)
		if usageErr != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		if !allowed {
			writeAuthError(writer, http.StatusTooManyRequests, "daily_call_limit_exceeded")
			return
		}
		response, createErr := service.publicAnswer(request.Context(), assistant, visitorHash, input.ConversationID, input.Question)
		if createErr != nil {
			if errors.Is(createErr, aiapplicationdomain.ErrInvalid) {
				writeAuthError(writer, http.StatusUnprocessableEntity, "assistant_model_unavailable")
				return
			}
			if errors.Is(createErr, aiapplicationdomain.ErrNotFound) {
				http.NotFound(writer, request)
				return
			}
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		writePublicJSON(writer, response, http.StatusAccepted)
		return
	}
	http.NotFound(writer, request)
}

func (service *Service) consumePublicRate(writer http.ResponseWriter, request *http.Request, shareToken, visitorHash string) bool {
	remoteHash := publicRemoteHash(request)
	now := time.Now()
	for _, rate := range []struct {
		scope string
		key   string
		limit int
	}{
		{scope: "share_token", key: publicHash(shareToken), limit: publicShareTokenRateLimit},
		{scope: "visitor", key: visitorHash, limit: publicVisitorRateLimit},
		{scope: "ip", key: remoteHash, limit: publicIPRateLimit},
	} {
		allowed, err := service.aiapplications.ConsumeExternalRate(request.Context(), rate.scope, rate.key, now, rate.limit)
		if err != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return false
		}
		if !allowed {
			writeAuthError(writer, http.StatusTooManyRequests, "rate_limit_exceeded")
			return false
		}
	}
	return true
}

func publicHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func publicRemoteHash(request *http.Request) string {
	remote := request.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	return publicHash(remote)
}

func (service *Service) enqueueExternalAnswer(ctx context.Context, assistant aiapplicationdomain.SmartAssistant, visitorHash, conversationID, question string) (aiapplicationdomain.ExternalResponse, error) {
	repository := service.workspace.Repository()
	externalRepository, ok := repository.(externalWorkspaceRepository)
	if !ok {
		return aiapplicationdomain.ExternalResponse{}, fmt.Errorf("external session repository is unavailable")
	}
	var conversation aiapplicationdomain.ExternalConversation
	if conversationID != "" {
		var err error
		conversation, err = service.aiapplications.GetExternalConversation(ctx, conversationID, assistant.ID, visitorHash)
		if err != nil {
			return aiapplicationdomain.ExternalResponse{}, err
		}
	} else {
		session, err := externalRepository.CreateExternalSession(ctx, assistant.OwnerID, assistant.ExpertID, assistant.ExpertTeamID)
		if err != nil {
			return aiapplicationdomain.ExternalResponse{}, err
		}
		if err := service.aiapplications.BindAssistantSession(ctx, assistant.OwnerID, assistant.ID, session.ID); err != nil {
			_ = externalRepository.DeleteExternalSession(ctx, assistant.OwnerID, session.ID)
			return aiapplicationdomain.ExternalResponse{}, err
		}
		snapshot, err := json.Marshal(assistant)
		if err != nil {
			_ = externalRepository.DeleteExternalSession(ctx, assistant.OwnerID, session.ID)
			return aiapplicationdomain.ExternalResponse{}, err
		}
		conversation, err = service.aiapplications.CreateExternalConversation(ctx, aiapplicationdomain.ExternalConversation{AssistantID: assistant.ID, OwnerID: assistant.OwnerID, ExecutionSessionID: session.ID, VisitorHash: visitorHash, AssistantSnapshot: snapshot, ShareTokenRevision: assistant.Share.TokenRevision})
		if err != nil {
			_ = externalRepository.DeleteExternalSession(ctx, assistant.OwnerID, session.ID)
			return aiapplicationdomain.ExternalResponse{}, err
		}
	}
	_, assistantMessage, err := repository.CreateMessagePair(ctx, assistant.OwnerID, conversation.ExecutionSessionID, question, nil)
	if err != nil {
		return aiapplicationdomain.ExternalResponse{}, err
	}
	return service.aiapplications.CreateExternalResponse(ctx, aiapplicationdomain.ExternalResponse{ConversationID: conversation.ID, AssistantMessageID: assistantMessage.ID, State: assistantMessage.State, Content: assistantMessage.Content, Error: assistantMessage.Error})
}

func responseKind(state string) string {
	if state == "completed" {
		return "answer"
	}
	if state == "failed" || state == "cancelled" {
		return "error"
	}
	return "generating"
}

func publicVisitor(request *http.Request) (string, *http.Cookie, error) {
	visitor := ""
	if cookie, err := request.Cookie(visitorCookieName); err == nil {
		visitor = cookie.Value
	}
	var cookie *http.Cookie
	if visitor == "" {
		value := make([]byte, 24)
		if _, err := rand.Read(value); err != nil {
			return "", nil, err
		}
		visitor = base64.RawURLEncoding.EncodeToString(value)
		secure := request.TLS != nil
		sameSite := http.SameSiteLaxMode
		if secure {
			sameSite = http.SameSiteNoneMode
		}
		cookie = &http.Cookie{Name: visitorCookieName, Value: visitor, Path: "/", HttpOnly: true, SameSite: sameSite, Secure: secure, MaxAge: 86400}
	}
	digest := sha256.Sum256([]byte(visitor))
	return base64.RawURLEncoding.EncodeToString(digest[:]), cookie, nil
}

func (service *Service) publicAssistantEmbed(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "embed" || parts[1] != "assistant" || request.Method != http.MethodGet {
		http.NotFound(writer, request)
		return
	}
	token := parts[2]
	assistant, err := service.aiapplications.ResolveSharedAssistant(request.Context(), token)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	if !publicRequestOriginAllowed(request, assistant.Share.AllowedOrigins) {
		writeAuthError(writer, http.StatusForbidden, "origin_not_allowed")
		return
	}
	frameAncestors := "*"
	if len(assistant.Share.AllowedOrigins) > 0 {
		frameAncestors = strings.Join(assistant.Share.AllowedOrigins, " ")
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'; img-src 'self'; connect-src 'self'; frame-ancestors "+frameAncestors)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "no-store")
	name := html.EscapeString(assistant.Name)
	intro := html.EscapeString(assistant.Introduction)
	publicURL := "/api/v1/public/assistants/" + html.EscapeString(token)
	writer.Write([]byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + name + `</title></head><body><div id="app" data-assistant-api="` + publicURL + `" data-width="` + html.EscapeString(assistant.Share.Width) + `" data-height="` + fmt.Sprint(assistant.Share.Height) + `"><noscript>` + intro + `</noscript></div><script type="module" src="/assets/assistant-embed.js"></script></body></html>`))
}

func publicOriginAllowed(origin string, allowed []string) bool {
	if origin == "" || len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if strings.TrimRight(strings.TrimSpace(candidate), "/") == strings.TrimRight(origin, "/") {
			return true
		}
	}
	return false
}

func publicRequestOriginAllowed(request *http.Request, allowed []string) bool {
	// An embedded page fetches from its own platform origin, not its parent's
	// origin. The configured CSP frame-ancestors still restricts the parent.
	origin := request.Header.Get("Origin")
	parsed, err := url.Parse(origin)
	if err == nil && parsed.User == nil && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Host != "" && strings.EqualFold(parsed.Host, request.Host) && (parsed.Scheme == "https" || parsed.Scheme == "http") {
		return true
	}
	return publicOriginAllowed(origin, allowed)
}

func writePublicJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
