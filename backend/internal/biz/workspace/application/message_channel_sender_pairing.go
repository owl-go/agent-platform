package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

// This read-only query must not take the Worker claim lock or return credentials.
type ChannelSenderPairingRepository interface {
	ChannelAccountReceiving(context.Context, string, string) (bool, error)
}

type channelSenderPairing struct {
	id, provider, binding string
	started, expires      time.Time
	cancel                context.CancelFunc
	done                  chan struct{}
}

// SetPairingContext binds temporary receivers to the API process lifetime.
// Call once during construction, before serving requests.
func (s *MessageChannels) SetPairingContext(ctx context.Context) { s.pairingContext = ctx }

// StartSenderPairing uses an authenticated login or the owner's disabled saved
// channel. Credentials never leave the server. The exact one-use message only
// identifies a candidate; saving the audience remains the owner's confirmation.
func (s *MessageChannels) StartSenderPairing(ctx context.Context, owner, workflow, loginID, channelID string, version int64) (result ChannelLogin, resultErr error) {
	if err := s.loginAccess(ctx, owner, workflow); err != nil {
		return result, err
	}
	created := false
	if loginID == "" {
		if channelID == "" {
			return result, domain.ErrInvalid
		}
		stored, err := s.repository.GetMessageChannel(ctx, owner, workflow, channelID)
		if err != nil {
			return result, err
		}
		if !supportsSenderPairing(stored.Channel.Provider) || stored.Channel.Enabled || stored.Channel.Version != version {
			return result, domain.ErrConflict
		}
		credentials, err := s.credentials(stored)
		if err != nil {
			return result, err
		}
		login, err := s.StartLogin(ctx, owner, workflow, stored.Channel.Provider, stored.Channel.Region, "credentials", channelID, version, credentials)
		clear(credentials)
		if err != nil {
			return result, err
		}
		loginID, created = login.ID, true
	} else if channelID != "" || version != 0 {
		return result, domain.ErrInvalid
	}
	l, err := s.findLogin(owner, workflow, loginID)
	if err != nil {
		return result, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	defer func() {
		if created && resultErr != nil {
			s.removeLogin(l)
		}
	}()
	now := time.Now().UTC()
	if !supportsSenderPairing(l.public.Provider) || l.public.Status != "connected" || !now.Before(l.public.ExpiresAt) || l.identity.BindingID == "" || (l.public.Provider == "feishu" && l.identity.TenantID == "") {
		return result, domain.ErrConflict
	}
	receiver, ok := s.transports[l.public.Provider].StreamReceiver.(ChannelStreamHealthReceiver)
	if !ok {
		return result, domain.ErrInvalid
	}
	if l.pairing != nil {
		select {
		case <-l.pairing.done:
		default:
			return result, domain.ErrConflict
		}
	}
	state, err := s.openLogin(l)
	if err != nil {
		return result, err
	}
	tenant := l.identity.TenantID
	if l.public.Provider == "dingtalk" && l.channel != "" {
		saved, err := s.repository.GetMessageChannel(ctx, owner, workflow, l.channel)
		if err != nil {
			clear(state.Credentials)
			return result, err
		}
		if saved.Channel.Version != l.version || saved.Channel.Enabled {
			clear(state.Credentials)
			return result, domain.ErrConflict
		}
		if saved.Channel.AccountID == l.identity.ID && saved.Channel.BindingID == l.identity.BindingID {
			// Retained accounts keep their enterprise boundary during pairing.
			// A newly authenticated app remains unpinned until receive/reply validation.
			tenant = saved.Channel.TenantID
		}
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		clear(state.Credentials)
		return result, err
	}
	expires := now.Add(2 * time.Minute)
	if expires.After(l.public.ExpiresAt) {
		expires = l.public.ExpiresAt
	}
	pairCtx, cancel := context.WithDeadline(s.pairingContext, expires)
	p := &channelSenderPairing{id: uuid.NewString(), provider: l.public.Provider, binding: l.identity.BindingID, started: now, expires: expires, cancel: cancel, done: make(chan struct{})}
	if err := s.reserveSenderPairing(ctx, p); err != nil {
		cancel()
		clear(state.Credentials)
		return result, err
	}
	l.pairing = p
	l.public.PairingCode, l.public.PairingStatus = "", "connecting"
	l.public.PairingExpiresAt = &p.expires
	l.public.SuggestedSenderID = ""
	code := "pair " + hex.EncodeToString(nonce)
	stored := ChannelStored{Channel: domain.MessageChannel{OwnerID: owner, WorkflowID: workflow, Provider: l.public.Provider, Region: l.region, AccountID: l.identity.ID, TenantID: tenant, BindingID: l.identity.BindingID}}
	go s.receiveSenderPairing(pairCtx, l, p, receiver, stored, state.Credentials, code)
	return l.public, nil
}

func supportsSenderPairing(provider string) bool {
	return provider == "feishu" || provider == "dingtalk" || provider == "wecom"
}
func senderPairingBinding(provider, binding string) string {
	return provider + ":" + binding
}

func (s *MessageChannels) reserveSenderPairing(ctx context.Context, p *channelSenderPairing) error {
	if !supportsSenderPairing(p.provider) || p.binding == "" || p.id == "" {
		return domain.ErrInvalid
	}
	s.pairingMu.Lock()
	defer s.pairingMu.Unlock()
	// Existing login quotas additionally bound this to four per User.
	if len(s.pairings) >= 16 || s.pairings[senderPairingBinding(p.provider, p.binding)] != "" {
		return domain.ErrConflict
	}
	repository, ok := s.repository.(ChannelSenderPairingRepository)
	if !ok {
		return domain.ErrInvalid
	}
	receiving, err := repository.ChannelAccountReceiving(ctx, p.provider, p.binding)
	if err != nil {
		return err
	}
	if receiving {
		return domain.ErrConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.pairings[senderPairingBinding(p.provider, p.binding)] = p.id
	return nil
}

func (s *MessageChannels) receiveSenderPairing(ctx context.Context, l *channelLoginSession, p *channelSenderPairing, receiver ChannelStreamHealthReceiver, stored ChannelStored, credentials ChannelCredentials, code string) {
	defer clear(credentials)
	defer p.cancel()
	defer func() {
		s.pairingMu.Lock()
		if s.pairings[senderPairingBinding(p.provider, p.binding)] == p.id {
			delete(s.pairings, senderPairingBinding(p.provider, p.binding))
		}
		s.pairingMu.Unlock()
		l.mu.Lock()
		if l.pairing == p {
			if l.public.PairingStatus == "connecting" || l.public.PairingStatus == "waiting" {
				l.public.PairingStatus = "failed"
				if !time.Now().Before(p.expires) {
					l.public.PairingStatus = "expired"
				}
			}
			l.public.PairingCode = ""
		}
		l.mu.Unlock()
		close(p.done)
	}()
	// This sink never calls Receive, Inbox, a sender, or a Runtime.
	_ = receiver.ConnectWithHealth(ctx, stored, credentials, func(_ context.Context, message domain.ChannelMessage) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		now := time.Now().UTC()
		if stored.Channel.Provider == "dingtalk" && (message.TenantID == "" || (stored.Channel.TenantID != "" && message.TenantID != stored.Channel.TenantID)) {
			return nil
		}
		if l.pairing != p || ctx.Err() != nil || l.public.Status != "connected" || l.public.PairingStatus != "waiting" || !now.Before(p.expires) || message.Validate(now) != nil || message.Bot || message.Group || message.SenderID == stored.Channel.AccountID || message.SenderID == "" || message.Text != code || message.EventID == "" || message.MessageID == "" || message.OccurredAt.Before(p.started.Add(-5*time.Second)) || message.OccurredAt.After(now.Add(30*time.Second)) {
			return nil
		}
		l.public.SuggestedSenderID = message.SenderID
		l.public.PairingStatus, l.public.PairingCode = "recognized", ""
		p.cancel()
		return nil
	}, func(_ context.Context, health string) error {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.pairing == p && ctx.Err() == nil {
			if l.public.PairingStatus == "connecting" && health == "connected" {
				l.public.PairingStatus, l.public.PairingCode = "waiting", code
			}
			if l.public.PairingStatus == "waiting" && health == "connecting" {
				l.public.PairingStatus, l.public.PairingCode = "connecting", ""
			}
		}
		return nil
	})
}
