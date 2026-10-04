package messagechannel

import (
	"net/http"

	"agent-platform/backend/internal/biz/workspace/application"
)

// NewTransports registers each provider's actual reception mode and sender.
// Adding a provider does not change Application dispatch or connection supervision.
type TransportOptions struct {
	ApprovedEndpoints []ApprovedEndpoint
	Cursor            application.ChannelReceiveCursor
	dialSocket        channelSocketDialer
}

func NewTransports(transport http.RoundTripper, options ...TransportOptions) map[string]application.ChannelTransport {
	var option TransportOptions
	if len(options) > 0 {
		option = options[0]
	}
	dial := option.dialSocket
	if dial == nil {
		dial = dialChannelSocket
	}
	h := NewHTTP(transport)
	h.approve(option.ApprovedEndpoints, transport)
	matrix, whatsapp := &Matrix{HTTP: h, cursor: option.Cursor}, &WhatsApp{h}
	signal, bluebubbles := &Signal{HTTP: h, cursor: option.Cursor}, &BlueBubbles{HTTP: h, cursor: option.Cursor}
	wecom, wechat := &WeCom{dial: dial}, &WeChat{HTTP: h, cursor: option.Cursor}
	qqbot, yuanbao := &QQBot{HTTP: h, dial: dial}, &Yuanbao{HTTP: h, dial: dial}
	telegram, slack := &Telegram{h}, &Slack{h}
	discord, dingtalk, feishu := &Discord{h}, &DingTalk{h}, &Feishu{h}
	return map[string]application.ChannelTransport{
		"matrix":      {Account: matrix, StreamReceiver: matrix, Sender: matrix},
		"whatsapp":    {Account: whatsapp, WebhookReceiver: whatsapp, Sender: whatsapp},
		"signal":      {Account: signal, StreamReceiver: signal, Sender: signal},
		"wecom":       {Account: wecom, StreamReceiver: wecom, Sender: wecom},
		"wechat":      {Account: wechat, StreamReceiver: wechat, Sender: wechat, Typing: wechat},
		"qqbot":       {Account: qqbot, StreamReceiver: qqbot, Sender: qqbot},
		"bluebubbles": {Account: bluebubbles, StreamReceiver: bluebubbles, Sender: bluebubbles},
		"yuanbao":     {Account: yuanbao, StreamReceiver: yuanbao, Sender: yuanbao},
		"telegram":    {Account: telegram, WebhookReceiver: telegram, Sender: telegram},
		"slack":       {Account: slack, WebhookReceiver: slack, Sender: slack},
		"discord":     {Account: discord, StreamReceiver: discord, Sender: discord},
		"dingtalk":    {Account: dingtalk, StreamReceiver: dingtalk, Sender: dingtalk, Response: dingtalk, ResponseFallback: dingtalk},
		"feishu":      {Account: feishu, StreamReceiver: feishu, Sender: feishu, Response: feishu},
	}
}
