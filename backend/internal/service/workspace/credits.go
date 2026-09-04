package workspace

import (
	"context"
	"strings"
	"time"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	accountdomain "agent-platform/backend/internal/biz/account/domain"
	creditsdomain "agent-platform/backend/internal/biz/credits/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) GetCreditBalance(ctx context.Context, _ *workspacev1.GetCreditBalanceRequest) (*workspacev1.CreditBalance, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	balance, err := service.credits.Balance(ctx, principal.UserID, service.userTimezone(ctx, principal.UserID))
	if err != nil {
		return nil, publicError(err)
	}
	return creditBalanceResponse(balance), nil
}

func (service *Service) ListCreditLedger(ctx context.Context, request *workspacev1.ListCreditLedgerRequest) (*workspacev1.ListCreditLedgerResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	cursor := ""
	if request.Cursor != nil {
		cursor = *request.Cursor
	}
	limit := 50
	if request.Limit != nil {
		limit = int(*request.Limit)
	}
	page, err := service.credits.Ledger(ctx, owner, cursor, limit)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListCreditLedgerResponse{}
	if page.NextCursor != "" {
		response.NextCursor = &page.NextCursor
	}
	for _, item := range page.Items {
		entry := &workspacev1.CreditLedgerEntry{Id: item.ID, Type: item.Type, AmountHundredths: int64(item.Amount), ResultingBalanceHundredths: int64(item.ResultingBalance), CreditDay: item.CreditDay, CreatedAt: timestamppb.New(item.CreatedAt)}
		if item.Reason != "" {
			entry.Reason = &item.Reason
		}
		response.Items = append(response.Items, entry)
	}
	return response, nil
}

func (service *Service) RedeemCreditCode(ctx context.Context, request *workspacev1.RedeemCreditCodeRequest) (*workspacev1.CreditBalance, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	balance, err := service.credits.Redeem(ctx, owner, service.userTimezone(ctx, owner), request.Code)
	if err != nil {
		return nil, publicError(err)
	}
	return creditBalanceResponse(balance), nil
}

func (service *Service) ConfigureUserDailyCredits(ctx context.Context, request *workspacev1.ConfigureUserDailyCreditsRequest) (*workspacev1.CreditBalance, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	balance, err := service.credits.ConfigureDailyAllocation(ctx, request.UserId, "", creditsdomain.Amount(request.AllocationHundredths))
	if err != nil {
		return nil, publicError(err)
	}
	return creditBalanceResponse(balance), nil
}

func (service *Service) AdjustUserCredits(ctx context.Context, request *workspacev1.AdjustUserCreditsRequest) (*workspacev1.CreditBalance, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	balance, err := service.credits.Adjust(ctx, request.UserId, "", creditsdomain.Amount(request.AmountHundredths), request.Reason)
	if err != nil {
		return nil, publicError(err)
	}
	return creditBalanceResponse(balance), nil
}

func (service *Service) ListModelCreditRates(ctx context.Context, _ *workspacev1.ListModelCreditRatesRequest) (*workspacev1.ListModelCreditRatesResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	rates, err := service.credits.ListRates(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.ListModelCreditRatesResponse{}
	for _, rate := range rates {
		response.Items = append(response.Items, creditRateResponse(rate))
	}
	return response, nil
}

func (service *Service) CreateModelCreditRate(ctx context.Context, request *workspacev1.CreateModelCreditRateRequest) (*workspacev1.ModelCreditRate, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	rate := creditsdomain.ModelCreditRate{InputMultiplierMicros: request.InputMultiplierMicros, OutputMultiplierMicros: request.OutputMultiplierMicros, Fallback: creditsdomain.Amount(request.FallbackHundredths)}
	if request.ProviderType != nil || request.ApiProtocol != nil || request.ProviderModelId != nil {
		rate.Key = &creditsdomain.ModelRateKey{ProviderType: valueOrEmpty(request.ProviderType), Protocol: valueOrEmpty(request.ApiProtocol), ModelID: valueOrEmpty(request.ProviderModelId)}
	}
	created, err := service.credits.CreateRateRevision(ctx, administrator.UserID, rate, valueOrEmpty(request.ExpectedRevisionId))
	if err != nil {
		return nil, publicError(err)
	}
	return creditRateResponse(creditsdomain.RateRevision{Rate: created}), nil
}

func (service *Service) CreateRedemptionCodeBatch(ctx context.Context, request *workspacev1.CreateRedemptionCodeBatchRequest) (*workspacev1.RedemptionCodeBatch, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	var optionalExpiry *time.Time
	if request.ExpiresAt != nil {
		expiresAt := request.ExpiresAt.AsTime()
		optionalExpiry = &expiresAt
	}
	batch, err := service.credits.CreateRedemptionBatch(ctx, administrator.UserID, int(request.Count), creditsdomain.Amount(request.ValueHundredths), optionalExpiry)
	if err != nil {
		return nil, publicError(err)
	}
	response := &workspacev1.RedemptionCodeBatch{Id: batch.ID, Count: int32(batch.Count), ValueHundredths: int64(batch.Value), CreatedAt: timestamppb.New(batch.CreatedAt)}
	if batch.ExpiresAt != nil {
		response.ExpiresAt = timestamppb.New(*batch.ExpiresAt)
	}
	for _, code := range batch.Codes {
		response.Codes = append(response.Codes, &workspacev1.RedemptionCode{Id: code.ID, Identifier: code.Identifier, Plaintext: code.Plaintext, State: code.State})
	}
	return response, nil
}

func (service *Service) administrator(ctx context.Context) (accountdomain.Principal, error) {
	principal, err := service.accounts.Current(ctx)
	if err != nil {
		return accountdomain.Principal{}, publicError(err)
	}
	if !principal.Administrator {
		return accountdomain.Principal{}, publicError(accountdomain.ErrForbidden)
	}
	return principal, nil
}

func (service *Service) userTimezone(ctx context.Context, userID string) string {
	settings, err := service.workspace.Repository().GetSettings(ctx, userID)
	if err == nil && strings.TrimSpace(settings.Timezone) != "" {
		return settings.Timezone
	}
	return "Asia/Shanghai"
}

func creditBalanceResponse(balance creditsdomain.Balance) *workspacev1.CreditBalance {
	response := &workspacev1.CreditBalance{TotalHundredths: int64(balance.Total), DailyRemainingHundredths: int64(balance.DailyRemaining), PersistentHundredths: int64(balance.Persistent), TodayConsumedHundredths: int64(balance.TodayConsumed), DailyAllocationHundredths: int64(balance.DailyAllocation), CreditDay: balance.CreditDay, Timezone: balance.Timezone, NextAllocationAt: timestamppb.New(balance.NextAllocationAt), PendingEffectiveDay: optionalString(balance.PendingEffectiveDay), Version: balance.Version}
	if balance.PendingDailyAllocation != nil {
		value := int64(*balance.PendingDailyAllocation)
		response.PendingDailyAllocationHundredths = &value
	}
	return response
}

func creditRateResponse(revision creditsdomain.RateRevision) *workspacev1.ModelCreditRate {
	rate := revision.Rate
	response := &workspacev1.ModelCreditRate{RevisionId: rate.RevisionID, InputMultiplierMicros: rate.InputMultiplierMicros, OutputMultiplierMicros: rate.OutputMultiplierMicros, FallbackHundredths: int64(rate.Fallback), CreatedAt: timestamppb.New(rate.CreatedAt)}
	if rate.Key != nil {
		response.ProviderType, response.ApiProtocol, response.ProviderModelId = &rate.Key.ProviderType, &rate.Key.Protocol, &rate.Key.ModelID
	}
	if revision.SupersededAt != nil {
		response.SupersededAt = timestamppb.New(*revision.SupersededAt)
	}
	return response
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
