package messagechannel

import (
	"net/http"

	"agent-platform/backend/internal/biz/workspace/application"
)

// NewTransports registers each provider's actual reception mode and sender.
// Adding a provider does not change Application dispatch or connection supervision.
func NewTransports(transport http.RoundTripper) map[string]application.ChannelTransport {
	h := NewHTTP(transport)
	telegram, slack := &Telegram{h}, &Slack{h}
	discord, dingtalk, feishu := &Discord{h}, &DingTalk{h}, &Feishu{h}
	return map[string]application.ChannelTransport{
		"telegram": {Account: telegram, WebhookReceiver: telegram, Sender: telegram},
		"slack":    {Account: slack, WebhookReceiver: slack, Sender: slack},
		"discord":  {Account: discord, StreamReceiver: discord, Sender: discord},
		"dingtalk": {Account: dingtalk, StreamReceiver: dingtalk, Sender: dingtalk},
		"feishu":   {Account: feishu, StreamReceiver: feishu, Sender: feishu},
	}
}
