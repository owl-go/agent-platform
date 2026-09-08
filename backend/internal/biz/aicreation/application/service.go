package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strconv"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/aicreation/domain"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

var ErrInsufficientCredits = errors.New("Available Credit is insufficient")

type Repository interface {
	SaveModel(context.Context, domain.ImageModelRevision) (domain.ImageModelRevision, error)
	GetModel(context.Context, string) (domain.ImageModelRevision, error)
	ListModels(context.Context, bool) ([]domain.ImageModelRevision, error)
	DeleteModel(context.Context, string, int64, time.Time) error
	SaveReferenceUpload(context.Context, domain.ReferenceUpload) (domain.ReferenceUpload, error)
	GetReferenceUploads(context.Context, string, []string) ([]domain.ReferenceUpload, error)
	DeleteReferenceUpload(context.Context, string, string) (domain.ReferenceUpload, error)
	ListPromptCandidates(context.Context) ([]domain.PromptOptimizationCandidate, error)
	ReplacePromptCandidates(context.Context, string, []domain.PromptOptimizationCandidate) error
	SubmitRecord(context.Context, domain.ImageGenerationRecord, string) (domain.ImageGenerationRecord, error)
	SettleRecord(context.Context, domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error)
	GetRecord(context.Context, string, string) (domain.ImageGenerationRecord, error)
	GetRecordByRequestID(context.Context, string, string) (domain.ImageGenerationRecord, error)
	ListRecords(context.Context, string, int) ([]domain.ImageGenerationRecord, error)
	ListEvents(context.Context, string, string, int64, int) ([]GenerationEvent, error)
	DeleteRecord(context.Context, string, string, time.Time) (domain.ImageGenerationRecord, error)
	FinalizeDeletedRecord(context.Context, string) error
	ResolveExpiredDispatch(context.Context, time.Time) (bool, error)
	ListExpiredObjects(context.Context, time.Time, int) ([]ExpiredObject, error)
	ForgetExpiredObject(context.Context, ExpiredObject, time.Time) error
	TrackObject(context.Context, string, time.Time) error
	UntrackObject(context.Context, string) error
	ClaimNext(context.Context, time.Time) (domain.ImageGenerationRecord, bool, error)
	UpdateRecord(context.Context, domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error)
}

type ExpiredObject struct {
	Kind     string
	RecordID string
	UploadID string
	Position int
	Key      string
}

type GenerationEvent struct {
	Sequence  int64
	Type      string
	CreatedAt time.Time
}

type ProviderRequest struct {
	ConnectionID      string
	ConnectionVersion int64
	ModelID           string
	Prompt            string
	Inputs            []domain.ReferenceImage
	Size              string
	Quality           string
	Format            string
	Background        string
	Count             int
}

type ProviderResult struct{ Images [][]byte }

type ProviderFailure struct {
	Code           string
	OutcomeUnknown bool
	Cause          error
}

func (failure *ProviderFailure) Error() string { return failure.Code }
func (failure *ProviderFailure) Unwrap() error { return failure.Cause }

type ImageProvider interface {
	Create(context.Context, ProviderRequest) (ProviderResult, error)
}

type ObjectMetadata struct {
	Size        int64
	SHA256      string
	ContentType string
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, ObjectMetadata) error
	Get(context.Context, string) (io.ReadCloser, ObjectMetadata, error)
	Delete(context.Context, string) error
}

type Clock func() time.Time

type OptimizationRequest struct {
	Candidate domain.PromptOptimizationCandidate
	Prompt    string
	Locale    string
}

type OptimizationResult struct {
	Prompt       string
	InputTokens  int64
	OutputTokens int64
}

type PromptOptimizer interface {
	Optimize(context.Context, OptimizationRequest) (OptimizationResult, error)
}

type TextCreditAdmission struct{ Value any }

type TextCredits interface {
	Admit(context.Context, string, string, string, domain.PromptOptimizationCandidate) (TextCreditAdmission, error)
	SettleText(context.Context, TextCreditAdmission, int64, int64) error
	Abort(context.Context, TextCreditAdmission) error
}

type Service struct {
	repository  Repository
	provider    ImageProvider
	objects     ObjectStore
	now         Clock
	optimizer   PromptOptimizer
	textCredits TextCredits
}

func (service *Service) EnablePromptOptimization(optimizer PromptOptimizer, credits TextCredits) error {
	if optimizer == nil || credits == nil {
		return fmt.Errorf("Prompt Optimizer and text Credits are required")
	}
	service.optimizer, service.textCredits = optimizer, credits
	return nil
}

func (service *Service) ListPromptCandidates(ctx context.Context) ([]domain.PromptOptimizationCandidate, error) {
	return service.repository.ListPromptCandidates(ctx)
}

func (service *Service) ReplacePromptCandidates(ctx context.Context, administratorID string, candidates []domain.PromptOptimizationCandidate) error {
	if strings.TrimSpace(administratorID) == "" {
		return fmt.Errorf("%w: Administrator is required", domain.ErrInvalid)
	}
	for _, candidate := range candidates {
		if candidate.ProviderModelID == "" || (candidate.Protocol != "openai_responses" && candidate.Protocol != "openai_chat_completions") {
			return fmt.Errorf("%w: Prompt Optimization candidate is invalid", domain.ErrInvalid)
		}
	}
	return service.repository.ReplacePromptCandidates(ctx, administratorID, candidates)
}

func (service *Service) OptimizePrompt(ctx context.Context, ownerID, timezone, candidateID, prompt, locale string) (OptimizationResult, error) {
	if service.optimizer == nil || service.textCredits == nil {
		return OptimizationResult{}, fmt.Errorf("Prompt Optimization is unavailable")
	}
	if strings.TrimSpace(prompt) == "" || len([]rune(prompt)) > 10_000 {
		return OptimizationResult{}, fmt.Errorf("%w: prompt is invalid", domain.ErrInvalid)
	}
	records, err := service.repository.ListRecords(ctx, ownerID, 100)
	if err != nil {
		return OptimizationResult{}, err
	}
	for _, record := range records {
		if !record.State.Terminal() {
			return OptimizationResult{}, domain.ErrConflict
		}
	}
	candidates, err := service.repository.ListPromptCandidates(ctx)
	if err != nil {
		return OptimizationResult{}, err
	}
	var candidate domain.PromptOptimizationCandidate
	for _, item := range candidates {
		if item.ProviderModelID == candidateID {
			candidate = item
			break
		}
	}
	if candidate.ProviderModelID == "" {
		return OptimizationResult{}, domain.ErrNotFound
	}
	executionID := "prompt-optimization-" + uuid.NewString()
	admission, err := service.textCredits.Admit(ctx, ownerID, executionID, timezone, candidate)
	if err != nil {
		return OptimizationResult{}, err
	}
	result, err := service.optimizer.Optimize(ctx, OptimizationRequest{Candidate: candidate, Prompt: prompt, Locale: locale})
	if err != nil {
		_ = service.textCredits.Abort(ctx, admission)
		return OptimizationResult{}, err
	}
	if strings.TrimSpace(result.Prompt) == "" || len([]rune(result.Prompt)) > 10_000 {
		_ = service.textCredits.Abort(ctx, admission)
		return OptimizationResult{}, fmt.Errorf("%w: optimized prompt is invalid", domain.ErrInvalid)
	}
	if err := service.textCredits.SettleText(ctx, admission, result.InputTokens, result.OutputTokens); err != nil {
		return OptimizationResult{}, err
	}
	return result, nil
}

func New(repository Repository, provider ImageProvider, objects ObjectStore, now Clock) (*Service, error) {
	if repository == nil || provider == nil || objects == nil {
		return nil, fmt.Errorf("AI Creation Repository, Image Provider, and Object Store are required")
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, provider: provider, objects: objects, now: now}, nil
}

type CreateImageModelRequest struct {
	AdministratorID   string
	DisplayName       string
	ConnectionID      string
	ConnectionVersion int64
	ConnectionName    string
	ModelID           string
	Modes             []domain.Mode
	Sizes             []string
	Qualities         []string
	Formats           []string
	Backgrounds       []string
	DefaultSize       string
	DefaultQuality    string
	DefaultFormat     string
	DefaultBackground string
	Rates             map[string]int64
}

func (service *Service) CreateImageModel(ctx context.Context, request CreateImageModelRequest) (domain.ImageModelRevision, error) {
	if strings.TrimSpace(request.AdministratorID) == "" {
		return domain.ImageModelRevision{}, fmt.Errorf("%w: Administrator is required", domain.ErrInvalid)
	}
	now := service.now().UTC()
	model := domain.ImageModelRevision{
		ID: uuid.NewString(), RevisionID: uuid.NewString(), DisplayName: strings.TrimSpace(request.DisplayName),
		ConnectionID: request.ConnectionID, ConnectionVersion: request.ConnectionVersion, ConnectionName: request.ConnectionName,
		ModelID: request.ModelID, Protocol: domain.ProtocolOpenAIImages, Modes: request.Modes, Sizes: request.Sizes,
		Qualities: request.Qualities, Formats: request.Formats, Backgrounds: request.Backgrounds,
		DefaultSize: request.DefaultSize, DefaultQuality: request.DefaultQuality, DefaultFormat: request.DefaultFormat,
		DefaultBackground: request.DefaultBackground, Rates: request.Rates, State: domain.ModelUnverified,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	if err := model.ValidateConfiguration(); err != nil {
		return domain.ImageModelRevision{}, err
	}
	return service.repository.SaveModel(ctx, model)
}

func (service *Service) ReviseImageModel(ctx context.Context, modelID string, expectedVersion int64, request CreateImageModelRequest) (domain.ImageModelRevision, error) {
	current, err := service.repository.GetModel(ctx, modelID)
	if err != nil {
		return domain.ImageModelRevision{}, err
	}
	if current.Version != expectedVersion {
		return domain.ImageModelRevision{}, domain.ErrVersionConflict
	}
	if current.State == domain.ModelDeleted {
		return domain.ImageModelRevision{}, domain.ErrConflict
	}
	now := service.now().UTC()
	model := domain.ImageModelRevision{
		ID: modelID, RevisionID: uuid.NewString(), PredecessorID: current.RevisionID, PredecessorVersion: current.Version, DisplayName: strings.TrimSpace(request.DisplayName),
		ConnectionID: request.ConnectionID, ConnectionVersion: request.ConnectionVersion, ConnectionName: request.ConnectionName,
		ModelID: request.ModelID, Protocol: domain.ProtocolOpenAIImages, Modes: request.Modes, Sizes: request.Sizes,
		Qualities: request.Qualities, Formats: request.Formats, Backgrounds: request.Backgrounds,
		DefaultSize: request.DefaultSize, DefaultQuality: request.DefaultQuality, DefaultFormat: request.DefaultFormat,
		DefaultBackground: request.DefaultBackground, Rates: request.Rates, State: domain.ModelUnverified,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	if err := model.ValidateConfiguration(); err != nil {
		return domain.ImageModelRevision{}, err
	}
	return service.repository.SaveModel(ctx, model)
}

func (service *Service) DeleteImageModel(ctx context.Context, administratorID, modelID string, expectedVersion int64) error {
	if strings.TrimSpace(administratorID) == "" {
		return fmt.Errorf("%w: Administrator is required", domain.ErrInvalid)
	}
	return service.repository.DeleteModel(ctx, modelID, expectedVersion, service.now().UTC())
}

func (service *Service) VerifyImageModel(ctx context.Context, administratorID, modelID string) (domain.ImageModelRevision, error) {
	if strings.TrimSpace(administratorID) == "" {
		return domain.ImageModelRevision{}, fmt.Errorf("%w: Administrator is required", domain.ErrInvalid)
	}
	model, err := service.repository.GetModel(ctx, modelID)
	if err != nil {
		return model, err
	}
	if model.State == domain.ModelDeleted {
		return model, domain.ErrConflict
	}
	verificationRequest := domain.GenerationRequest{
		Mode: domain.ModeGenerate, Prompt: "A small teal circle on a white background.", Size: model.DefaultSize,
		Quality: model.DefaultQuality, Format: model.DefaultFormat, Background: model.DefaultBackground, Count: 1,
	}
	if !containsMode(model.Modes, domain.ModeGenerate) {
		data, decodeErr := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
		if decodeErr != nil {
			return model, decodeErr
		}
		digest := sha256.Sum256(data)
		key := fmt.Sprintf("ai-creation/verification/%s/%s", model.ID, uuid.NewString())
		metadata := ObjectMetadata{Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), ContentType: "image/png"}
		if err := service.repository.TrackObject(ctx, key, service.now().UTC().Add(24*time.Hour)); err != nil {
			return model, err
		}
		if err := service.objects.Put(ctx, key, bytes.NewReader(data), metadata); err != nil {
			return model, err
		}
		defer func() {
			if service.objects.Delete(ctx, key) == nil {
				_ = service.repository.UntrackObject(ctx, key)
			}
		}()
		verificationRequest.Mode = domain.ModeEdit
		verificationRequest.References = []domain.ReferenceImage{{Position: 1, ObjectKey: key, SHA256: metadata.SHA256, MediaType: metadata.ContentType, Size: metadata.Size, Width: 1, Height: 1}}
	}
	result, err := service.provider.Create(ctx, providerRequest(model, verificationRequest))
	if err != nil {
		return model, fmt.Errorf("verify Image Model: %w", err)
	}
	if len(result.Images) != 1 {
		return model, fmt.Errorf("%w: verification returned an invalid result count", domain.ErrInvalid)
	}
	if _, err := validateOutput(result.Images[0], model.DefaultSize, model.DefaultFormat); err != nil {
		return model, fmt.Errorf("verify Image Model: %w", err)
	}
	model.VerifiedAt = service.now().UTC()
	model.UpdatedAt = model.VerifiedAt
	model.Version++
	return service.repository.SaveModel(ctx, model)
}

func containsMode(modes []domain.Mode, wanted domain.Mode) bool {
	for _, mode := range modes {
		if mode == wanted {
			return true
		}
	}
	return false
}

func (service *Service) SetImageModelAvailability(ctx context.Context, administratorID, modelID string, available bool) (domain.ImageModelRevision, error) {
	if strings.TrimSpace(administratorID) == "" {
		return domain.ImageModelRevision{}, fmt.Errorf("%w: Administrator is required", domain.ErrInvalid)
	}
	model, err := service.repository.GetModel(ctx, modelID)
	if err != nil {
		return model, err
	}
	if model.State == domain.ModelDeleted {
		return model, domain.ErrConflict
	}
	if available {
		if model.VerifiedAt.IsZero() {
			return model, fmt.Errorf("%w: Image Model is unverified", domain.ErrInvalid)
		}
		model.State = domain.ModelAvailable
	} else {
		model.State = domain.ModelDisabled
	}
	model.UpdatedAt = service.now().UTC()
	model.Version++
	return service.repository.SaveModel(ctx, model)
}

func (service *Service) ListImageModels(ctx context.Context, availableOnly bool) ([]domain.ImageModelRevision, error) {
	return service.repository.ListModels(ctx, availableOnly)
}

type SubmitRequest struct {
	OwnerID            string
	RequestID          string
	OriginalPrompt     string
	Timezone           string
	ModelID            string
	ReferenceUploadIDs []string
	Generation         domain.GenerationRequest
}

func (service *Service) Submit(ctx context.Context, request SubmitRequest) (domain.ImageGenerationRecord, error) {
	if strings.TrimSpace(request.OwnerID) == "" {
		return domain.ImageGenerationRecord{}, fmt.Errorf("%w: User is required", domain.ErrInvalid)
	}
	if len([]rune(request.OriginalPrompt)) > 10_000 {
		return domain.ImageGenerationRecord{}, fmt.Errorf("%w: original prompt is invalid", domain.ErrInvalid)
	}
	if strings.TrimSpace(request.OriginalPrompt) == "" {
		request.OriginalPrompt = request.Generation.Prompt
	}
	fingerprint, err := requestFingerprint(request)
	if err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	if request.RequestID != "" {
		existing, err := service.repository.GetRecordByRequestID(ctx, request.OwnerID, request.RequestID)
		if err == nil {
			if existing.RequestFingerprint != fingerprint {
				return domain.ImageGenerationRecord{}, domain.ErrConflict
			}
			return existing, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.ImageGenerationRecord{}, err
		}
	}
	request.Generation.References = append([]domain.ReferenceImage(nil), request.Generation.References...)
	model, err := service.repository.GetModel(ctx, request.ModelID)
	if err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	if len(request.ReferenceUploadIDs) > 0 {
		uploads, err := service.repository.GetReferenceUploads(ctx, request.OwnerID, request.ReferenceUploadIDs)
		if err != nil {
			return domain.ImageGenerationRecord{}, err
		}
		if len(uploads) != len(request.ReferenceUploadIDs) {
			return domain.ImageGenerationRecord{}, domain.ErrNotFound
		}
		byID := make(map[string]domain.ReferenceUpload, len(uploads))
		for _, upload := range uploads {
			byID[upload.ID] = upload
		}
		request.Generation.References = make([]domain.ReferenceImage, 0, len(request.ReferenceUploadIDs))
		for position, id := range request.ReferenceUploadIDs {
			upload, ok := byID[id]
			if !ok || upload.BoundAt != nil || !service.now().Before(upload.Image.ExpiresAt) {
				return domain.ImageGenerationRecord{}, domain.ErrNotFound
			}
			image := upload.Image
			image.Position, image.SourceID = position+1, id
			request.Generation.References = append(request.Generation.References, image)
		}
	}
	if err := model.ValidateRequest(request.Generation); err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	amount, err := model.MaximumReservation(request.Generation.Size, request.Generation.Quality, request.Generation.Count)
	if err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	now := service.now().UTC()
	if request.RequestID == "" {
		request.RequestID = uuid.NewString()
	}
	for index := range request.Generation.References {
		request.Generation.References[index].ExpiresAt = now.Add(90 * 24 * time.Hour)
	}
	record := domain.ImageGenerationRecord{
		ID: uuid.NewString(), RequestID: request.RequestID, RequestFingerprint: fingerprint, OwnerID: request.OwnerID, Model: model, OriginalPrompt: request.OriginalPrompt,
		Prompt: request.Generation.Prompt, Request: request.Generation, RequestedCount: request.Generation.Count,
		ReservationAmount: amount, State: domain.StatePending, CreatedAt: now, Version: 1,
	}
	stagedCopies := make([]string, 0, len(record.Request.References))
	for index := range record.Request.References {
		reference := &record.Request.References[index]
		reader, metadata, err := service.objects.Get(ctx, reference.ObjectKey)
		if err != nil {
			service.deleteStagedReferences(ctx, record.OwnerID, stagedCopies)
			return domain.ImageGenerationRecord{}, fmt.Errorf("open Reference Image: %w", err)
		}
		key := fmt.Sprintf("ai-creation/records/%s/%s/inputs/%d", record.OwnerID, record.ID, reference.Position)
		stagingID := uuid.NewString()
		staged := domain.ReferenceUpload{ID: stagingID, OwnerID: record.OwnerID, CreatedAt: now, Image: domain.ReferenceImage{ObjectKey: key, SHA256: reference.SHA256, MediaType: reference.MediaType, Size: metadata.Size, Width: reference.Width, Height: reference.Height, ExpiresAt: now.Add(24 * time.Hour)}}
		if _, err := service.repository.SaveReferenceUpload(ctx, staged); err != nil {
			_ = reader.Close()
			service.deleteStagedReferences(ctx, record.OwnerID, stagedCopies)
			return domain.ImageGenerationRecord{}, err
		}
		stagedCopies = append(stagedCopies, stagingID)
		putErr := service.objects.Put(ctx, key, reader, ObjectMetadata{Size: metadata.Size, SHA256: reference.SHA256, ContentType: reference.MediaType})
		closeErr := reader.Close()
		if putErr != nil || closeErr != nil {
			service.deleteStagedReferences(ctx, record.OwnerID, stagedCopies)
			if putErr != nil {
				return domain.ImageGenerationRecord{}, putErr
			}
			return domain.ImageGenerationRecord{}, closeErr
		}
		reference.ObjectKey = key
		reference.SourceID = stagingID
	}
	stored, err := service.repository.SubmitRecord(ctx, record, request.Timezone)
	if err != nil || stored.ID != record.ID {
		service.deleteStagedReferences(ctx, record.OwnerID, stagedCopies)
	}
	return stored, err
}

func requestFingerprint(request SubmitRequest) (string, error) {
	payload := struct {
		ModelID            string
		OriginalPrompt     string
		ReferenceUploadIDs []string
		Generation         domain.GenerationRequest
	}{request.ModelID, request.OriginalPrompt, request.ReferenceUploadIDs, request.Generation}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("fingerprint Image Generation submission: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (service *Service) deleteStagedReferences(ctx context.Context, ownerID string, ids []string) {
	for _, id := range ids {
		_ = service.DeleteReference(ctx, ownerID, id)
	}
}

func (service *Service) UploadReference(ctx context.Context, ownerID string, data []byte) (domain.ReferenceUpload, error) {
	if strings.TrimSpace(ownerID) == "" || len(data) == 0 || len(data) > 20*1024*1024 {
		return domain.ReferenceUpload{}, fmt.Errorf("%w: Reference Image size is invalid", domain.ErrInvalid)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || int64(config.Width)*int64(config.Height) > 64_000_000 {
		return domain.ReferenceUpload{}, fmt.Errorf("%w: Reference Image cannot be decoded", domain.ErrInvalid)
	}
	format = normalizeFormat(format)
	if format != "png" && format != "jpeg" && format != "webp" {
		return domain.ReferenceUpload{}, fmt.Errorf("%w: Reference Image format is unsupported", domain.ErrInvalid)
	}
	now, id := service.now().UTC(), uuid.NewString()
	digest := sha256.Sum256(data)
	metadata := ObjectMetadata{Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), ContentType: "image/" + format}
	key := fmt.Sprintf("ai-creation/temp/%s/%s", ownerID, id)
	upload := domain.ReferenceUpload{ID: id, OwnerID: ownerID, CreatedAt: now, Image: domain.ReferenceImage{ObjectKey: key, SHA256: metadata.SHA256, MediaType: metadata.ContentType, Size: metadata.Size, Width: config.Width, Height: config.Height, ExpiresAt: now.Add(24 * time.Hour)}}
	stored, err := service.repository.SaveReferenceUpload(ctx, upload)
	if err != nil {
		return domain.ReferenceUpload{}, err
	}
	if err := service.objects.Put(ctx, key, bytes.NewReader(data), metadata); err != nil {
		if service.objects.Delete(ctx, key) == nil {
			_, _ = service.repository.DeleteReferenceUpload(ctx, ownerID, id)
		}
		return domain.ReferenceUpload{}, err
	}
	return stored, nil
}

func (service *Service) DeleteReference(ctx context.Context, ownerID, uploadID string) error {
	uploads, err := service.repository.GetReferenceUploads(ctx, ownerID, []string{uploadID})
	if err != nil || len(uploads) != 1 || uploads[0].BoundAt != nil {
		if err != nil {
			return err
		}
		return domain.ErrNotFound
	}
	if err := service.objects.Delete(ctx, uploads[0].Image.ObjectKey); err != nil {
		return err
	}
	_, err = service.repository.DeleteReferenceUpload(ctx, ownerID, uploadID)
	return err
}

func (service *Service) GetRecord(ctx context.Context, ownerID, recordID string) (domain.ImageGenerationRecord, error) {
	return service.repository.GetRecord(ctx, ownerID, recordID)
}

func (service *Service) ListRecords(ctx context.Context, ownerID string, limit int) ([]domain.ImageGenerationRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return service.repository.ListRecords(ctx, ownerID, limit)
}

func (service *Service) ListEvents(ctx context.Context, ownerID, recordID string, after int64) ([]GenerationEvent, error) {
	return service.repository.ListEvents(ctx, ownerID, recordID, after, 100)
}

func (service *Service) Stop(ctx context.Context, ownerID, recordID string) (domain.ImageGenerationRecord, error) {
	record, err := service.repository.GetRecord(ctx, ownerID, recordID)
	if err != nil {
		return record, err
	}
	if record.State == domain.StateCancelled {
		return record, nil
	}
	if err := record.Cancel(record.ValidatedCount, service.now().UTC()); err != nil {
		return record, err
	}
	return service.repository.SettleRecord(ctx, record)
}

func (service *Service) Regenerate(ctx context.Context, ownerID, timezone, recordID, requestID string) (domain.ImageGenerationRecord, error) {
	original, err := service.repository.GetRecord(ctx, ownerID, recordID)
	if err != nil {
		return domain.ImageGenerationRecord{}, err
	}
	if !original.State.Terminal() {
		return domain.ImageGenerationRecord{}, domain.ErrConflict
	}
	for _, reference := range original.Request.References {
		if !service.now().Before(reference.ExpiresAt) {
			return domain.ImageGenerationRecord{}, fmt.Errorf("%w: Reference Image expired", domain.ErrInvalid)
		}
	}
	generation := original.Request
	generation.References = append([]domain.ReferenceImage(nil), original.Request.References...)
	for index := range generation.References {
		generation.References[index].SourceID = ""
	}
	return service.Submit(ctx, SubmitRequest{OwnerID: ownerID, RequestID: requestID, OriginalPrompt: original.OriginalPrompt, Timezone: timezone, ModelID: original.Model.ID, Generation: generation})
}

func (service *Service) Delete(ctx context.Context, ownerID, recordID string) error {
	record, err := service.repository.DeleteRecord(ctx, ownerID, recordID, service.now().UTC())
	if err != nil {
		return err
	}
	var firstErr error
	for _, reference := range record.Request.References {
		if err := service.objects.Delete(ctx, reference.ObjectKey); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := service.repository.ForgetExpiredObject(ctx, ExpiredObject{Kind: "reference", RecordID: record.ID, Position: reference.Position, Key: reference.ObjectKey}, service.now().UTC()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, image := range record.Images {
		if err := service.objects.Delete(ctx, image.ObjectKey); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := service.repository.ForgetExpiredObject(ctx, ExpiredObject{Kind: "output", RecordID: record.ID, Position: image.Position, Key: image.ObjectKey}, service.now().UTC()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return service.repository.FinalizeDeletedRecord(ctx, record.ID)
}

func (service *Service) ProcessNext(ctx context.Context) (bool, error) {
	now := service.now().UTC()
	resolved, err := service.repository.ResolveExpiredDispatch(ctx, now)
	if err != nil || resolved {
		return resolved, err
	}
	record, claimed, err := service.repository.ClaimNext(ctx, now)
	if err != nil || !claimed {
		return claimed, err
	}
	record.DispatchMarked = true
	record.Version++
	if _, err := service.repository.UpdateRecord(ctx, record); err != nil {
		current, getErr := service.repository.GetRecord(ctx, record.OwnerID, record.ID)
		if getErr == nil && current.State.Terminal() {
			return true, nil
		}
		return true, err
	}
	providerCtx, cancelProvider := context.WithTimeout(ctx, 4*time.Minute)
	monitorDone := make(chan struct{})
	go service.monitorCancellation(providerCtx, cancelProvider, record.OwnerID, record.ID, monitorDone)
	result, providerErr := service.provider.Create(providerCtx, providerRequest(record.Model, record.Request))
	cancelProvider()
	<-monitorDone
	current, err := service.repository.GetRecord(ctx, record.OwnerID, record.ID)
	if err != nil {
		return true, err
	}
	if current.State.Terminal() {
		return true, nil
	}
	record = current
	if providerErr != nil {
		record.SafeError = "image_provider_unavailable"
		var failure *ProviderFailure
		if errors.As(providerErr, &failure) {
			record.SafeError = failure.Code
			if failure.OutcomeUnknown {
				if err := record.Unknown(service.now().UTC()); err != nil {
					return true, err
				}
				_, err = service.repository.SettleRecord(ctx, record)
				return true, err
			}
		}
		if err := record.Complete(0, service.now().UTC()); err != nil {
			return true, err
		}
		_, err = service.repository.SettleRecord(ctx, record)
		return true, err
	}
	images := make([]domain.GeneratedImage, 0, min(len(result.Images), record.RequestedCount))
	for position, data := range result.Images {
		if position >= record.RequestedCount {
			break
		}
		metadata, err := validateOutput(data, record.Request.Size, record.Request.Format)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("ai-creation/records/%s/%s/outputs/%d", record.OwnerID, record.ID, position+1)
		if err := service.repository.TrackObject(ctx, key, service.now().UTC().Add(24*time.Hour)); err != nil {
			continue
		}
		if err := service.objects.Put(ctx, key, bytes.NewReader(data), metadata); err != nil {
			if service.objects.Delete(ctx, key) == nil {
				_ = service.repository.UntrackObject(ctx, key)
			}
			continue
		}
		images = append(images, domain.GeneratedImage{
			Position: position + 1, ObjectKey: key, SHA256: metadata.SHA256, MediaType: metadata.ContentType,
			Size: metadata.Size, Width: parseDimension(record.Request.Size, 0), Height: parseDimension(record.Request.Size, 1),
			ExpiresAt: record.CreatedAt.Add(90 * 24 * time.Hour),
		})
	}
	record.Images = images
	if len(images) < len(result.Images) {
		record.SafeError = "image_output_invalid"
	}
	consumed := int64(0)
	if len(images) > 0 {
		consumed, err = record.Model.MaximumReservation(record.Request.Size, record.Request.Quality, len(images))
		if err != nil {
			return true, err
		}
	}
	record.ConsumptionAmount = consumed
	if err := record.Complete(len(images), service.now().UTC()); err != nil {
		return true, err
	}
	if _, err = service.repository.SettleRecord(ctx, record); err != nil {
		for _, image := range images {
			if service.objects.Delete(ctx, image.ObjectKey) == nil {
				_ = service.repository.UntrackObject(ctx, image.ObjectKey)
			}
		}
		current, getErr := service.repository.GetRecord(ctx, record.OwnerID, record.ID)
		if getErr == nil && current.State.Terminal() {
			return true, nil
		}
		return true, err
	}
	return true, nil
}

func (service *Service) monitorCancellation(ctx context.Context, cancel context.CancelFunc, ownerID, recordID string, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			record, err := service.repository.GetRecord(ctx, ownerID, recordID)
			if err == nil && record.State.Terminal() {
				cancel()
				return
			}
		}
	}
}

func (service *Service) CleanupExpired(ctx context.Context) (int, error) {
	now := service.now().UTC()
	items, err := service.repository.ListExpiredObjects(ctx, now, 100)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, item := range items {
		if err := service.objects.Delete(ctx, item.Key); err != nil {
			return removed, err
		}
		if err := service.repository.ForgetExpiredObject(ctx, item, now); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func providerRequest(model domain.ImageModelRevision, request domain.GenerationRequest) ProviderRequest {
	return ProviderRequest{
		ConnectionID: model.ConnectionID, ConnectionVersion: model.ConnectionVersion, ModelID: model.ModelID,
		Prompt: request.Prompt, Inputs: request.References, Size: request.Size, Quality: request.Quality,
		Format: request.Format, Background: request.Background, Count: request.Count,
	}
}

func validateOutput(data []byte, requestedSize, requestedFormat string) (ObjectMetadata, error) {
	if len(data) == 0 || len(data) > 25*1024*1024 {
		return ObjectMetadata{}, fmt.Errorf("%w: generated image size is invalid", domain.ErrInvalid)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width*config.Height > 64_000_000 {
		return ObjectMetadata{}, fmt.Errorf("%w: generated image cannot be decoded", domain.ErrInvalid)
	}
	wantWidth, wantHeight := parseDimension(requestedSize, 0), parseDimension(requestedSize, 1)
	if config.Width != wantWidth || config.Height != wantHeight || normalizeFormat(format) != normalizeFormat(requestedFormat) {
		return ObjectMetadata{}, fmt.Errorf("%w: generated image does not match the request", domain.ErrInvalid)
	}
	digest := sha256.Sum256(data)
	return ObjectMetadata{Size: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), ContentType: "image/" + normalizeFormat(format)}, nil
}

func parseDimension(size string, part int) int {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0
	}
	value, _ := strconv.Atoi(parts[part])
	return value
}

func normalizeFormat(format string) string {
	if format == "jpg" {
		return "jpeg"
	}
	return strings.ToLower(format)
}
