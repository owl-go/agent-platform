package messagechannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/gorilla/websocket"
)

// Leave thirty seconds inside the provider's six-minute stream window. The
// received timestamp is protected with the callback req_id in the Inbox.
const wecomStreamWindow = 330 * time.Second

func wecomStreamIdentity(s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage) (string, int64, bool) {
	received, err := strconv.ParseInt(m.Reply["received_at"], 10, 64)
	reqID := m.Reply["req_id"]
	if err != nil || received <= 0 || received > time.Now().Add(time.Second).UnixMilli() || reqID == "" || len(reqID) > 256 || !utf8.ValidString(reqID) || strings.ContainsFunc(reqID, unicode.IsControl) || s.Channel.AccountID == "" || c["bot_id"] != s.Channel.AccountID || c["bot_secret"] == "" || m.MessageID == "" || m.SenderID == "" || m.ChatID == "" || (!m.Group && m.ChatID != m.SenderID) {
		return "", 0, false
	}
	digest := sha256.Sum256([]byte(s.Channel.AccountID + "\x00" + m.MessageID))
	return hex.EncodeToString(digest[:]), received + wecomStreamWindow.Milliseconds(), true
}

func wecomStreamPreview(preview application.ChannelResponsePreview, final bool) string {
	bounded := func(text string, limit int) string {
		runes := []rune(text)
		return string(runes[:min(limit, len(runes))])
	}
	parts := []string{}
	if !final {
		status := bounded(preview.Status, 120)
		if status == "" || status == "已收到，正在准备执行" {
			status = "正在分析"
		}
		parts = append(parts, safeCardMarkdown(fmt.Sprintf("⏳ %s · 本步骤已完成 %d 次工具调用 · 已运行 %d 分 %d 秒", status, preview.ToolsCompleted, preview.ElapsedSeconds/60, preview.ElapsedSeconds%60)))
	}
	if preview.Summary != "" {
		parts = append(parts, "**思考摘要**\n\n"+safeCardMarkdown(bounded(preview.Summary, 600)))
	}
	if preview.Answer != "" {
		answer := bounded(preview.Answer, 1800)
		if !final && len([]rune(preview.Answer)) > 1800 {
			answer += "\n\n正在生成更多内容…"
		}
		parts = append(parts, safeCardMarkdown(answer))
	}
	return strings.Join(parts, "\n\n")
}

func (a *WeCom) CreateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, key string, preview application.ChannelResponsePreview, final bool) application.ChannelSendResult {
	id, expires, valid := wecomStreamIdentity(s, c, m)
	if !valid || key == "" {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	// The first reply must also satisfy the callback's five-second deadline.
	initialDeadline := time.UnixMilli(expires - wecomStreamWindow.Milliseconds()).Add(5 * time.Second)
	if !time.Now().Before(initialDeadline) {
		return application.ChannelSendResult{State: "expired", Code: "provider_stream_expired"}
	}
	requestCtx, cancel := context.WithDeadline(ctx, initialDeadline)
	defer cancel()
	// The handle contains no callback capability or draft. Updates re-read the
	// original encrypted Reply through the application port.
	handle := id + "." + strconv.FormatInt(expires, 10)
	return a.UpdateResponse(requestCtx, s, c, m, handle, preview, final)
}

func (a *WeCom) UpdateResponse(ctx context.Context, s application.ChannelStored, c application.ChannelCredentials, m domain.ChannelMessage, handle string, preview application.ChannelResponsePreview, final bool) application.ChannelSendResult {
	id, expires, valid := wecomStreamIdentity(s, c, m)
	openHandle := id + "." + strconv.FormatInt(expires, 10)
	closedHandle := openHandle + ".closed"
	if !valid || (handle != openHandle && handle != closedHandle) || (final && len([]rune(preview.Answer)) > 1800) {
		return application.ChannelSendResult{State: "failed", Code: "provider_reply_invalid"}
	}
	if handle == closedHandle || !time.Now().Before(time.UnixMilli(expires)) {
		return application.ChannelSendResult{State: "expired", Code: "provider_stream_expired"}
	}
	session := a.bindings.get(s)
	if session == nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_disconnected", RetryAfter: 30 * time.Second}
	}
	// Successive frames share req_id. Serialize requests, and discard the entire
	// connection after an uncertain ACK so a late receipt cannot confirm a final
	// frame that was never accepted. Waiting for the gate respects cancellation.
	select {
	case session.replyGate <- struct{}{}:
		defer func() { <-session.replyGate }()
	case <-ctx.Done():
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_send_pending", RetryAfter: time.Second}
	case <-session.ctx.Done():
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_disconnected", RetryAfter: 30 * time.Second}
	}
	if ctx.Err() != nil {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_send_pending", RetryAfter: time.Second}
	}
	if !time.Now().Before(time.UnixMilli(expires)) {
		return application.ChannelSendResult{State: "expired", Code: "provider_stream_expired"}
	}
	requestCtx, cancel := context.WithDeadline(ctx, time.UnixMilli(expires))
	defer cancel()
	closing := !final && time.Now().Add(5*time.Second).UnixMilli() >= expires
	if closing {
		preview.Answer = ""
	}
	content := wecomStreamPreview(preview, final)
	if closing {
		content += "\n\n任务尚未结束，结果将另行发送。"
	}
	if m.Group {
		content = safeCardMarkdown("["+m.SenderID+"] ") + content
	}
	body := map[string]any{"msgtype": "stream", "stream": map[string]any{"id": id, "finish": final || closing, "content": content}}
	data, err := session.request(requestCtx, m.Reply["req_id"], websocket.TextMessage, wecomRequest("aibot_respond_msg", m.Reply["req_id"], body))
	var frame wecomFrame
	if err != nil || json.Unmarshal(data, &frame) != nil || frame.Code == nil {
		session.socket.Close()
		return application.ChannelSendResult{State: "outcome_unknown", Code: "provider_send_unconfirmed"}
	}
	if *frame.Code == 45009 {
		return application.ChannelSendResult{State: "retry_wait", Code: "provider_rate_limited", RetryAfter: time.Minute}
	}
	if *frame.Code != 0 {
		return application.ChannelSendResult{State: "failed", Code: "provider_rejected"}
	}
	if closing {
		handle = closedHandle
	}
	return application.ChannelSendResult{State: "sent", MessageID: handle}
}
