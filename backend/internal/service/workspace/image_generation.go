package workspace

import (
	"context"
	"errors"
	"sort"

	workspacev1 "agent-platform/backend/api/workspace/v1"
	aicreationapplication "agent-platform/backend/internal/biz/aicreation/application"
	aicreationdomain "agent-platform/backend/internal/biz/aicreation/domain"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (service *Service) ListImageGenerationOptions(ctx context.Context, _ *workspacev1.ListImageGenerationOptionsRequest) (*workspacev1.ListImageGenerationOptionsResponse, error) {
	if _, err := service.owner(ctx); err != nil {
		return nil, err
	}
	models, err := service.aicreation.ListImageModels(ctx, true)
	if err != nil {
		return nil, publicError(err)
	}
	candidates, err := service.aicreation.ListPromptCandidates(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListImageGenerationOptionsResponse{ImageModels: imageModelResponses(models, false), PromptOptimizationModels: promptCandidateResponses(candidates, false)}, nil
}

func (service *Service) OptimizeImagePrompt(ctx context.Context, request *workspacev1.OptimizeImagePromptRequest) (*workspacev1.OptimizeImagePromptResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	result, err := service.aicreation.OptimizePrompt(ctx, owner, settings.Timezone, request.ProviderModelId, request.Prompt, request.Locale)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.OptimizeImagePromptResponse{Prompt: result.Prompt, InputTokens: result.InputTokens, OutputTokens: result.OutputTokens}, nil
}

func (service *Service) ReplacePromptOptimizationCandidates(ctx context.Context, request *workspacev1.ReplacePromptOptimizationCandidatesRequest) (*workspacev1.ListPromptOptimizationCandidatesResponse, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]aicreationdomain.PromptOptimizationCandidate, 0, len(request.Items))
	var apiKey []byte
	for _, item := range request.Items {
		apiKey = []byte(item.ApiKey)
		candidates = append(candidates, aicreationdomain.PromptOptimizationCandidate{ProviderModelID: item.ProviderModelId, DisplayName: item.ProviderModelId, ProviderType: "openai", ModelID: item.ProviderModelId, Protocol: "openai_responses", Endpoint: item.Endpoint, APIKeyConfigured: len(apiKey) > 0, Instruction: item.Instruction})
	}
	defer clear(apiKey)
	if len(candidates) == 1 && len(apiKey) == 0 {
		current, listErr := service.aicreation.ListPromptCandidates(ctx)
		if listErr != nil {
			return nil, publicError(listErr)
		}
		candidates[0].APIKeyConfigured = len(current) == 1 && current[0].APIKeyConfigured
	}
	if err := service.aicreation.ReplacePromptCandidates(ctx, administrator.UserID, candidates, apiKey); err != nil {
		return nil, publicError(err)
	}
	items, err := service.aicreation.ListPromptCandidates(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListPromptOptimizationCandidatesResponse{Items: promptCandidateResponses(items, true)}, nil
}

func (service *Service) ListPromptOptimizationCandidates(ctx context.Context, _ *workspacev1.ListPromptOptimizationCandidatesRequest) (*workspacev1.ListPromptOptimizationCandidatesResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	items, err := service.aicreation.ListPromptCandidates(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListPromptOptimizationCandidatesResponse{Items: promptCandidateResponses(items, true)}, nil
}

func promptCandidateResponses(items []aicreationdomain.PromptOptimizationCandidate, includeEndpoint bool) []*workspacev1.PromptOptimizationCandidate {
	result := make([]*workspacev1.PromptOptimizationCandidate, 0, len(items))
	for _, item := range items {
		response := &workspacev1.PromptOptimizationCandidate{ProviderModelId: item.ProviderModelID, DisplayName: item.DisplayName, ApiProtocol: item.Protocol, ApiKeyConfigured: item.APIKeyConfigured, Instruction: item.Instruction}
		if includeEndpoint {
			response.Endpoint = &item.Endpoint
		}
		result = append(result, response)
	}
	return result
}

func (service *Service) ListImageModels(ctx context.Context, _ *workspacev1.ListImageModelsRequest) (*workspacev1.ListImageModelsResponse, error) {
	if _, err := service.administrator(ctx); err != nil {
		return nil, err
	}
	models, err := service.aicreation.ListImageModels(ctx, false)
	if err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.ListImageModelsResponse{Items: imageModelResponses(models, true)}, nil
}

func (service *Service) CreateImageModel(ctx context.Context, request *workspacev1.CreateImageModelRequest) (*workspacev1.ImageModel, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	apiKey := []byte(request.ApiKey)
	defer clear(apiKey)
	modes, sizes, qualities, formats, backgrounds, rates := platformImageOptions()
	model, err := service.aicreation.CreateImageModel(ctx, aicreationapplication.CreateImageModelRequest{
		AdministratorID: administrator.UserID, DisplayName: request.ProviderModelId, Endpoint: request.Endpoint, APIKey: apiKey, ModelID: request.ProviderModelId,
		Modes: modes, Sizes: sizes, Qualities: qualities, Formats: formats, Backgrounds: backgrounds,
		DefaultSize: sizes[0], DefaultQuality: qualities[0], DefaultFormat: formats[0], DefaultBackground: backgrounds[0], Rates: rates,
	})
	if err != nil {
		return nil, publicError(err)
	}
	return imageModelResponse(model, true), nil
}

func platformImageOptions() ([]aicreationdomain.Mode, []string, []string, []string, []string, map[string]int64) {
	modes := []aicreationdomain.Mode{aicreationdomain.ModeGenerate, aicreationdomain.ModeEdit}
	sizes := []string{"1024x1024", "1536x1024", "1024x1536"}
	qualities := []string{"auto", "low", "medium", "high"}
	formats := []string{"png", "jpeg", "webp"}
	backgrounds := []string{"opaque", "transparent"}
	rates := make(map[string]int64, len(sizes)*len(qualities))
	for _, size := range sizes {
		for _, quality := range qualities {
			rates[aicreationdomain.RateKey(size, quality)] = 100
		}
	}
	return modes, sizes, qualities, formats, backgrounds, rates
}

func (service *Service) ReviseImageModel(ctx context.Context, request *workspacev1.ReviseImageModelRequest) (*workspacev1.ImageModel, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	var apiKey []byte
	if request.ReplacementApiKey != nil {
		apiKey = []byte(*request.ReplacementApiKey)
		defer clear(apiKey)
	}
	modes, sizes, qualities, formats, backgrounds, rates := platformImageOptions()
	model, err := service.aicreation.ReviseImageModel(ctx, request.ImageModelId, request.ExpectedVersion, aicreationapplication.CreateImageModelRequest{
		AdministratorID: administrator.UserID, DisplayName: request.ProviderModelId, Endpoint: request.Endpoint, APIKey: apiKey, ModelID: request.ProviderModelId,
		Modes: modes, Sizes: sizes, Qualities: qualities, Formats: formats, Backgrounds: backgrounds,
		DefaultSize: sizes[0], DefaultQuality: qualities[0], DefaultFormat: formats[0], DefaultBackground: backgrounds[0], Rates: rates,
	})
	if err != nil {
		return nil, publicError(err)
	}
	return imageModelResponse(model, true), nil
}

func (service *Service) DeleteImageModel(ctx context.Context, request *workspacev1.DeleteImageModelRequest) (*workspacev1.DeleteResponse, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.aicreation.DeleteImageModel(ctx, administrator.UserID, request.ImageModelId, request.ExpectedVersion); err != nil {
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func (service *Service) VerifyImageModel(ctx context.Context, request *workspacev1.VerifyImageModelRequest) (*workspacev1.ImageModel, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	model, err := service.aicreation.VerifyImageModel(ctx, administrator.UserID, request.ImageModelId)
	if err != nil {
		return nil, publicError(err)
	}
	return imageModelResponse(model, true), nil
}

func (service *Service) SetImageModelAvailability(ctx context.Context, request *workspacev1.SetImageModelAvailabilityRequest) (*workspacev1.ImageModel, error) {
	administrator, err := service.administrator(ctx)
	if err != nil {
		return nil, err
	}
	model, err := service.aicreation.SetImageModelAvailability(ctx, administrator.UserID, request.ImageModelId, request.Available)
	if err != nil {
		return nil, publicError(err)
	}
	return imageModelResponse(model, true), nil
}

func (service *Service) SubmitImageGeneration(ctx context.Context, request *workspacev1.SubmitImageGenerationRequest) (*workspacev1.ImageGenerationRecord, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	record, err := service.aicreation.Submit(ctx, aicreationapplication.SubmitRequest{
		OwnerID: owner, RequestID: request.RequestId, OriginalPrompt: request.OriginalPrompt, Timezone: settings.Timezone, ModelID: request.ImageModelId, ReferenceUploadIDs: request.ReferenceUploadIds,
		Generation: aicreationdomain.GenerationRequest{Mode: aicreationdomain.Mode(request.Mode), Prompt: request.Prompt, Size: request.Size, Quality: request.Quality, Format: request.Format, Background: request.Background, Count: int(request.Count)},
	})
	if err != nil {
		return nil, publicError(err)
	}
	return imageGenerationResponse(record), nil
}

func (service *Service) ListImageGenerations(ctx context.Context, request *workspacev1.ListImageGenerationsRequest) (*workspacev1.ListImageGenerationsResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	records, err := service.aicreation.ListRecords(ctx, owner, int(request.GetLimit()))
	if err != nil {
		return nil, publicError(err)
	}
	items := make([]*workspacev1.ImageGenerationRecord, 0, len(records))
	for _, record := range records {
		items = append(items, imageGenerationResponse(record))
	}
	return &workspacev1.ListImageGenerationsResponse{Items: items}, nil
}

func (service *Service) GetImageGeneration(ctx context.Context, request *workspacev1.GetImageGenerationRequest) (*workspacev1.ImageGenerationRecord, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	record, err := service.aicreation.GetRecord(ctx, owner, request.RecordId)
	if err != nil {
		return nil, publicError(err)
	}
	return imageGenerationResponse(record), nil
}

func (service *Service) StopImageGeneration(ctx context.Context, request *workspacev1.StopImageGenerationRequest) (*workspacev1.ImageGenerationRecord, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	record, err := service.aicreation.Stop(ctx, owner, request.RecordId)
	if err != nil {
		return nil, publicError(err)
	}
	return imageGenerationResponse(record), nil
}

func (service *Service) RegenerateImageGeneration(ctx context.Context, request *workspacev1.RegenerateImageGenerationRequest) (*workspacev1.ImageGenerationRecord, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := service.workspace.Repository().GetSettings(ctx, owner)
	if err != nil {
		return nil, publicError(err)
	}
	record, err := service.aicreation.Regenerate(ctx, owner, settings.Timezone, request.RecordId, request.RequestId)
	if err != nil {
		return nil, publicError(err)
	}
	return imageGenerationResponse(record), nil
}

func (service *Service) DeleteImageGeneration(ctx context.Context, request *workspacev1.DeleteImageGenerationRequest) (*workspacev1.DeleteResponse, error) {
	owner, err := service.owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := service.aicreation.Delete(ctx, owner, request.RecordId); err != nil {
		if errors.Is(err, aicreationdomain.ErrNotFound) {
			return &workspacev1.DeleteResponse{Deleted: true}, nil
		}
		return nil, publicError(err)
	}
	return &workspacev1.DeleteResponse{Deleted: true}, nil
}

func imageModelResponses(models []aicreationdomain.ImageModelRevision, includeEndpoint bool) []*workspacev1.ImageModel {
	result := make([]*workspacev1.ImageModel, 0, len(models))
	for _, model := range models {
		result = append(result, imageModelResponse(model, includeEndpoint))
	}
	return result
}

func imageModelResponse(model aicreationdomain.ImageModelRevision, includeEndpoint bool) *workspacev1.ImageModel {
	rates := make([]*workspacev1.ImageCreditRate, 0, len(model.Rates))
	for _, size := range model.Sizes {
		for _, quality := range model.Qualities {
			if amount, ok := model.Rates[aicreationdomain.RateKey(size, quality)]; ok {
				rates = append(rates, &workspacev1.ImageCreditRate{Size: size, Quality: quality, AmountHundredths: amount})
			}
		}
	}
	sort.SliceStable(rates, func(i, j int) bool { return rates[i].Size+rates[i].Quality < rates[j].Size+rates[j].Quality })
	modes := make([]string, 0, len(model.Modes))
	for _, mode := range model.Modes {
		modes = append(modes, string(mode))
	}
	response := &workspacev1.ImageModel{Id: model.ID, RevisionId: model.RevisionID, DisplayName: model.DisplayName, ProviderModelId: model.ModelID, ApiProtocol: model.Protocol, Modes: modes, Sizes: model.Sizes, Qualities: model.Qualities, Formats: model.Formats, Backgrounds: model.Backgrounds, DefaultSize: model.DefaultSize, DefaultQuality: model.DefaultQuality, DefaultFormat: model.DefaultFormat, DefaultBackground: model.DefaultBackground, Rates: rates, State: string(model.State), ApiKeyConfigured: model.APIKeyConfigured, CreatedAt: timestamppb.New(model.CreatedAt), UpdatedAt: timestamppb.New(model.UpdatedAt), Version: model.Version}
	if includeEndpoint {
		response.Endpoint = &model.Endpoint
	}
	if !model.VerifiedAt.IsZero() {
		response.VerifiedAt = timestamppb.New(model.VerifiedAt)
	}
	return response
}

func imageGenerationResponse(record aicreationdomain.ImageGenerationRecord) *workspacev1.ImageGenerationRecord {
	images := make([]*workspacev1.GeneratedImage, 0, len(record.Images))
	for _, image := range record.Images {
		images = append(images, &workspacev1.GeneratedImage{Position: int32(image.Position), MediaType: image.MediaType, EncodedSize: image.Size, Width: int32(image.Width), Height: int32(image.Height), ExpiresAt: timestamppb.New(image.ExpiresAt)})
	}
	response := &workspacev1.ImageGenerationRecord{Id: record.ID, ImageModelId: record.Model.ID, ImageModelRevisionId: record.Model.RevisionID, ImageModelName: record.Model.DisplayName, Prompt: record.Prompt, Mode: string(record.Request.Mode), Size: record.Request.Size, Quality: record.Request.Quality, Format: record.Request.Format, Background: record.Request.Background, RequestedCount: int32(record.RequestedCount), ValidatedCount: int32(record.ValidatedCount), ReservationHundredths: record.ReservationAmount, ConsumptionHundredths: record.ConsumptionAmount, State: string(record.State), Images: images, CreatedAt: timestamppb.New(record.CreatedAt), Version: record.Version}
	if record.SafeError != "" {
		response.SafeError = &record.SafeError
	}
	if record.StartedAt != nil {
		response.StartedAt = timestamppb.New(*record.StartedAt)
	}
	if record.CompletedAt != nil {
		response.CompletedAt = timestamppb.New(*record.CompletedAt)
	}
	return response
}
