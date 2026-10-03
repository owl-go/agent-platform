package application

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type transportTestCipher struct{}

func (transportTestCipher) Encrypt(b []byte, _ string) ([]byte, error) {
	return append([]byte(nil), b...), nil
}
func (transportTestCipher) Decrypt(b []byte, _ string) ([]byte, error) {
	return append([]byte(nil), b...), nil
}

type transportTestAccount struct{}

func (transportTestAccount) Identify(context.Context, ChannelCredentials, string) (ChannelIdentity, error) {
	return ChannelIdentity{ID: "bot"}, nil
}

type transportTestWebhook struct {
	callbackURL string
	callback    ChannelCallback
	err         error
}

func (r *transportTestWebhook) Configure(_ context.Context, _ ChannelStored, _ ChannelCredentials, target string) error {
	r.callbackURL = target
	return r.err
}
func (r *transportTestWebhook) Callback(context.Context, ChannelStored, ChannelCredentials, http.Header, []byte) (ChannelCallback, error) {
	return r.callback, r.err
}

type transportTestSender struct {
	calls     int
	text, key string
	message   domain.ChannelMessage
	result    ChannelSendResult
}

func (s *transportTestSender) Send(_ context.Context, _ ChannelStored, _ ChannelCredentials, m domain.ChannelMessage, text, key string) ChannelSendResult {
	s.calls++
	s.text, s.key, s.message = text, key, m
	return s.result
}

type transportTestRepository struct {
	MessageChannelRepository
	stored         ChannelStored
	job            *ChannelSendJob
	received       []domain.ChannelMessage
	reply          []byte
	receiveErr     error
	finish         ChannelSendResult
	admitted       int
	active         []ChannelStored
	receiveContext context.Context
}

func (r *transportTestRepository) GetMessageChannel(context.Context, string, string, string) (ChannelStored, error) {
	return r.stored, nil
}
func (r *transportTestRepository) ControlMessageChannel(_ context.Context, _, _, _ string, _ int64, _, _ string) (ChannelStored, error) {
	return r.stored, nil
}
func (r *transportTestRepository) ReceiveChannelMessage(ctx context.Context, _ ChannelStored, m domain.ChannelMessage, reply []byte) (bool, error) {
	r.receiveContext = ctx
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if r.receiveErr != nil {
		return false, r.receiveErr
	}
	r.received = append(r.received, m)
	r.reply = append([]byte(nil), reply...)
	return true, nil
}
func (r *transportTestRepository) ClaimChannelDelivery(context.Context) (*ChannelSendJob, error) {
	return r.job, nil
}
func (r *transportTestRepository) FinishChannelDelivery(_ context.Context, _ *ChannelSendJob, result ChannelSendResult) error {
	r.finish = result
	return nil
}
func (r *transportTestRepository) AdmitChannelMessage(context.Context) (bool, error) {
	r.admitted++
	return true, nil
}
func (r *transportTestRepository) ActiveMessageChannels(context.Context) ([]ChannelStored, error) {
	return r.active, nil
}
func (r *transportTestRepository) SetChannelHealth(context.Context, string, int64, string, string) error {
	return nil
}

func transportTestFixture() (*transportTestRepository, *transportTestWebhook, *transportTestSender, ChannelTransport) {
	c := domain.MessageChannel{ID: "channel", Provider: "test-webhook", OwnerID: "owner", WorkflowID: "workflow", Enabled: true, Version: 1, ConfigVersion: 1, Audience: domain.ChannelAudience{SenderIDs: []string{"alice"}, AllowDirect: true}}
	credentials, _ := json.Marshal(ChannelCredentials{"bot_token": "secret-token"})
	message := domain.ChannelMessage{EventID: "event", MessageID: "message", SenderID: "alice", ChatID: "chat", ThreadID: "thread", Text: "question secret-token", OccurredAt: time.Now(), Reply: map[string]string{"target": "private-reply"}}
	repo := &transportTestRepository{stored: ChannelStored{Channel: c, Ciphertext: credentials}}
	receiver := &transportTestWebhook{callback: ChannelCallback{Messages: []domain.ChannelMessage{message}, Response: map[string]string{"ack": "ok"}}}
	sender := &transportTestSender{result: ChannelSendResult{State: "sent", MessageID: "receipt"}}
	return repo, receiver, sender, ChannelTransport{Account: transportTestAccount{}, WebhookReceiver: receiver, Sender: sender}
}

func TestChannelWebhookUsesIndependentReceiverAndCommitsBeforeACK(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "committed", true: "repository_failure"}[fail], func(t *testing.T) {
			repo, receiver, sender, _ := transportTestFixture()
			if fail {
				repo.receiveErr = errors.New("inbox unavailable")
			}
			registry := map[string]ChannelTransport{"test-webhook": {WebhookReceiver: receiver}}
			app := NewMessageChannels(repo, transportTestCipher{}, registry, true, "https://workspace.example.test")
			delete(registry, "test-webhook") // Startup registration is owned by the Application.
			response, err := app.Callback(context.Background(), "test-webhook", "channel", nil, nil)
			if fail {
				if !errors.Is(err, repo.receiveErr) || response != nil || len(repo.received) != 0 {
					t.Fatalf("failure acknowledged: %v, %v", response, err)
				}
			} else {
				if err != nil || response == nil || len(repo.received) != 1 {
					t.Fatalf("callback: %v, %v", response, err)
				}
				if repo.received[0].Text != "question [REDACTED]" || repo.received[0].Reply != nil || len(repo.reply) == 0 {
					t.Fatal("receive protection not applied")
				}
			}
			if sender.calls != 0 || repo.admitted != 0 {
				t.Fatal("reception sent an answer or admitted a Run")
			}
			if _, ok := any(receiver).(ChannelSender); ok {
				t.Fatal("receiver fake must be independent of sender")
			}
		})
	}
}

func TestChannelReceiverConfigurationUsesRegisteredReceiveMode(t *testing.T) {
	repo, receiver, sender, transport := transportTestFixture()
	repo.stored.Channel.Enabled = false
	app := NewMessageChannels(repo, transportTestCipher{}, map[string]ChannelTransport{"test-webhook": transport}, true, "https://workspace.example.test/")
	c, err := app.Control(context.Background(), "owner", "workflow", "channel", 1, "validate")
	expected := "https://workspace.example.test/api/v1/message-channel-callbacks/test-webhook/channel"
	if err != nil || receiver.callbackURL != expected || c.CallbackURL != expected {
		t.Fatalf("configure: %q, %q, %v", receiver.callbackURL, c.CallbackURL, err)
	}
	if sender.calls != 0 {
		t.Fatal("configuration invoked sender")
	}
}

func TestChannelSenderUsesSavedReplyWithoutReceiverOrRunAdmission(t *testing.T) {
	repo, _, sender, _ := transportTestFixture()
	reply, _ := json.Marshal(map[string]string{"target": "private-reply"})
	repo.job = &ChannelSendJob{Stored: repo.stored, ReplyCiphertext: reply, Delivery: domain.ChannelDelivery{ID: "stable-delivery"}, Message: domain.ChannelMessage{EventID: "event", ChatID: "chat", ThreadID: "thread"}, Text: "answer secret-token"}
	// The delivery loop needs only its sender, not account identification or reception.
	app := NewMessageChannels(repo, transportTestCipher{}, map[string]ChannelTransport{"test-webhook": {Sender: sender}}, true, "")
	for range 2 {
		processed, err := app.ProcessDelivery(context.Background())
		if err != nil || !processed {
			t.Fatalf("delivery: %v, %v", processed, err)
		}
	}
	if sender.calls != 2 || sender.text != "answer [REDACTED]" || sender.key != "stable-delivery" || sender.message.Reply["target"] != "private-reply" || sender.message.ThreadID != "thread" || repo.finish != sender.result {
		t.Fatalf("saved delivery changed: %#v, %#v", sender, repo.finish)
	}
	if repo.admitted != 0 || len(repo.received) != 0 {
		t.Fatal("sending admitted a Run or received a message")
	}
}

func TestChannelMissingSenderFailsDeliveryWithoutPanic(t *testing.T) {
	repo, _, _, _ := transportTestFixture()
	repo.job = &ChannelSendJob{Stored: repo.stored}
	app := NewMessageChannels(repo, transportTestCipher{}, nil, true, "")
	processed, err := app.ProcessDelivery(context.Background())
	if err != nil || !processed || repo.finish.State != "failed" || repo.finish.Code != "provider_unavailable" {
		t.Fatalf("missing sender: %v, %v, %#v", processed, err, repo.finish)
	}
}

type transportTestStream struct {
	configured chan struct{}
	received   chan error
	stopped    chan struct{}
	message    domain.ChannelMessage
	expired    bool
}

func (r *transportTestStream) Configure(context.Context, ChannelStored, ChannelCredentials, string) error {
	close(r.configured)
	return nil
}
func (r *transportTestStream) Connect(ctx context.Context, _ ChannelStored, _ ChannelCredentials, sink ChannelMessageSink) error {
	defer close(r.stopped)
	deadline := time.Now().Add(time.Minute)
	if r.expired {
		deadline = time.Now().Add(-time.Second)
	}
	receiveCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	r.received <- sink(receiveCtx, r.message)
	<-ctx.Done()
	return ctx.Err()
}

func TestChannelStreamUsesSinkDeadlineAndStopsOnRemoval(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "committed", true: "expired_context"}[expired], func(t *testing.T) {
			repo, webhook, sender, _ := transportTestFixture()
			repo.stored.Channel.Provider = "test-stream"
			repo.active = []ChannelStored{repo.stored}
			receiver := &transportTestStream{configured: make(chan struct{}), received: make(chan error, 1), stopped: make(chan struct{}), message: webhook.callback.Messages[0], expired: expired}
			app := NewMessageChannels(repo, transportTestCipher{}, map[string]ChannelTransport{"test-stream": {StreamReceiver: receiver}}, true, "")
			connections := NewChannelConnections(app, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := connections.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-receiver.received:
				if expired && !errors.Is(err, context.DeadlineExceeded) || !expired && err != nil {
					t.Fatalf("sink deadline: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stream did not receive")
			}
			if _, ok := repo.receiveContext.Deadline(); !ok {
				t.Fatal("sink lost transport deadline")
			}
			if expired && len(repo.received) != 0 || !expired && len(repo.received) != 1 || sender.calls != 0 {
				t.Fatal("stream ignored sink result or invoked sender")
			}
			if response, err := app.Callback(ctx, "test-stream", "channel", nil, nil); !errors.Is(err, domain.ErrInvalid) || response != nil {
				t.Fatal("stream accepted webhook")
			}
			repo.active = nil
			if _, err := connections.ProcessNext(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case <-receiver.stopped:
			case <-time.After(3 * time.Second):
				t.Fatal("removed stream remained connected")
			}
		})
	}
}

func TestChannelRegistrationRejectsMissingOrAmbiguousRoles(t *testing.T) {
	repo, _, _, transport := transportTestFixture()
	stream := &transportTestStream{}
	cases := map[string]ChannelTransport{
		"missing_account":    {WebhookReceiver: transport.WebhookReceiver, Sender: transport.Sender},
		"missing_sender":     {Account: transport.Account, WebhookReceiver: transport.WebhookReceiver},
		"missing_receiver":   {Account: transport.Account, Sender: transport.Sender},
		"ambiguous_receiver": {Account: transport.Account, WebhookReceiver: transport.WebhookReceiver, StreamReceiver: stream, Sender: transport.Sender},
	}
	for name, registration := range cases {
		t.Run(name, func(t *testing.T) {
			app := NewMessageChannels(repo, transportTestCipher{}, map[string]ChannelTransport{"test-webhook": registration}, true, "")
			_, err := app.Save(context.Background(), "owner", "workflow", "", 0, domain.MessageChannel{Provider: "test-webhook", Name: "test", Audience: repo.stored.Channel.Audience}, nil)
			if !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("incomplete registration accepted: %v", err)
			}
			if registration.receiver() == nil {
				if response, err := app.Callback(context.Background(), "test-webhook", "channel", nil, nil); !errors.Is(err, domain.ErrInvalid) || response != nil {
					t.Fatal("invalid receive mode accepted callback")
				}
			}
		})
	}
}
