package application

import (
	"context"
	"net/http"

	"agent-platform/backend/internal/biz/workspace/domain"
)

// Transport implementations return bounded, credential-free diagnostics.
// Account identification, reception and sending can be implemented separately.
type ChannelAccount interface {
	Identify(context.Context, ChannelCredentials, string) (ChannelIdentity, error)
}

type ChannelReceiver interface {
	Configure(context.Context, ChannelStored, ChannelCredentials, string) error
}

// Webhook receivers authenticate the original request before returning messages.
// The Application commits those messages before returning Response to the caller.
type ChannelWebhookReceiver interface {
	ChannelReceiver
	Callback(context.Context, ChannelStored, ChannelCredentials, http.Header, []byte) (ChannelCallback, error)
}

// A nil sink result means the message was durably received or safely ignored.
// Receivers must not ACK a failed sink call or log message/credential contents.
// The sink context must derive from the connection context, and may add a deadline.
type ChannelMessageSink func(context.Context, domain.ChannelMessage) error

// Connect owns reception until cancellation or failure and releases its resources.
type ChannelStreamReceiver interface {
	ChannelReceiver
	Connect(context.Context, ChannelStored, ChannelCredentials, ChannelMessageSink) error
}

// Sending never invokes reception or starts a Run. The key is stable per Delivery.
type ChannelSender interface {
	Send(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string, string) ChannelSendResult
}

// ChannelTransport explicitly registers one receive mode and an independent sender.
// A provider can reuse an implementation across roles without a combined interface
// or unsupported-method stubs. Registration is fixed at process startup.
type ChannelTransport struct {
	Account         ChannelAccount
	WebhookReceiver ChannelWebhookReceiver
	StreamReceiver  ChannelStreamReceiver
	Sender          ChannelSender
}

func (t ChannelTransport) receiver() ChannelReceiver {
	if t.WebhookReceiver != nil && t.StreamReceiver == nil {
		return t.WebhookReceiver
	}
	if t.StreamReceiver != nil && t.WebhookReceiver == nil {
		return t.StreamReceiver
	}
	return nil
}

func (t ChannelTransport) complete() bool {
	return t.Account != nil && t.Sender != nil && t.receiver() != nil
}
