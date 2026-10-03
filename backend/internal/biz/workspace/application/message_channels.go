package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
)

type ChannelCipher interface {
	Encrypt([]byte, string) ([]byte, error)
	Decrypt([]byte, string) ([]byte, error)
}
type ChannelCredentials map[string]string
type ChannelStored struct {
	Channel    domain.MessageChannel
	Ciphertext []byte
}
type ChannelSendJob struct {
	Delivery        domain.ChannelDelivery
	Stored          ChannelStored
	Message         domain.ChannelMessage
	ReplyCiphertext []byte
	InboxID         string
	Text            string
	Lease           string
}
type ChannelIdentity struct{ ID, Name, TenantID, BindingID string }
type ChannelSendResult struct {
	MessageID, State, Code string
	RetryAfter             time.Duration
}
type ChannelCallback struct {
	Messages []domain.ChannelMessage
	Response any
}

// Transport implementations must return bounded, credential-free diagnostics.
type ChannelAdapter interface {
	Identify(context.Context, ChannelCredentials, string) (ChannelIdentity, error)
	Configure(context.Context, ChannelStored, ChannelCredentials, string) error
	Callback(context.Context, ChannelStored, ChannelCredentials, http.Header, []byte) (ChannelCallback, error)
	Send(context.Context, ChannelStored, ChannelCredentials, domain.ChannelMessage, string, string) ChannelSendResult
}
type ChannelConnector interface {
	Connect(context.Context, ChannelStored, ChannelCredentials, func(context.Context, domain.ChannelMessage) error) error
}

type MessageChannelRepository interface {
	ListMessageChannels(context.Context, string, string) ([]domain.MessageChannel, error)
	GetMessageChannel(context.Context, string, string, string) (ChannelStored, error)
	SaveMessageChannel(context.Context, ChannelStored, int64) (ChannelStored, error)
	ControlMessageChannel(context.Context, string, string, string, int64, string, string) (ChannelStored, error)
	ReceiveChannelMessage(context.Context, ChannelStored, domain.ChannelMessage, []byte) (bool, error)
	AdmitChannelMessage(context.Context) (bool, error)
	ClaimChannelDelivery(context.Context) (*ChannelSendJob, error)
	FinishChannelDelivery(context.Context, *ChannelSendJob, ChannelSendResult) error
	ListChannelDeliveries(context.Context, string, string, string) ([]domain.ChannelDelivery, error)
	RetryChannelDelivery(context.Context, string, string, string, string, int64, bool) error
	ActiveMessageChannels(context.Context) ([]ChannelStored, error)
	SetChannelHealth(context.Context, string, int64, string, string) error
}

type MessageChannels struct {
	repository   MessageChannelRepository
	cipher       ChannelCipher
	adapters     map[string]ChannelAdapter
	enabled      bool
	callbackBase string
	limits       ChannelLimits
}

func NewMessageChannels(repository MessageChannelRepository, cipher ChannelCipher, adapters map[string]ChannelAdapter, enabled bool, callbackBase string, options ...ChannelLimits) *MessageChannels {
	limits := ChannelLimits{}.Effective()
	if len(options) > 0 {
		limits = options[0].Effective()
	}
	if protected, ok := repository.(interface {
		ConfigureChannelProtection(ChannelCipher, bool, ChannelLimits)
	}); ok {
		protected.ConfigureChannelProtection(cipher, enabled, limits)
	}
	return &MessageChannels{repository: repository, cipher: cipher, adapters: adapters, enabled: enabled, callbackBase: strings.TrimRight(callbackBase, "/"), limits: limits}
}
func ChannelCredentialAAD(c domain.MessageChannel) string {
	return "message-channel:" + c.OwnerID + ":" + c.ID + ":" + fmt.Sprint(c.ConfigVersion)
}
func ChannelReplyAAD(id string, m domain.ChannelMessage) string {
	return "message-channel-reply:" + id + ":" + m.EventID
}
func (s *MessageChannels) credentials(stored ChannelStored) (ChannelCredentials, error) {
	b, err := s.cipher.Decrypt(stored.Ciphertext, ChannelCredentialAAD(stored.Channel))
	if err != nil {
		return nil, fmt.Errorf("channel_credentials_unavailable")
	}
	var c ChannelCredentials
	err = json.Unmarshal(b, &c)
	if err != nil {
		return nil, fmt.Errorf("channel_credentials_unavailable")
	}
	return c, nil
}
func (s *MessageChannels) List(ctx context.Context, owner, workflow string) ([]domain.MessageChannel, error) {
	items, err := s.repository.ListMessageChannels(ctx, owner, workflow)
	for i := range items {
		s.decorate(&items[i])
	}
	return items, err
}
func (s *MessageChannels) decorate(c *domain.MessageChannel) {
	if c.Provider == "telegram" || c.Provider == "slack" {
		c.CallbackURL = s.callbackBase + "/api/v1/message-channel-callbacks/" + c.Provider + "/" + c.ID
	}
}

func (s *MessageChannels) Save(ctx context.Context, owner, workflow, id string, version int64, c domain.MessageChannel, credentials ChannelCredentials) (domain.MessageChannel, error) {
	if !s.enabled {
		return c, fmt.Errorf("%w: message channels are disabled by the administrator", domain.ErrInvalid)
	}
	adapter, ok := s.adapters[c.Provider]
	if !ok || len(c.Name) > 100 || strings.TrimSpace(c.Name) == "" || (c.Region != "" && c.Region != "feishu" && c.Region != "lark") || (c.Provider != "feishu" && c.Region != "") {
		return c, domain.ErrInvalid
	}
	if err := c.Audience.Validate(); err != nil {
		return c, err
	}
	if id != "" {
		old, err := s.repository.GetMessageChannel(ctx, owner, workflow, id)
		if err != nil {
			return c, err
		}
		if old.Channel.Version != version || old.Channel.Enabled || old.Channel.Provider != c.Provider {
			return c, domain.ErrConflict
		}
		if len(credentials) == 0 {
			credentials, err = s.credentials(old)
			if err != nil {
				return c, err
			}
		}
	} else {
		if version != 0 {
			return c, domain.ErrInvalid
		}
		id = uuid.NewString()
	}
	for k, v := range credentials {
		if len(k) > 64 || len(v) > 4096 || strings.ContainsAny(v, "\r\n\x00") {
			return c, domain.ErrInvalid
		}
	}
	identity, err := adapter.Identify(ctx, credentials, c.Region)
	if err != nil {
		return c, err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return c, err
	}
	credentials["callback_secret"] = hex.EncodeToString(secret)
	c.ID, c.OwnerID, c.WorkflowID, c.Version = id, owner, workflow, version+1
	c.AccountID, c.AccountName, c.TenantID = identity.ID, identity.Name, identity.TenantID
	c.BindingID = identity.BindingID
	if c.BindingID == "" {
		c.BindingID = identity.ID + ":" + identity.TenantID
	}
	c.ConfigVersion = c.Version
	c.Enabled = false
	c.ValidationState = "unverified"
	c.ValidationCode = ""
	c.ValidationUntil = nil
	c.Health = "disconnected"
	c.ErrorCode = ""
	b, err := json.Marshal(credentials)
	if err != nil {
		return c, err
	}
	cipher, err := s.cipher.Encrypt(b, ChannelCredentialAAD(c))
	if err != nil {
		return c, err
	}
	saved, err := s.repository.SaveMessageChannel(ctx, ChannelStored{Channel: c, Ciphertext: cipher}, version)
	s.decorate(&saved.Channel)
	return saved.Channel, err
}

type MessageChannelMaintenanceRepository interface {
	MaintainMessageChannels(context.Context) (bool, error)
}

func (s *MessageChannels) ProcessMaintenance(ctx context.Context) (bool, error) {
	repository, ok := s.repository.(MessageChannelMaintenanceRepository)
	if !ok {
		return false, nil
	}
	return repository.MaintainMessageChannels(ctx)
}

func (s *MessageChannels) Control(ctx context.Context, owner, workflow, id string, version int64, action string) (domain.MessageChannel, error) {
	if !s.enabled && action != "disable" && action != "delete" {
		return domain.MessageChannel{}, domain.ErrInvalid
	}
	if action != "validate" && action != "enable" && action != "disable" && action != "delete" && action != "reset" {
		return domain.MessageChannel{}, domain.ErrInvalid
	}
	code := ""
	if action == "validate" {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return domain.MessageChannel{}, err
		}
		code = "verify " + hex.EncodeToString(b)
	}
	old, err := s.repository.GetMessageChannel(ctx, owner, workflow, id)
	if err != nil {
		return domain.MessageChannel{}, err
	}
	if action == "validate" {
		if old.Channel.Version != version || old.Channel.Enabled {
			return old.Channel, domain.ErrConflict
		}
		credentials, err := s.credentials(old)
		if err != nil {
			return old.Channel, err
		}
		s.decorate(&old.Channel)
		if err = s.adapters[old.Channel.Provider].Configure(ctx, old, credentials, old.Channel.CallbackURL); err != nil {
			return old.Channel, err
		}
	}
	stored, err := s.repository.ControlMessageChannel(ctx, owner, workflow, id, version, action, code)
	s.decorate(&stored.Channel)
	return stored.Channel, err
}

func (s *MessageChannels) Receive(ctx context.Context, stored ChannelStored, message domain.ChannelMessage) error {
	if !s.enabled {
		return nil
	}
	now := time.Now().UTC()
	if len(message.Text) > s.limits.MaxTextBytes || message.Validate(now) != nil || !stored.Channel.Audience.Allows(message) {
		return nil
	}
	if !stored.Channel.Enabled && !(stored.Channel.ValidationState == "testing" && stored.Channel.ValidationUntil != nil && now.Before(*stored.Channel.ValidationUntil) && strings.TrimSpace(message.Text) == stored.Channel.ValidationCode) {
		return nil
	}
	credentials, err := s.credentials(stored)
	if err != nil {
		return err
	}
	for _, key := range []string{"bot_token", "signing_secret", "client_secret", "app_secret", "callback_secret"} {
		v := credentials[key]
		if v != "" {
			message.Text = strings.ReplaceAll(message.Text, v, "[REDACTED]")
		}
	}
	if target := message.Reply["session_webhook"]; target != "" {
		message.Text = strings.ReplaceAll(message.Text, target, "[REDACTED]")
	}
	b, err := json.Marshal(message.Reply)
	if err != nil {
		return err
	}
	reply, err := s.cipher.Encrypt(b, ChannelReplyAAD(stored.Channel.ID, message))
	if err != nil {
		return err
	}
	message.Reply = nil
	_, err = s.repository.ReceiveChannelMessage(ctx, stored, message, reply)
	return err
}
func (s *MessageChannels) Callback(ctx context.Context, provider, id string, headers http.Header, body []byte) (any, error) {
	if !s.enabled || len(body) > 64*1024 {
		return nil, domain.ErrInvalid
	}
	adapter, ok := s.adapters[provider]
	if !ok || (provider != "telegram" && provider != "slack") {
		return nil, domain.ErrInvalid
	}
	stored, err := s.repository.GetMessageChannel(ctx, "", "", id)
	if err != nil || stored.Channel.Provider != provider {
		return nil, domain.ErrNotFound
	}
	credentials, err := s.credentials(stored)
	if err != nil {
		return nil, err
	}
	parsed, err := adapter.Callback(ctx, stored, credentials, headers, body)
	if err != nil {
		return nil, err
	}
	for _, msg := range parsed.Messages {
		if err = s.Receive(ctx, stored, msg); err != nil {
			return nil, err
		}
	}
	return parsed.Response, nil
}
func (s *MessageChannels) ProcessInbox(ctx context.Context) (bool, error) {
	if !s.enabled {
		return false, nil
	}
	return s.repository.AdmitChannelMessage(ctx)
}
func (s *MessageChannels) ProcessDelivery(ctx context.Context) (bool, error) {
	if !s.enabled {
		return false, nil
	}
	job, err := s.repository.ClaimChannelDelivery(ctx)
	if err != nil || job == nil {
		return false, err
	}
	c, err := s.credentials(job.Stored)
	if err != nil {
		return true, s.repository.FinishChannelDelivery(ctx, job, ChannelSendResult{State: "failed", Code: "credentials_unavailable"})
	}
	b, err := s.cipher.Decrypt(job.ReplyCiphertext, ChannelReplyAAD(job.Stored.Channel.ID, job.Message))
	if err != nil || json.Unmarshal(b, &job.Message.Reply) != nil {
		return true, s.repository.FinishChannelDelivery(ctx, job, ChannelSendResult{State: "failed", Code: "reply_unavailable"})
	}
	text := job.Text
	for _, key := range []string{"bot_token", "signing_secret", "client_secret", "app_secret", "callback_secret"} {
		v := c[key]
		if v != "" {
			text = strings.ReplaceAll(text, v, "[REDACTED]")
		}
	}
	if target := job.Message.Reply["session_webhook"]; target != "" {
		text = strings.ReplaceAll(text, target, "[REDACTED]")
	}
	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result := s.adapters[job.Stored.Channel.Provider].Send(sendCtx, job.Stored, c, job.Message, text, job.Delivery.ID)
	return true, s.repository.FinishChannelDelivery(ctx, job, result)
}
func (s *MessageChannels) Deliveries(ctx context.Context, owner, workflow, id string) ([]domain.ChannelDelivery, error) {
	return s.repository.ListChannelDeliveries(ctx, owner, workflow, id)
}
func (s *MessageChannels) Retry(ctx context.Context, owner, workflow, id, delivery string, version int64, confirm bool) error {
	return s.repository.RetryChannelDelivery(ctx, owner, workflow, id, delivery, version, confirm)
}
