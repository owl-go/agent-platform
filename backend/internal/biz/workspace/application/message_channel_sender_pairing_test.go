package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
)

type pairingTestRepository struct {
	*loginTestRepository
	receiving        bool
	checks, controls int
}

func (r *pairingTestRepository) ChannelAccountReceiving(context.Context, string, string) (bool, error) {
	r.checks++
	return r.receiving, nil
}
func (r *pairingTestRepository) GetMessageChannel(_ context.Context, owner, workflow, id string) (ChannelStored, error) {
	if owner != "owner" || workflow != "workflow" || id != r.saved.Channel.ID {
		return ChannelStored{}, domain.ErrNotFound
	}
	return r.saved, nil
}
func (r *pairingTestRepository) ControlMessageChannel(context.Context, string, string, string, int64, string, string) (ChannelStored, error) {
	r.controls++
	return r.saved, nil
}

type pairingTestAccount struct{}

func (pairingTestAccount) Identify(_ context.Context, c ChannelCredentials, region string) (ChannelIdentity, error) {
	return ChannelIdentity{ID: "bot", Name: "Bot", TenantID: "tenant", BindingID: region + ":" + c["app_id"]}, nil
}

type pairingConnection struct {
	ctx     context.Context
	stored  ChannelStored
	receive ChannelMessageSink
	health  ChannelConnectionHealthSink
	fail    chan struct{}
}
type pairingTestReceiver struct{ connections chan pairingConnection }

func (*pairingTestReceiver) Configure(context.Context, ChannelStored, ChannelCredentials, string) error {
	return nil
}
func (r *pairingTestReceiver) Connect(ctx context.Context, s ChannelStored, c ChannelCredentials, sink ChannelMessageSink) error {
	return r.ConnectWithHealth(ctx, s, c, sink, nil)
}
func (r *pairingTestReceiver) ConnectWithHealth(ctx context.Context, s ChannelStored, c ChannelCredentials, sink ChannelMessageSink, health ChannelConnectionHealthSink) error {
	if c["app_secret"] != "secret" {
		return domain.ErrInvalid
	}
	connection := pairingConnection{ctx: ctx, stored: s, receive: sink, health: health, fail: make(chan struct{})}
	r.connections <- connection
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-connection.fail:
		return errors.New("private-provider-detail")
	}
}
func senderPairingFixture(t *testing.T) (*MessageChannels, *pairingTestRepository, *pairingTestReceiver, *transportTestSender) {
	t.Helper()
	app, repo, _ := loginTestFixture(t)
	r := &pairingTestRepository{loginTestRepository: repo}
	app.repository = r
	receiver := &pairingTestReceiver{connections: make(chan pairingConnection, 8)}
	sender := &transportTestSender{}
	app.transports["feishu"] = ChannelTransport{Account: pairingTestAccount{}, StreamReceiver: receiver, Sender: sender}
	lifetime, cancel := context.WithCancel(context.Background())
	app.SetPairingContext(lifetime)
	t.Cleanup(cancel)
	return app, r, receiver, sender
}
func pairLogin(t *testing.T, app *MessageChannels, region string) ChannelLogin {
	t.Helper()
	login, err := app.StartLogin(context.Background(), "owner", "workflow", "feishu", region, "credentials", "", 0, ChannelCredentials{"app_id": "app", "app_secret": "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return login
}
func pairConnection(t *testing.T, r *pairingTestReceiver) pairingConnection {
	t.Helper()
	select {
	case c := <-r.connections:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("pair receiver not started")
		return pairingConnection{}
	}
}
func pairPoll(t *testing.T, app *MessageChannels, id string) ChannelLogin {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		result, err := app.PollLogin(context.Background(), "owner", "workflow", id, "")
		if err == nil {
			return result
		}
		// PollLogin deliberately refuses overlapping access while callbacks update progress.
		if !errors.Is(err, domain.ErrConflict) || time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
}

func pairMessage(code, sender string) domain.ChannelMessage {
	return domain.ChannelMessage{EventID: "event", MessageID: "message", SenderID: sender, ChatID: "chat", Text: code, OccurredAt: time.Now().UTC()}
}
func pairDone(t *testing.T, entry *channelLoginSession) {
	t.Helper()
	select {
	case <-entry.pairing.done:
	case <-time.After(2 * time.Second):
		t.Fatal("temporary receiver was not released")
	}
}
func TestSenderPairingRequiresHandshakeThenRecognizesExactlyOneDirectSender(t *testing.T) {
	for _, region := range []string{"feishu", "lark"} {
		t.Run(region, func(t *testing.T) {
			app, repo, receiver, sender := senderPairingFixture(t)
			login := pairLogin(t, app, region)
			initial, err := app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if initial.PairingStatus != "connecting" || initial.PairingCode != "" {
				t.Fatal("code exposed before authenticated handshake")
			}
			conn := pairConnection(t, receiver)
			if conn.stored.Channel.TenantID != "tenant" || conn.stored.Channel.Region != region {
				t.Fatal("authenticated identity lost")
			}
			if err = conn.health(conn.ctx, "connected"); err != nil {
				t.Fatal(err)
			}
			waiting := pairPoll(t, app, login.ID)
			if waiting.PairingStatus != "waiting" || len(waiting.PairingCode) != 37 || !strings.HasPrefix(waiting.PairingCode, "pair ") {
				t.Fatal("missing bounded random code")
			}
			invalid := []domain.ChannelMessage{pairMessage("wrong code", "ou_other"), pairMessage(waiting.PairingCode, "bot"), pairMessage(waiting.PairingCode, "ou_other"), pairMessage(waiting.PairingCode, "ou_other"), pairMessage(waiting.PairingCode, "ou_other"), pairMessage(waiting.PairingCode, "ou_other")}
			invalid[2].Group = true
			invalid[3].Bot = true
			invalid[4].OccurredAt = time.Now().Add(-time.Minute)
			invalid[5].ChatID = ""
			for _, m := range invalid {
				if err := conn.receive(conn.ctx, m); err != nil {
					t.Fatal(err)
				}
			}
			if pairPoll(t, app, login.ID).SuggestedSenderID != "" {
				t.Fatal("non-pairing message authorized a sender")
			}
			_ = conn.health(conn.ctx, "connecting")
			if pairPoll(t, app, login.ID).PairingCode != "" {
				t.Fatal("disconnected receiver retained usable code")
			}
			_ = conn.health(conn.ctx, "connected")
			var wg sync.WaitGroup
			for _, id := range []string{"ou_alice", "ou_bob"} {
				wg.Add(1)
				go func(sender string) {
					defer wg.Done()
					_ = conn.receive(conn.ctx, pairMessage(waiting.PairingCode, sender))
				}(id)
			}
			wg.Wait()
			recognized := pairPoll(t, app, login.ID)
			if recognized.PairingStatus != "recognized" || recognized.PairingCode != "" || (recognized.SuggestedSenderID != "ou_alice" && recognized.SuggestedSenderID != "ou_bob") {
				t.Fatalf("recognition: %#v", recognized)
			}
			_ = conn.receive(conn.ctx, pairMessage(waiting.PairingCode, "ou_replay"))
			if pairPoll(t, app, login.ID).SuggestedSenderID != recognized.SuggestedSenderID {
				t.Fatal("code replay replaced sender")
			}
			entry, _ := app.findLogin("owner", "workflow", login.ID)
			pairDone(t, entry)
			if repo.saves != 0 || sender.calls != 0 {
				t.Fatal("pairing saved or sent without owner confirmation")
			}
			channel := domain.MessageChannel{Provider: "feishu", Region: region, Name: "Feishu", Audience: domain.ChannelAudience{SenderIDs: []string{recognized.SuggestedSenderID}, AllowDirect: true}}
			if _, err = app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, channel, login.ID); err != nil {
				t.Fatal(err)
			}
			if repo.saves != 1 {
				t.Fatal("owner confirmation was not saved")
			}
		})
	}
}
func TestSenderPairingCancellationExpiryFailureAndShutdownReleaseConnection(t *testing.T) {
	for _, action := range []string{"cancel", "expiry", "failure", "shutdown", "save"} {
		t.Run(action, func(t *testing.T) {
			app, _, receiver, _ := senderPairingFixture(t)
			lifetime, cancel := context.WithCancel(context.Background())
			defer cancel()
			app.SetPairingContext(lifetime)
			login := pairLogin(t, app, "feishu")
			entry, _ := app.findLogin("owner", "workflow", login.ID)
			if action == "expiry" {
				entry.mu.Lock()
				entry.public.ExpiresAt = time.Now().Add(50 * time.Millisecond)
				entry.mu.Unlock()
			}
			_, err := app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0)
			if err != nil {
				t.Fatal(err)
			}
			conn := pairConnection(t, receiver)
			_ = conn.health(conn.ctx, "connected")
			switch action {
			case "cancel":
				if err = app.CancelLogin(context.Background(), "owner", "workflow", login.ID); err != nil {
					t.Fatal(err)
				}
			case "failure":
				close(conn.fail)
			case "shutdown":
				cancel()
			case "save":
				_, err = app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, domain.MessageChannel{Provider: "feishu", Region: "feishu", Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"ou_manual"}, AllowDirect: true}}, login.ID)
				if err != nil {
					t.Fatal(err)
				}
			}
			pairDone(t, entry)
			app.pairingMu.Lock()
			count := len(app.pairings)
			app.pairingMu.Unlock()
			if count != 0 {
				t.Fatal("pairing capacity leaked")
			}
			if action == "failure" || action == "shutdown" || action == "expiry" {
				result := pairPoll(t, app, login.ID)
				if result.PairingCode != "" || result.SuggestedSenderID != "" {
					t.Fatal("dead pair retained code or sender")
				}
			}
		})
	}
}
func TestSenderPairingRejectsForeignScopeAndCompetingReceivers(t *testing.T) {
	app, repo, receiver, _ := senderPairingFixture(t)
	login := pairLogin(t, app, "feishu")
	for _, args := range []struct {
		owner, workflow, login, channel string
		version                         int64
	}{{"other", "workflow", login.ID, "", 0}, {"owner", "other", login.ID, "", 0}, {"owner", "workflow", login.ID, "injected", 0}, {"owner", "workflow", login.ID, "", 1}} {
		if _, err := app.StartSenderPairing(context.Background(), args.owner, args.workflow, args.login, args.channel, args.version); err == nil {
			t.Fatal("pair crossed owner/workflow/configuration scope")
		}
	}
	repo.receiving = true
	if _, err := app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("active receiver accepted")
	}
	repo.receiving = false
	_, err := app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	conn := pairConnection(t, receiver)
	_ = conn.health(conn.ctx, "connected")
	second := pairLogin(t, app, "feishu")
	if _, err = app.StartSenderPairing(context.Background(), "owner", "workflow", second.ID, "", 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("same app took a second receiver")
	}
	repo.saved.Channel = domain.MessageChannel{ID: "channel", Provider: "feishu", BindingID: "feishu:app", Version: 1}
	if _, err = app.Control(context.Background(), "owner", "workflow", "channel", 1, "enable"); !errors.Is(err, domain.ErrConflict) || repo.controls != 0 {
		t.Fatal("permanent reception started during pairing")
	}
	if _, err = app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("active nonce replaced")
	}
}
func TestSenderPairingSavedChannelAndCodeRotation(t *testing.T) {
	app, repo, receiver, _ := senderPairingFixture(t)
	login := pairLogin(t, app, "feishu")
	channel := domain.MessageChannel{Provider: "feishu", Region: "feishu", Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"ou_manual"}, AllowDirect: true}}
	saved, err := app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, channel, login.ID)
	if err != nil {
		t.Fatal(err)
	}
	repo.saved.Channel.Version = 3
	if _, err = app.StartSenderPairing(context.Background(), "owner", "workflow", "", saved.ID, 2); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale saved channel accepted")
	}
	pair, err := app.StartSenderPairing(context.Background(), "owner", "workflow", "", saved.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	conn := pairConnection(t, receiver)
	_ = conn.health(conn.ctx, "connected")
	first := pairPoll(t, app, pair.ID)
	_ = conn.receive(conn.ctx, pairMessage(first.PairingCode, "ou_first"))
	entry, _ := app.findLogin("owner", "workflow", pair.ID)
	pairDone(t, entry)
	rotated, err := app.StartSenderPairing(context.Background(), "owner", "workflow", pair.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	next := pairConnection(t, receiver)
	_ = next.health(next.ctx, "connected")
	second := pairPoll(t, app, rotated.ID)
	if first.PairingCode == second.PairingCode {
		t.Fatal("nonce reused")
	}
	_ = next.receive(next.ctx, pairMessage(first.PairingCode, "ou_replay"))
	if pairPoll(t, app, pair.ID).SuggestedSenderID != "" {
		t.Fatal("old code survived rotation")
	}
}

func TestSenderPairingCancelledRequestDoesNotReserveReceiver(t *testing.T) {
	app, _, receiver, _ := senderPairingFixture(t)
	login := pairLogin(t, app, "feishu")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := app.StartSenderPairing(ctx, "owner", "workflow", login.ID, "", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
	if len(app.pairings) != 0 || len(receiver.connections) != 0 {
		t.Fatal("cancelled request started pairing")
	}
}
func TestSenderPairingSaveRejectsChangedAuthenticatedIdentity(t *testing.T) {
	app, repo, _, _ := senderPairingFixture(t)
	login := pairLogin(t, app, "feishu")
	entry, _ := app.findLogin("owner", "workflow", login.ID)
	entry.identity.TenantID = "previous-tenant"
	_, err := app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, domain.MessageChannel{Provider: "feishu", Region: "feishu", Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"ou_confirmed"}, AllowDirect: true}}, login.ID)
	if !errors.Is(err, domain.ErrConflict) || repo.saves != 0 {
		t.Fatal("sender saved against changed account identity")
	}
}

func TestSenderPairingGlobalConnectionLimit(t *testing.T) {
	app, _, _, _ := senderPairingFixture(t)
	for i := 0; i < 16; i++ {
		id := strings.Repeat("x", i+1)
		if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: id, binding: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: "overflow", binding: "overflow"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("global receiver limit bypassed")
	}
}
