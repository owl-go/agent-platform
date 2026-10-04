package application

import (
	"context"
	"net/http"
	"net/url"

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

// Native authenticated handshakes can report transport health without inventing
// an incoming user message. Only connecting and connected are accepted.
type ChannelConnectionHealthSink func(context.Context, string) error
type ChannelStreamHealthReceiver interface {
	ChannelStreamReceiver
	ConnectWithHealth(context.Context, ChannelStored, ChannelCredentials, ChannelMessageSink, ChannelConnectionHealthSink) error
}

// Sending never invokes reception or starts a Run. The key is stable per Delivery.
type ChannelSender interface {
	Send(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string, string) ChannelSendResult
}

// Typing is optional, transient provider activity, independent of answer delivery.
// Its session retains any provider ticket only until the execution ends.
type ChannelTypingSender interface {
	BeginTyping(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage) (ChannelTypingSession, error)
}

type ChannelTypingSession interface {
	Refresh(context.Context) error
	Stop(context.Context) error
}

// Responses reuse one provider message throughout a Run. State contains only
// provider identifiers; creation intent is persisted before the network call.
type ChannelResponseState struct {
	Phase      string `json:"phase,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	ReactionID string `json:"reaction_id,omitempty"`
}
type ChannelResponseSender interface {
	React(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage) (string, error)
	ClearReaction(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string) error
	CreateResponse(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string) ChannelSendResult
	UpdateResponse(context.Context, ChannelStored, ChannelCredentials, string, string, bool) ChannelSendResult
}

// The Runtime executor supplies only cumulative, redacted final-member answer
// text. Raw Runtime events, tools and reasoning are never passed to this port.
type ChannelResponseProgress interface {
	UpdateChannelResponse(context.Context, ExecutionJob, string)
}

// ChannelTransport explicitly registers one receive mode and an independent sender.
// A provider can reuse an implementation across roles without a combined interface
// or unsupported-method stubs. Registration is fixed at process startup.
type ChannelTransport struct {
	Account         ChannelAccount
	WebhookReceiver ChannelWebhookReceiver
	StreamReceiver  ChannelStreamReceiver
	Sender          ChannelSender
	Typing          ChannelTypingSender
	Response        ChannelResponseSender
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

// GET subscription verification is separate from authenticated incoming messages.
type ChannelWebhookChallengeVerifier interface {
	Challenge(context.Context, ChannelStored, ChannelCredentials, url.Values) (string, error)
}
