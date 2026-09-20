package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	aiapplicationdomain "agent-platform/backend/internal/biz/aiapplication/domain"
	workspacedomain "agent-platform/backend/internal/biz/workspace/domain"
)

const visitorCookieName = "agent_workspace_visitor"

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
	if !publicOriginAllowed(request.Header.Get("Origin"), assistant.Share.AllowedOrigins) {
		writeAuthError(writer, http.StatusForbidden, "origin_not_allowed")
		return
	}
	visitorHash, visitorCookie, visitorErr := publicVisitor(request)
	if visitorErr != nil {
		writeAuthError(writer, http.StatusInternalServerError, "request_failed")
		return
	}
	if visitorCookie != nil {
		http.SetCookie(writer, visitorCookie)
	}
	if len(parts) == 9 && parts[5] == "conversations" && parts[7] == "responses" && request.Method == http.MethodGet {
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
		if aiapplicationdomain.DefaultSafetyPolicy().Decide(input.Question) == aiapplicationdomain.SafetyRefuse {
			writePublicJSON(writer, map[string]string{"kind": "refusal", "answer": aiapplicationdomain.SafetyRefusal}, http.StatusOK)
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
		faq, matched, faqErr := service.aiapplications.FAQAnswer(request.Context(), assistant.OwnerID, assistant.ID, input.Question)
		if faqErr != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		if matched {
			writePublicJSON(writer, map[string]any{"kind": "faq", "answer_markdown": faq.AnswerMarkdown, "faq_id": faq.ID}, http.StatusOK)
			return
		}
		grounded := false
		if len(assistant.KnowledgeBaseIDs) > 0 {
			chunks, searchErr := service.aiapplications.SearchKnowledge(request.Context(), assistant.OwnerID, assistant.KnowledgeBaseIDs, input.Question, 5)
			if searchErr != nil {
				writeAuthError(writer, http.StatusInternalServerError, "request_failed")
				return
			}
			if len(chunks) > 0 {
				grounded = true
			}
		}
		if !grounded {
			writePublicJSON(writer, map[string]string{"kind": "refusal", "answer": aiapplicationdomain.SafetyRefusal}, http.StatusOK)
			return
		}
		response, createErr := service.enqueueExternalAnswer(request.Context(), assistant, visitorHash, input.ConversationID, input.Question)
		if createErr != nil {
			writeAuthError(writer, http.StatusInternalServerError, "request_failed")
			return
		}
		writePublicJSON(writer, map[string]any{"kind": "generating", "response_id": response.ID, "conversation_id": response.ConversationID, "state": response.State}, http.StatusAccepted)
		return
	}
	http.NotFound(writer, request)
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
		cookie = &http.Cookie{Name: visitorCookieName, Value: visitor, Path: "/", HttpOnly: true, SameSite: http.SameSiteNoneMode, Secure: request.TLS != nil, MaxAge: 86400}
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
	if !publicOriginAllowed(request.Header.Get("Origin"), assistant.Share.AllowedOrigins) {
		writeAuthError(writer, http.StatusForbidden, "origin_not_allowed")
		return
	}
	frameAncestors := "*"
	if len(assistant.Share.AllowedOrigins) > 0 {
		frameAncestors = strings.Join(assistant.Share.AllowedOrigins, " ")
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors "+frameAncestors)
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "no-store")
	name := html.EscapeString(assistant.Name)
	intro := html.EscapeString(assistant.Introduction)
	publicURL := "/api/v1/public/assistants/" + html.EscapeString(token)
	visitorScript := `<script>const api='` + publicURL + `';const out=document.querySelector('#answer');let conversationId='';function show(v){if(v.answer_markdown||v.answer){out.textContent=v.answer_markdown||v.answer;return}out.textContent=v.kind==='generating'?'正在整理答案…':'暂时无法回答此类问题'}async function waitForAnswer(v){for(let attempt=0;attempt<30;attempt++){const response=await fetch(api+'/conversations/'+encodeURIComponent(v.conversation_id)+'/responses/'+encodeURIComponent(v.response_id));const next=await response.json();if(next.kind!=='generating'){conversationId=next.conversation_id;show(next);return}await new Promise(resolve=>setTimeout(resolve,1000))}show({kind:'error',answer:'回答生成超时，请稍后重试'})}fetch(api).then(r=>r.json()).then(d=>{for(const f of d.faqs||[]){const b=document.createElement('button');b.className='faq';b.type='button';b.textContent=f.question;b.onclick=()=>show(f);document.querySelector('#faqs').appendChild(b)}});document.querySelector('#form').onsubmit=async(e)=>{e.preventDefault();const q=document.querySelector('#question').value;const response=await fetch(api+'/answer',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q,conversation_id:conversationId||undefined})});const next=await response.json();if(next.kind==='generating'){conversationId=next.conversation_id;await waitForAnswer(next)}else show(next)}</script>`
	writer.Write([]byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + name + `</title><style>body{font-family:system-ui,sans-serif;margin:0;padding:20px;color:#20342b;background:#f7faf8}main{max-width:720px;margin:auto;background:#fff;border:1px solid #dce8df;border-radius:16px;padding:20px}h1{margin:0 0 6px}p{color:#5b6c63}.faq{display:block;width:100%;text-align:left;border:1px solid #dce8df;border-radius:10px;background:#fff;padding:10px;margin:8px 0;cursor:pointer}textarea{box-sizing:border-box;width:100%;min-height:80px;padding:10px;border:1px solid #cbdad0;border-radius:10px}button[type=submit]{margin-top:8px;padding:9px 14px;border:0;border-radius:9px;background:#2b7d5b;color:#fff;cursor:pointer}#answer{white-space:pre-wrap;margin-top:16px}</style></head><body><main><h1>` + name + `</h1><p>` + intro + `</p><section id="faqs"></section><form id="form"><textarea id="question" placeholder="请输入问题"></textarea><button type="submit">提交问题</button></form><div id="answer" role="status"></div></main>` + visitorScript + `</body></html>`))
	return
	_, _ = writer.Write([]byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + name + `</title><style>body{font-family:system-ui,sans-serif;margin:0;padding:20px;color:#20342b;background:#f7faf8}main{max-width:720px;margin:auto;background:#fff;border:1px solid #dce8df;border-radius:16px;padding:20px}h1{margin:0 0 6px}p{color:#5b6c63}.faq{display:block;width:100%;text-align:left;border:1px solid #dce8df;border-radius:10px;background:#fff;padding:10px;margin:8px 0;cursor:pointer}textarea{box-sizing:border-box;width:100%;min-height:80px;padding:10px;border:1px solid #cbdad0;border-radius:10px}button[type=submit]{margin-top:8px;padding:9px 14px;border:0;border-radius:9px;background:#2b7d5b;color:#fff;cursor:pointer}#answer{white-space:pre-wrap;margin-top:16px}</style></head><body><main><h1>` + name + `</h1><p>` + intro + `</p><section id="faqs"></section><form id="form"><textarea id="question" placeholder="请输入问题"></textarea><button type="submit">提交问题</button></form><div id="answer" role="status"></div></main><script>const api=` + "'" + publicURL + "'" + `;const out=document.querySelector('#answer');function show(v){if(v.answer_markdown||v.answer){out.textContent=v.answer_markdown||v.answer;return}if(v.kind==='grounded_context'){out.textContent=(v.chunks||[]).map(c=>c.text).join('\n\n');return}out.textContent='暂时无法回答此类问题'}fetch(api).then(r=>r.json()).then(d=>{for(const f of d.faqs||[]){const b=document.createElement('button');b.className='faq';b.type='button';b.textContent=f.question;b.onclick=()=>show(f);document.querySelector('#faqs').appendChild(b)}});document.querySelector('#form').onsubmit=async(e)=>{e.preventDefault();const q=document.querySelector('#question').value;const r=await fetch(api+'/answer',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q})});show(await r.json())}</script></body></html>`))
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

func writePublicJSON(writer http.ResponseWriter, value any, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
