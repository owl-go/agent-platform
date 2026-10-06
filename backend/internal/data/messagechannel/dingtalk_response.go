package messagechannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

// StandardCard accepts inline JSON, without an app-specific template. Updates
// replace the cumulative content, following DingTalk's official bot tutorial.
func dingTalkCardPreview(preview application.ChannelResponsePreview, final bool) string {
	bounded := func(text string, limit int) string {
		runes := []rune(text)
		return string(runes[:min(limit, len(runes))])
	}
	title := "回复"
	contents := []any{}
	if !final {
		status := bounded(preview.Status, 120)
		if status == "" {
			status = "正在整理回答"
		}
		title = "⏳ 正在处理"
		if status == "等待工作流拥有者处理" {
			title = "⏸ 等待处理"
		}
		contents = append(contents, map[string]string{"type": "text", "id": "progress", "text": fmt.Sprintf("%s · 本步骤已完成 %d 次工具调用 · 已运行 %d 分 %d 秒", status, preview.ToolsCompleted, preview.ElapsedSeconds/60, preview.ElapsedSeconds%60)})
	}
	if preview.Summary != "" {
		contents = append(contents, map[string]string{"type": "markdown", "id": "summary", "text": "**思考摘要**\n\n" + safeCardMarkdown(bounded(preview.Summary, 600))})
	}
	if preview.Answer != "" {
		answer := bounded(preview.Answer, 1800)
		if !final && len([]rune(preview.Answer)) > 1800 {
			answer += "\n\n正在生成更多内容…"
		}
		contents = append(contents, map[string]string{"type": "markdown", "id": "answer", "text": safeCardMarkdown(answer)})
	}
	card, _ := json.Marshal(map[string]any{"config": map[string]bool{"autoLayout": true, "enableForward": false}, "header": map[string]any{"title": map[string]string{"type": "text", "text": title}}, "contents": contents})
	return string(card)
}

func (a *DingTalk) cardRequest(ctx context.Context, c application.ChannelCredentials, method, path string, body any, id string, update bool) application.ChannelSendResult {
	token, status, _, err := a.request(ctx, http.MethodPost, "https://api.dingtalk.com/v1.0/oauth2/accessToken", "", map[string]string{"appKey": c["client_id"], "appSecret": c["client_secret"]})
	if err != nil || status != http.StatusOK || rawString(token["accessToken"]) == "" {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_authentication_failed", RetryAfter: time.Minute}
	}
	result, status, retry, err := a.requestHeaders(ctx, method, "https://api.dingtalk.com"+path, "", body, http.Header{"X-Acs-Dingtalk-Access-Token": []string{rawString(token["accessToken"])}})
	if err == nil && status == http.StatusOK && rawString(result["processQueryKey"]) != "" {
		return application.ChannelSendResult{State: "sent", MessageID: id}
	}
	failed := failedSend(status, retry, err)
	if update && failed.State == "outcome_unknown" {
		// Replacing the same cumulative card is safe to retry; creation is not.
		failed.State, failed.RetryAfter = "retry_wait", time.Second
	}
	return failed
}

func (a *DingTalk) CreateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, key string, preview application.ChannelResponsePreview, final bool) application.ChannelSendResult {
	expires, err := strconv.ParseInt(m.Reply["expires_at"], 10, 64)
	if err != nil || !time.Now().Before(time.UnixMilli(expires)) {
		return application.ChannelSendResult{State: "expired", Code: "provider_reply_expired"}
	}
	if !validDingTalkReply(m.Reply["session_webhook"]) || s.Channel.AccountID == "" || c["client_id"] != s.Channel.AccountID || c["client_secret"] == "" || s.Channel.TenantID == "" || m.TenantID != s.Channel.TenantID || m.SenderID == "" || m.ChatID == "" || key == "" {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	digest := sha256.Sum256([]byte(s.Channel.AccountID + "\x00" + key))
	cardID := hex.EncodeToString(digest[:])
	// Persist only a provider identifier and the originating reply deadline.
	// A card never turns an expired session into unlimited proactive messaging.
	handle := cardID + "." + strconv.FormatInt(expires, 10)
	body := map[string]any{"cardTemplateId": "StandardCard", "cardBizId": cardID, "cardData": dingTalkCardPreview(preview, final), "robotCode": s.Channel.AccountID, "pullStrategy": false, "sendOptions": map[string]bool{"atAll": false}}
	if m.Group {
		body["openConversationId"] = m.ChatID
		atUsers, _ := json.Marshal([]map[string]string{{"userId": m.SenderID}})
		body["sendOptions"] = map[string]any{"atAll": false, "atUserListJson": string(atUsers)}
	} else {
		receiver, _ := json.Marshal(map[string]string{"userId": m.SenderID})
		body["singleChatReceiver"] = string(receiver)
	}
	return a.cardRequest(ctx, c, http.MethodPost, "/v1.0/im/v1.0/robot/interactiveCards/send", body, handle, false)
}

func (a *DingTalk) UpdateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, _ domain.ChannelMessage, handle string, preview application.ChannelResponsePreview, final bool) application.ChannelSendResult {
	parts := strings.Split(handle, ".")
	if len(parts) != 2 || len(parts[0]) != 64 || strings.ToLower(parts[0]) != parts[0] {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	if _, err := hex.DecodeString(parts[0]); err != nil || s.Channel.AccountID == "" || c["client_id"] != s.Channel.AccountID || c["client_secret"] == "" {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	expires, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || !time.Now().Before(time.UnixMilli(expires)) {
		return application.ChannelSendResult{State: "expired", Code: "provider_reply_expired"}
	}
	return a.cardRequest(ctx, c, http.MethodPut, "/v1.0/im/robots/interactiveCards", map[string]string{"cardBizId": parts[0], "cardData": dingTalkCardPreview(preview, final)}, handle, true)
}
