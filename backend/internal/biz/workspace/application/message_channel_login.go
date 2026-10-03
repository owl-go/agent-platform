package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

// Login is optional on an Account. It is separate from message reception/sending.
// State and Credentials stay encrypted on the server; only public progress is returned.
type ChannelQRLogin interface {
	StartLogin(context.Context) (ChannelLoginStep, error)
	PollLogin(context.Context, map[string]string, string) (ChannelLoginStep, error)
}
type ChannelLoginStep struct {
	Status, QRContent, SuggestedSenderID string
	State                                map[string]string
	Credentials                          ChannelCredentials
}
type ChannelLogin struct {
	ID, Provider, Status, QRContent, AccountID, AccountName, SuggestedSenderID string
	ExpiresAt                                                                  time.Time
}
type channelLoginSession struct {
	mu                               sync.Mutex
	owner, workflow, region, channel string
	version                          int64
	public                           ChannelLogin
	ciphertext                       []byte
	expiry                           *time.Timer
}
type channelLoginPrivate struct {
	State       map[string]string
	Credentials ChannelCredentials
}

func (l *channelLoginSession) aad() string {
	return "channel-login:" + l.owner + ":" + l.workflow + ":" + l.public.ID
}
func (s *MessageChannels) sealLogin(l *channelLoginSession, state channelLoginPrivate) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	l.ciphertext, err = s.cipher.Encrypt(data, l.aad())
	return err
}
func (s *MessageChannels) openLogin(l *channelLoginSession) (channelLoginPrivate, error) {
	var state channelLoginPrivate
	data, err := s.cipher.Decrypt(l.ciphertext, l.aad())
	if err != nil {
		return state, domain.ErrInvalid
	}
	if json.Unmarshal(data, &state) != nil {
		return state, domain.ErrInvalid
	}
	return state, nil
}
func validLoginCredentials(c ChannelCredentials) bool {
	if len(c) > 24 {
		return false
	}
	for k, v := range c {
		if len(k) > 64 || len(v) > 4096 || strings.ContainsAny(v, "\r\n\x00") {
			return false
		}
	}
	return true
}
func (s *MessageChannels) loginAccess(ctx context.Context, owner, workflow string) error {
	if !s.enabled || s.cipher == nil {
		return domain.ErrInvalid
	}
	_, err := s.repository.ListMessageChannels(ctx, owner, workflow)
	return err
}
func (s *MessageChannels) StartLogin(ctx context.Context, owner, workflow, provider, region, method, channel string, version int64, credentials ChannelCredentials) (ChannelLogin, error) {
	if err := s.loginAccess(ctx, owner, workflow); err != nil {
		return ChannelLogin{}, err
	}
	transport, ok := s.transports[provider]
	if !ok || !transport.complete() || !validLoginCredentials(credentials) || (region != "" && region != "feishu" && region != "lark") || (provider != "feishu" && region != "") {
		return ChannelLogin{}, domain.ErrInvalid
	}
	if channel != "" {
		old, err := s.repository.GetMessageChannel(ctx, owner, workflow, channel)
		if err != nil {
			return ChannelLogin{}, err
		}
		if old.Channel.Provider != provider || old.Channel.Version != version || old.Channel.Enabled {
			return ChannelLogin{}, domain.ErrConflict
		}
	} else if version != 0 {
		return ChannelLogin{}, domain.ErrInvalid
	}
	if method != "credentials" && method != "qr" {
		return ChannelLogin{}, domain.ErrInvalid
	}
	qr, qrOK := transport.Account.(ChannelQRLogin)
	if method == "qr" && (!qrOK || len(credentials) != 0) {
		return ChannelLogin{}, domain.ErrInvalid
	}
	l := &channelLoginSession{owner: owner, workflow: workflow, region: region, channel: channel, version: version, public: ChannelLogin{ID: uuid.NewString(), Provider: provider, Status: "waiting", ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}}
	s.loginMu.Lock()
	ownerCount := 0
	for id, entry := range s.logins {
		if time.Now().After(entry.public.ExpiresAt) {
			delete(s.logins, id)
		} else if entry.owner == owner {
			ownerCount++
		}
	}
	if len(s.logins) >= 512 || ownerCount >= 4 {
		s.loginMu.Unlock()
		return ChannelLogin{}, domain.ErrConflict
	}
	// Reserve capacity before external requests. Expiry/removal never logs secrets.
	s.logins[l.public.ID] = l
	s.loginMu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	var err error
	if method == "qr" {
		var step ChannelLoginStep
		step, err = qr.StartLogin(ctx)
		if err == nil && (step.Status != "waiting" || step.QRContent == "" || len(step.QRContent) > 4096) {
			err = domain.ErrInvalid
		}
		if err == nil {
			l.public.QRContent = step.QRContent
			err = s.sealLogin(l, channelLoginPrivate{State: step.State})
		}
	} else {
		err = s.completeLogin(ctx, l, credentials, "")
	}
	if err != nil {
		s.removeLogin(l)
		return ChannelLogin{}, err
	}
	l.expiry = time.AfterFunc(time.Until(l.public.ExpiresAt), func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.public.Status = "expired"
		s.removeLogin(l)
	})
	return l.public, nil
}
func (s *MessageChannels) completeLogin(ctx context.Context, l *channelLoginSession, c ChannelCredentials, sender string) error {
	if !validLoginCredentials(c) || len(c) == 0 {
		return domain.ErrInvalid
	}
	identity, err := s.transports[l.public.Provider].Account.Identify(ctx, c, l.region)
	if err != nil {
		return err
	}
	if identity.ID == "" {
		return domain.ErrInvalid
	}
	if err = s.sealLogin(l, channelLoginPrivate{Credentials: c}); err != nil {
		return err
	}
	l.public.Status, l.public.AccountID, l.public.AccountName = "connected", identity.ID, identity.Name
	l.public.SuggestedSenderID, l.public.QRContent = sender, ""
	return nil
}
func (s *MessageChannels) findLogin(owner, workflow, id string) (*channelLoginSession, error) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	l := s.logins[id]
	if l == nil || l.owner != owner || l.workflow != workflow {
		return nil, domain.ErrNotFound
	}
	return l, nil
}
func (s *MessageChannels) removeLogin(l *channelLoginSession) {
	if l.expiry != nil {
		l.expiry.Stop()
	}
	s.loginMu.Lock()
	delete(s.logins, l.public.ID)
	s.loginMu.Unlock()
	l.ciphertext = nil
	l.public.QRContent = ""
}
func (s *MessageChannels) PollLogin(ctx context.Context, owner, workflow, id, code string) (ChannelLogin, error) {
	if err := s.loginAccess(ctx, owner, workflow); err != nil {
		return ChannelLogin{}, err
	}
	if len(code) > 32 || strings.ContainsAny(code, "\r\n\x00") {
		return ChannelLogin{}, domain.ErrInvalid
	}
	l, err := s.findLogin(owner, workflow, id)
	if err != nil {
		return ChannelLogin{}, err
	}
	if !l.mu.TryLock() {
		return ChannelLogin{}, domain.ErrConflict
	}
	defer l.mu.Unlock()
	if time.Now().After(l.public.ExpiresAt) {
		l.public.Status = "expired"
		s.removeLogin(l)
		return l.public, nil
	}
	if l.public.Status == "connected" || l.public.Status == "failed" || l.public.Status == "expired" {
		return l.public, nil
	}
	state, err := s.openLogin(l)
	if err != nil {
		return ChannelLogin{}, err
	}
	qr, ok := s.transports[l.public.Provider].Account.(ChannelQRLogin)
	if !ok {
		return ChannelLogin{}, domain.ErrInvalid
	}
	step, err := qr.PollLogin(ctx, state.State, code)
	if err != nil {
		return ChannelLogin{}, err
	}
	if time.Now().After(l.public.ExpiresAt) {
		l.public.Status = "expired"
		s.removeLogin(l)
		return l.public, nil
	}
	switch step.Status {
	case "connected":
		if err = s.completeLogin(ctx, l, step.Credentials, step.SuggestedSenderID); err != nil {
			return ChannelLogin{}, err
		}
	case "waiting", "scanned", "verification_required":
		l.public.Status = step.Status
		if err = s.sealLogin(l, channelLoginPrivate{State: step.State}); err != nil {
			return ChannelLogin{}, err
		}
	case "expired", "failed":
		l.public.Status = step.Status
		l.ciphertext = nil
		l.public.QRContent = ""
	default:
		return ChannelLogin{}, domain.ErrInvalid
	}
	return l.public, nil
}
func (s *MessageChannels) CancelLogin(ctx context.Context, owner, workflow, id string) error {
	if err := s.loginAccess(ctx, owner, workflow); err != nil {
		return err
	}
	l, err := s.findLogin(owner, workflow, id)
	if err != nil {
		return err
	}
	if !l.mu.TryLock() {
		return domain.ErrConflict
	}
	defer l.mu.Unlock()
	l.public.Status = "expired"
	s.removeLogin(l)
	return nil
}
func (s *MessageChannels) SaveWithLogin(ctx context.Context, owner, workflow, id string, version int64, c domain.MessageChannel, login string) (domain.MessageChannel, error) {
	if err := s.loginAccess(ctx, owner, workflow); err != nil {
		return c, err
	}
	l, err := s.findLogin(owner, workflow, login)
	if err != nil {
		return c, err
	}
	if !l.mu.TryLock() {
		return c, domain.ErrConflict
	}
	defer l.mu.Unlock()
	if l.public.Status != "connected" || time.Now().After(l.public.ExpiresAt) || l.public.Provider != c.Provider || l.region != c.Region || l.channel != id || l.version != version {
		return c, fmt.Errorf("%w: channel login is not current", domain.ErrConflict)
	}
	state, err := s.openLogin(l)
	if err != nil {
		return c, err
	}
	result, err := s.Save(ctx, owner, workflow, id, version, c, state.Credentials)
	if err == nil {
		s.removeLogin(l)
	}
	return result, err
}
