package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"agent-platform/backend/internal/secretcrypto"
)

type loginTestRepository struct {
	MessageChannelRepository
	saved ChannelStored
	saves int
}

func (r *loginTestRepository) ListMessageChannels(_ context.Context, owner, workflow string) ([]domain.MessageChannel, error) {
	if owner != "owner" || workflow != "workflow" {
		return nil, domain.ErrNotFound
	}
	return nil, nil
}
func (r *loginTestRepository) SaveMessageChannel(_ context.Context, c ChannelStored, _ int64) (ChannelStored, error) {
	r.saved = c
	r.saves++
	return c, nil
}

type loginTestAccount struct {
	transportTestAccount
	calls int
}

func (a *loginTestAccount) StartLogin(context.Context) (ChannelLoginStep, error) {
	return ChannelLoginStep{Status: "waiting", QRContent: "https://provider.test/qr", State: map[string]string{"key": "temporary-secret"}}, nil
}
func (a *loginTestAccount) PollLogin(_ context.Context, state map[string]string, _ string) (ChannelLoginStep, error) {
	if state["key"] != "temporary-secret" {
		return ChannelLoginStep{}, domain.ErrInvalid
	}
	a.calls++
	return ChannelLoginStep{Status: "connected", Credentials: ChannelCredentials{"bot_token": "bot-secret"}, SuggestedSenderID: "alice"}, nil
}
func loginTestFixture(t *testing.T) (*MessageChannels, *loginTestRepository, *loginTestAccount) {
	t.Helper()
	box, err := secretcrypto.New(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	repo := &loginTestRepository{}
	account := &loginTestAccount{}
	app := NewMessageChannels(repo, box, map[string]ChannelTransport{"qr": {Account: account, WebhookReceiver: &transportTestWebhook{}, Sender: &transportTestSender{}}, "manual": {Account: transportTestAccount{}, WebhookReceiver: &transportTestWebhook{}, Sender: &transportTestSender{}}}, true, "https://workspace.test")
	t.Cleanup(func() {
		app.loginMu.Lock()
		entries := []*channelLoginSession{}
		for _, l := range app.logins {
			entries = append(entries, l)
		}
		app.loginMu.Unlock()
		for _, l := range entries {
			l.mu.Lock()
			app.removeLogin(l)
			l.mu.Unlock()
		}
	})
	return app, repo, account
}
func TestChannelQRLoginCredentialsStayServerSideAndAreConsumedOnce(t *testing.T) {
	app, repo, account := loginTestFixture(t)
	ctx := context.Background()
	login, err := app.StartLogin(ctx, "owner", "workflow", "qr", "", "qr", "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := app.findLogin("owner", "workflow", login.ID)
	if strings.Contains(string(entry.ciphertext), "temporary-secret") {
		t.Fatal("unencrypted provider state")
	}
	connected, err := app.PollLogin(ctx, "owner", "workflow", login.ID, "")
	if err != nil || connected.Status != "connected" || connected.SuggestedSenderID != "alice" {
		t.Fatalf("login not connected: %v", err)
	}
	public, _ := json.Marshal(connected)
	if strings.Contains(string(public), "bot-secret") || connected.QRContent != "" {
		t.Fatal("credentials or QR retained in public response")
	}
	c := domain.MessageChannel{Provider: "qr", Name: "Bot", Audience: domain.ChannelAudience{SenderIDs: []string{"alice"}, AllowDirect: true}}
	for _, bad := range []struct{ owner, workflow, provider string }{{"other", "workflow", "qr"}, {"owner", "other", "qr"}, {"owner", "workflow", "manual"}} {
		input := c
		input.Provider = bad.provider
		if _, err = app.SaveWithLogin(ctx, bad.owner, bad.workflow, "", 0, input, login.ID); err == nil {
			t.Fatal("login crossed scope")
		}
	}
	if _, err = app.SaveWithLogin(ctx, "owner", "workflow", "", 0, c, login.ID); err != nil {
		t.Fatal(err)
	}
	if repo.saves != 1 || account.calls != 1 {
		t.Fatal("unexpected credential use")
	}
	credentials, err := app.credentials(repo.saved)
	if err != nil || credentials["bot_token"] != "bot-secret" {
		t.Fatal("credential not encrypted into channel")
	}
	if _, err = app.SaveWithLogin(ctx, "owner", "workflow", "", 0, c, login.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("login replay accepted")
	}
}
func TestChannelLoginCancelledExpiredAndUnauthorizedCannotPollOrSave(t *testing.T) {
	for _, action := range []string{"cancel", "expire"} {
		t.Run(action, func(t *testing.T) {
			app, repo, account := loginTestFixture(t)
			ctx := context.Background()
			login, err := app.StartLogin(ctx, "owner", "workflow", "qr", "", "qr", "", 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = app.PollLogin(ctx, "other", "workflow", login.ID, ""); !errors.Is(err, domain.ErrNotFound) {
				t.Fatal("foreign owner saw login")
			}
			if action == "cancel" {
				if err = app.CancelLogin(ctx, "owner", "workflow", login.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				entry, _ := app.findLogin("owner", "workflow", login.ID)
				entry.public.ExpiresAt = time.Now().Add(-time.Second)
			}
			result, err := app.PollLogin(ctx, "owner", "workflow", login.ID, "")
			if action == "cancel" && !errors.Is(err, domain.ErrNotFound) || action == "expire" && (err != nil || result.Status != "expired" || result.QRContent != "") {
				t.Fatal("login remained live")
			}
			if repo.saves != 0 || account.calls != 0 {
				t.Fatal("expired or foreign login reached provider/storage")
			}
		})
	}
}
func TestManualChannelLoginUsesRealIdentitySeamAndBoundsSessions(t *testing.T) {
	app, _, _ := loginTestFixture(t)
	ctx := context.Background()
	if _, err := app.StartLogin(ctx, "owner", "workflow", "manual", "", "qr", "", 0, nil); err == nil {
		t.Fatal("invented QR capability")
	}
	for i := 0; i < 4; i++ {
		login, err := app.StartLogin(ctx, "owner", "workflow", "manual", "", "credentials", "", 0, ChannelCredentials{"token": "private-token"})
		if err != nil || login.Status != "connected" || login.AccountID != "bot" {
			t.Fatal("identity not verified")
		}
	}
	if _, err := app.StartLogin(ctx, "owner", "workflow", "manual", "", "credentials", "", 0, ChannelCredentials{"token": "private-token"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("unbounded login sessions")
	}
}
