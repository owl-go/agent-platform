package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	"agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/biz/workspace/domain"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Service) channelService() (*application.MessageChannels, error) {
	channels := s.workspace.MessageChannels()
	if channels == nil {
		return nil, publicError(domain.ErrInvalid)
	}
	return channels, nil
}
func (s *Service) ListMessageChannels(ctx context.Context, request *workspacev1.ListMessageChannelsRequest) (*workspacev1.ListMessageChannelsResponse, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	items, err := channels.List(ctx, owner, request.WorkflowId)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListMessageChannelsResponse{Items: []*workspacev1.MessageChannel{}, Available: s.config.MessageChannels.Enabled}
	for _, item := range items {
		response.Items = append(response.Items, channelResponse(item))
	}
	return response, nil
}
func (s *Service) SaveMessageChannel(ctx context.Context, request *workspacev1.SaveMessageChannelRequest) (*workspacev1.MessageChannel, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	if request.Audience == nil {
		return nil, publicError(domain.ErrInvalid)
	}
	input := domain.MessageChannel{Provider: request.Provider, Name: request.Name, Region: request.Region, Audience: domain.ChannelAudience{SenderIDs: request.Audience.SenderIds, GroupIDs: request.Audience.GroupIds, AllowDirect: request.Audience.AllowDirect}}
	var item domain.MessageChannel
	if request.LoginId != "" {
		if len(request.Credentials) != 0 {
			return nil, publicError(domain.ErrInvalid)
		}
		item, err = channels.SaveWithLogin(ctx, owner, request.WorkflowId, request.ChannelId, request.Version, input, request.LoginId)
	} else {
		item, err = channels.Save(ctx, owner, request.WorkflowId, request.ChannelId, request.Version, input, request.Credentials)
	}
	if err != nil {
		return nil, publicError(err)
	}
	return channelResponse(item), nil
}
func (s *Service) StartChannelLogin(ctx context.Context, r *workspacev1.StartChannelLoginRequest) (*workspacev1.ChannelLogin, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	login, err := channels.StartLogin(ctx, owner, r.WorkflowId, r.Provider, r.Region, r.Method, r.ChannelId, r.Version, r.Credentials)
	if err != nil {
		return nil, publicError(err)
	}
	return loginResponse(login), nil
}
func (s *Service) PollChannelLogin(ctx context.Context, r *workspacev1.PollChannelLoginRequest) (*workspacev1.ChannelLogin, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	login, err := channels.PollLogin(ctx, owner, r.WorkflowId, r.LoginId, r.VerificationCode)
	if err != nil {
		return nil, publicError(err)
	}
	return loginResponse(login), nil
}
func (s *Service) CancelChannelLogin(ctx context.Context, r *workspacev1.CancelChannelLoginRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	if err = channels.CancelLogin(ctx, owner, r.WorkflowId, r.LoginId); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}
func loginResponse(l application.ChannelLogin) *workspacev1.ChannelLogin {
	response := &workspacev1.ChannelLogin{Id: l.ID, Provider: l.Provider, Status: l.Status, QrContent: l.QRContent, AccountId: l.AccountID, AccountName: l.AccountName, SuggestedSenderId: l.SuggestedSenderID, ExpiresAt: timestamppb.New(l.ExpiresAt), PairingCode: l.PairingCode, PairingStatus: l.PairingStatus}
	if l.PairingExpiresAt != nil {
		response.PairingExpiresAt = timestamppb.New(*l.PairingExpiresAt)
	}
	return response
}

func (s *Service) StartChannelSenderPairing(ctx context.Context, r *workspacev1.StartChannelSenderPairingRequest) (*workspacev1.ChannelLogin, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	login, err := channels.StartSenderPairing(ctx, owner, r.WorkflowId, r.LoginId, r.ChannelId, r.Version)
	if err != nil {
		return nil, publicError(err)
	}
	return loginResponse(login), nil
}

func (s *Service) ControlMessageChannel(ctx context.Context, request *workspacev1.ControlMessageChannelRequest) (*workspacev1.MessageChannel, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	item, err := channels.Control(ctx, owner, request.WorkflowId, request.ChannelId, request.Version, request.Action)
	if err != nil {
		return nil, publicError(err)
	}
	return channelResponse(item), nil
}
func (s *Service) ListChannelDeliveries(ctx context.Context, request *workspacev1.ListChannelDeliveriesRequest) (*workspacev1.ListChannelDeliveriesResponse, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	items, err := channels.Deliveries(ctx, owner, request.WorkflowId, request.ChannelId)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListChannelDeliveriesResponse{Items: []*workspacev1.ChannelDelivery{}}
	for _, d := range items {
		response.Items = append(response.Items, &workspacev1.ChannelDelivery{Id: d.ID, ChannelId: d.ChannelID, RunId: d.RunID, Kind: d.Kind, Chunk: int32(d.Chunk), State: d.State, Attempts: int32(d.Attempts), ErrorCode: d.ErrorCode, ProviderMessageId: d.ProviderMessageID, CreatedAt: timestamppb.New(d.CreatedAt)})
	}
	return response, nil
}
func (s *Service) RetryChannelDelivery(ctx context.Context, request *workspacev1.RetryChannelDeliveryRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := s.owner(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.channelService()
	if err != nil {
		return nil, err
	}
	if err = channels.Retry(ctx, owner, request.WorkflowId, request.ChannelId, request.DeliveryId, request.Version, request.ConfirmPossibleDuplicate); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}
func channelResponse(c domain.MessageChannel) *workspacev1.MessageChannel {
	result := &workspacev1.MessageChannel{Id: c.ID, WorkflowId: c.WorkflowID, Provider: c.Provider, Name: c.Name, AccountId: c.AccountID, AccountName: c.AccountName, TenantId: c.TenantID, Region: c.Region, Audience: &workspacev1.ChannelAudience{SenderIds: c.Audience.SenderIDs, GroupIds: c.Audience.GroupIDs, AllowDirect: c.Audience.AllowDirect}, Enabled: c.Enabled, Version: c.Version, ConfigVersion: c.ConfigVersion, ValidationState: c.ValidationState, ValidationCode: c.ValidationCode, Health: c.Health, ErrorCode: c.ErrorCode, CallbackUrl: c.CallbackURL}
	if c.ValidationUntil != nil {
		result.ValidationUntil = timestamppb.New(*c.ValidationUntil)
	}
	return result
}

func channelCallbackRoute(method, path string) (string, string, bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "message-channel-callbacks" || (parts[3] != "telegram" && parts[3] != "slack" && parts[3] != "whatsapp" && parts[3] != "qqbot") {
		return "", "", false
	}
	if method != http.MethodPost && !(method == http.MethodGet && parts[3] == "whatsapp") {
		return "", "", false
	}
	if _, err := uuid.Parse(parts[4]); err != nil {
		return "", "", false
	}
	return parts[3], parts[4], true
}

// OIDC bypass is narrow. Every request is still provider-authenticated against
// the exact raw body before any owner or Workflow information is trusted.
func (s *Service) messageChannelCallback(w http.ResponseWriter, r *http.Request) {
	provider, id, ok := channelCallbackRoute(r.Method, r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	channels, err := s.channelService()
	if err != nil {
		http.Error(w, "channel_unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		result, err := channels.Challenge(r.Context(), provider, id, r.URL.Query())
		if err != nil {
			http.Error(w, "callback_rejected", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, result)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024+1))
	if err != nil || len(body) > 64*1024 {
		http.Error(w, "invalid_payload", http.StatusRequestEntityTooLarge)
		return
	}
	result, err := channels.Callback(r.Context(), provider, id, r.Header, body)
	if err != nil {
		status := http.StatusServiceUnavailable
		if errors.Is(err, domain.ErrChannelCallbackAuthentication) {
			status = http.StatusUnauthorized
		}
		http.Error(w, "callback_rejected", status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
