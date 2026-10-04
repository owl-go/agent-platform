package messagechannel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
)

func (a *Feishu) React(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage) (string, error) {
	token, err := a.token(ctx, c, s.Channel.Region)
	if err != nil {
		return "", err
	}
	result, status, _, err := a.request(ctx, http.MethodPost, feishuBase(s.Channel.Region)+"/open-apis/im/v1/messages/"+url.PathEscape(m.MessageID)+"/reactions", "Bearer "+token, map[string]any{"reaction_type": map[string]string{"emoji_type": "Typing"}})
	code, valid := feishuResponseCode(result)
	var data struct {
		ID string `json:"reaction_id"`
	}
	if err != nil || status != http.StatusOK || !valid || code != 0 || json.Unmarshal(result["data"], &data) != nil || data.ID == "" {
		return "", providerError("provider_reaction_failed")
	}
	return data.ID, nil
}
func (a *Feishu) ClearReaction(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, id string) error {
	token, err := a.token(ctx, c, s.Channel.Region)
	if err != nil {
		return err
	}
	result, status, _, err := a.request(ctx, http.MethodDelete, feishuBase(s.Channel.Region)+"/open-apis/im/v1/messages/"+url.PathEscape(m.MessageID)+"/reactions/"+url.PathEscape(id), "Bearer "+token, nil)
	code, valid := feishuResponseCode(result)
	if err != nil || status != http.StatusOK || !valid || code != 0 {
		return providerError("provider_reaction_failed")
	}
	return nil
}

// One shared card is patched at most once per second. Limit text by its worst
// JSON escaping expansion so the outer request stays below Feishu's 30 KB cap.
func feishuCardPreview(preview application.ChannelResponsePreview, final bool) string {
	text := preview.Answer
	title := "正在生成回复…"
	if final {
		title = "回复"
	}
	summaryRunes := []rune(preview.Summary)
	summary := string(summaryRunes[:min(600, len(summaryRunes))])
	limit := 2500
	if !final || summary != "" {
		limit = 1800
	}
	if runes := []rune(text); len(runes) > limit {
		text = string(runes[:limit])
		if final {
			text += "\n\n内容较长，请联系工作流拥有者查看完整结果。"
		} else {
			text += "\n\n正在生成更多内容…"
		}
	}
	if summary != "" {
		text = "思考摘要\n" + summary + "\n\n回答\n" + text
	}
	if !final {
		status := preview.Status
		if status == "" {
			status = "正在整理回答"
		}
		title = status
		progress := fmt.Sprintf("当前进度：%s · 本步骤已完成 %d 次工具调用 · 已运行 %d 分 %d 秒", status, preview.ToolsCompleted, preview.ElapsedSeconds/60, preview.ElapsedSeconds%60)
		text = progress + "\n\n" + text
	}
	// Dynamic answer Markdown must not embed provider images or mass mentions.
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	card, _ := json.Marshal(map[string]any{
		"config":   map[string]bool{"wide_screen_mode": true, "update_multi": true},
		"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}},
		"elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": text}}},
	})
	return string(card)
}
func feishuCard(text string, final bool) string {
	return feishuCardPreview(application.ChannelResponsePreview{Answer: text}, final)
}
func (a *Feishu) CreateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, key string) application.ChannelSendResult {
	return a.sendCard(ctx, s, c, m, "正在处理你的消息…", key, false)
}
func (a *Feishu) sendCard(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, text, key string, final bool) application.ChannelSendResult {
	token, err := a.token(ctx, c, s.Channel.Region)
	if err != nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_authentication_failed", RetryAfter: time.Minute}
	}
	body := map[string]any{"msg_type": "interactive", "content": feishuCard(text, final), "uuid": key}
	if m.ThreadID != "" {
		body["reply_in_thread"] = true
	}
	result, status, retry, err := a.request(ctx, http.MethodPost, feishuBase(s.Channel.Region)+"/open-apis/im/v1/messages/"+url.PathEscape(m.MessageID)+"/reply", "Bearer "+token, body)
	return feishuCardResult(result, status, retry, err, "")
}
func (a *Feishu) UpdateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, id string, preview application.ChannelResponsePreview, final bool) application.ChannelSendResult {
	token, err := a.token(ctx, c, s.Channel.Region)
	if err != nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_authentication_failed", RetryAfter: time.Minute}
	}
	result, status, retry, err := a.request(ctx, http.MethodPatch, feishuBase(s.Channel.Region)+"/open-apis/im/v1/messages/"+url.PathEscape(id), "Bearer "+token, map[string]string{"content": feishuCardPreview(preview, final)})
	sent := feishuCardResult(result, status, retry, err, id)
	// Repeating the same cumulative card replacement is safe after uncertainty.
	if sent.State == "outcome_unknown" {
		sent.State, sent.RetryAfter = "retry_wait", time.Second
	}
	return sent
}
func feishuCardResult(result map[string]json.RawMessage, status int, retry time.Duration, err error, id string) application.ChannelSendResult {
	code, valid := feishuResponseCode(result)
	if status == http.StatusTooManyRequests || valid && (code == 99991400 || code == 99991401) {
		return failedSend(http.StatusTooManyRequests, max(retry, time.Second), err)
	}
	if err != nil || status != http.StatusOK || !valid || code != 0 {
		if status == http.StatusOK && (err != nil || !valid) {
			return failedSend(0, retry, err)
		}
		if status == http.StatusOK {
			status = http.StatusBadRequest
		}
		return failedSend(status, retry, err)
	}
	if id == "" {
		var data struct {
			ID string `json:"message_id"`
		}
		if json.Unmarshal(result["data"], &data) != nil || data.ID == "" {
			return failedSend(0, 0, nil)
		}
		id = data.ID
	}
	return application.ChannelSendResult{State: "sent", MessageID: id}
}
