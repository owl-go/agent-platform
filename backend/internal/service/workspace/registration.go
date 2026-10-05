package workspace

import (
	"context"
	"fmt"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountapplication "agent-platform/backend/internal/biz/account/application"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
)

func (service *Service) EnableRegistration(app *accountapplication.Registration, key string) error {
	broker, err := NewRegistrationBroker(app, key)
	if err != nil {
		return err
	}
	service.registration = broker
	return nil
}
func (service *Service) GetRegistrationSettings(ctx context.Context, _ *workspacev1.GetRegistrationSettingsRequest) (*workspacev1.RegistrationSettingsResponse, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	if err = principal.RequireAdministrator(); err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.RegistrationSettingsResponse{}
	if service.registration == nil {
		return response, nil
	}
	response.Available = service.registration.app.Available()
	for _, provider := range []string{accountdomain.RegistrationWeChat, accountdomain.RegistrationFeishu} {
		s, err := service.registration.app.AdminSettings(ctx, provider)
		if err != nil {
			return nil, publicError(err)
		}
		response.Items = append(response.Items, service.registrationMethodResponse(s))
	}
	return response, nil
}
func (service *Service) UpdateRegistrationSettings(ctx context.Context, request *workspacev1.UpdateRegistrationSettingsRequest) (*workspacev1.RegistrationMethod, error) {
	if service.registration == nil {
		return nil, publicError(fmt.Errorf("registration is unavailable"))
	}
	s, err := service.registration.app.Save(ctx, accountdomain.RegistrationSettings{Provider: request.Provider, Enabled: request.Enabled, AppID: request.AppId, AppSecret: request.AppSecret, OfficialAccountID: request.OfficialAccountId, VerificationToken: request.VerificationToken, EncodingAESKey: request.EncodingAesKey}, request.ExpectedVersion, request.Reason)
	if err != nil {
		return nil, publicError(err)
	}
	return service.registrationMethodResponse(s), nil
}
func (service *Service) registrationMethodResponse(s accountdomain.RegistrationSettings) *workspacev1.RegistrationMethod {
	return &workspacev1.RegistrationMethod{Provider: s.Provider, Enabled: s.Enabled, Ready: s.Ready, AppId: s.AppID, AppSecretConfigured: s.AppSecret != "", TenantKey: s.TenantKey, OfficialAccountId: s.OfficialAccountID, VerificationTokenConfigured: s.VerificationToken != "", EncodingAesKeyConfigured: s.EncodingAESKey != "", Version: s.Version, CallbackUrl: service.registration.app.Callback(s.Provider)}
}
