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
	checkedProvider  string
}

func (r *pairingTestRepository) ChannelAccountReceiving(_ context.Context, provider, _ string) (bool, error) {
	r.checks++
	r.checkedProvider = provider
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
	if c["client_id"] != "" {
		return ChannelIdentity{ID: c["client_id"], Name: "Bot", BindingID: c["client_id"]}, nil
	}
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
	if c["app_secret"] != "secret" && c["client_secret"] != "secret" {
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
	app.transports["dingtalk"] = ChannelTransport{Account: pairingTestAccount{}, StreamReceiver: receiver, Sender: sender}
	lifetime, cancel := context.WithCancel(context.Background())
	app.SetPairingContext(lifetime)
	t.Cleanup(cancel)
	return app, r, receiver, sender
}
func pairLogin(t *testing.T, app *MessageChannels, region string) ChannelLogin {
	t.Helper()
	provider := "feishu"
	credentials := ChannelCredentials{"app_id": "app", "app_secret": "secret"}
	if region == "dingtalk" {
		provider, region = "dingtalk", ""
		credentials = ChannelCredentials{"client_id": "app", "client_secret": "secret"}
	}
	login, err := app.StartLogin(context.Background(), "owner", "workflow", provider, region, "credentials", "", 0, credentials)
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
	return domain.ChannelMessage{EventID: "event", MessageID: "message", SenderID: sender, TenantID: "tenant", ChatID: "chat", Text: code, OccurredAt: time.Now().UTC()}
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
	for _, region := range []string{"feishu", "lark", "dingtalk"} {
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
			expectedTenant, expectedRegion := "tenant", region
			if region == "dingtalk" {
				expectedTenant, expectedRegion = "", ""
			}
			if conn.stored.Channel.TenantID != expectedTenant || conn.stored.Channel.Region != expectedRegion || repo.checkedProvider != conn.stored.Channel.Provider {
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
			invalid[1].SenderID = conn.stored.Channel.AccountID
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
			channel := domain.MessageChannel{Provider: conn.stored.Channel.Provider, Region: conn.stored.Channel.Region, Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{recognized.SuggestedSenderID}, AllowDirect: true}}
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
	for _, provider := range []string{"feishu", "dingtalk"} {
		t.Run(provider, func(t *testing.T) {
			for _, action := range []string{"cancel", "expiry", "failure", "shutdown", "save"} {
				t.Run(action, func(t *testing.T) {
					app, _, receiver, _ := senderPairingFixture(t)
					lifetime, cancel := context.WithCancel(context.Background())
					defer cancel()
					app.SetPairingContext(lifetime)
					login := pairLogin(t, app, provider)
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
						_, err = app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, domain.MessageChannel{Provider: login.Provider, Region: entry.region, Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"ou_manual"}, AllowDirect: true}}, login.ID)
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
		if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: id, provider: "feishu", binding: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: "overflow", provider: "feishu", binding: "overflow"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("global receiver limit bypassed")
	}
}

func TestDingTalkSenderPairingKeepsSavedEnterpriseAndRequiresExplicitAudienceSave(t *testing.T) {
	app, repo, receiver, sender := senderPairingFixture(t)
	login := pairLogin(t, app, "dingtalk")
	channel := domain.MessageChannel{Provider: "dingtalk", Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"staff_existing"}, AllowDirect: true}}
	saved, err := app.SaveWithLogin(context.Background(), "owner", "workflow", "", 0, channel, login.ID)
	if err != nil {
		t.Fatal(err)
	}
	repo.saved.Channel.TenantID = "tenant"
	pair, err := app.StartSenderPairing(context.Background(), "owner", "workflow", "", saved.ID, saved.Version)
	if err != nil {
		t.Fatal(err)
	}
	conn := pairConnection(t, receiver)
	if conn.stored.Channel.Provider != "dingtalk" || conn.stored.Channel.TenantID != "tenant" || repo.checkedProvider != "dingtalk" {
		t.Fatal("saved enterprise or provider boundary was lost")
	}
	_ = conn.health(conn.ctx, "connected")
	waiting := pairPoll(t, app, pair.ID)
	for _, tenant := range []string{"", "other-enterprise"} {
		message := pairMessage(waiting.PairingCode, "staff_other")
		message.TenantID = tenant
		_ = conn.receive(conn.ctx, message)
	}
	if pairPoll(t, app, pair.ID).SuggestedSenderID != "" {
		t.Fatal("unverified enterprise suggested a Staff ID")
	}
	_ = conn.receive(conn.ctx, pairMessage(waiting.PairingCode, "staff_confirmed"))
	entry, _ := app.findLogin("owner", "workflow", pair.ID)
	pairDone(t, entry)
	recognized := pairPoll(t, app, pair.ID)
	if recognized.PairingStatus != "recognized" || recognized.SuggestedSenderID != "staff_confirmed" || sender.calls != 0 || repo.saves != 1 {
		t.Fatal("recognition sent a message, saved an audience, or lost Staff ID")
	}
	// Pairing does not substitute for receive-and-reply validation or pin a new
	// enterprise identity that the credentials-only Identify cannot verify.
	channel.Audience.SenderIDs = []string{"staff_existing", recognized.SuggestedSenderID}
	if _, err = app.SaveWithLogin(context.Background(), "owner", "workflow", saved.ID, saved.Version, channel, pair.ID); err != nil {
		t.Fatal(err)
	}
	if repo.saves != 2 || repo.saved.Channel.Enabled || repo.saved.Channel.ValidationState != "unverified" || repo.saved.Channel.TenantID != "" {
		t.Fatal("pairing bypassed receive-and-reply validation")
	}
}

func TestDingTalkSenderPairingRequiresEnterpriseOnNewAccount(t *testing.T) {
	app, _, receiver, _ := senderPairingFixture(t)
	login := pairLogin(t, app, "dingtalk")
	_, err := app.StartSenderPairing(context.Background(), "owner", "workflow", login.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	conn := pairConnection(t, receiver)
	_ = conn.health(conn.ctx, "connected")
	waiting := pairPoll(t, app, login.ID)
	message := pairMessage(waiting.PairingCode, "staff")
	message.TenantID = ""
	_ = conn.receive(conn.ctx, message)
	if pairPoll(t, app, login.ID).SuggestedSenderID != "" {
		t.Fatal("missing authenticated enterprise accepted")
	}
	message.TenantID = "tenant"
	_ = conn.receive(conn.ctx, message)
	if pairPoll(t, app, login.ID).SuggestedSenderID != "staff" {
		t.Fatal("authenticated Staff ID was not recognized")
	}
}

func TestSenderPairingBindingAndReceptionAdmissionAreProviderScoped(t *testing.T) {
	app, repo, _, _ := senderPairingFixture(t)
	for _, provider := range []string{"feishu", "dingtalk"} {
		p := &channelSenderPairing{id: provider, provider: provider, binding: "shared-binding"}
		if err := app.reserveSenderPairing(context.Background(), p); err != nil || repo.checkedProvider != provider {
			t.Fatalf("provider scope %s: %v", provider, err)
		}
		repo.saved.Channel = domain.MessageChannel{ID: "channel", Provider: provider, BindingID: "shared-binding"}
		if _, err := app.Control(context.Background(), "owner", "workflow", "channel", 0, "enable"); !errors.Is(err, domain.ErrConflict) || repo.controls != 0 {
			t.Fatal("permanent reception competed with provider pairing")
		}
	}
	if len(app.pairings) != 2 {
		t.Fatal("different providers collided on a binding")
	}
	if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: "again", provider: "dingtalk", binding: "shared-binding"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("duplicate provider receiver accepted")
	}
	repo.receiving = true
	if err := app.reserveSenderPairing(context.Background(), &channelSenderPairing{id: "active", provider: "dingtalk", binding: "active-binding"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("active DingTalk reception accepted pairing")
	}
}
