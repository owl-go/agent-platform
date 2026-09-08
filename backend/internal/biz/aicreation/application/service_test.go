package application_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/aicreation/application"
	"agent-platform/backend/internal/biz/aicreation/domain"
)

func TestAdministratorPublishesModelAndUserGeneratesImage(t *testing.T) {
	ctx := context.Background()
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 10_000}
	provider := &fakeProvider{outputs: [][]byte{onePixelPNG(t)}}
	objects := newMemoryObjects()
	service := mustService(t, repository, credits, provider, objects)

	model, err := service.CreateImageModel(ctx, application.CreateImageModelRequest{
		AdministratorID: "admin-1", DisplayName: "Studio", ConnectionID: "connection-1", ConnectionVersion: 1,
		ConnectionName: "OpenAI", ModelID: "gpt-image-1", Modes: []domain.Mode{domain.ModeGenerate},
		Sizes: []string{"1x1"}, Qualities: []string{"high"}, Formats: []string{"png"}, Backgrounds: []string{"opaque"},
		DefaultSize: "1x1", DefaultQuality: "high", DefaultFormat: "png", DefaultBackground: "opaque",
		Rates: map[string]int64{domain.RateKey("1x1", "high"): 125},
	})
	if err != nil {
		t.Fatalf("CreateImageModel() error = %v", err)
	}
	if model.State != domain.ModelUnverified {
		t.Fatalf("State = %q, want unverified", model.State)
	}
	if _, err := service.VerifyImageModel(ctx, "admin-1", model.ID); err != nil {
		t.Fatalf("VerifyImageModel() error = %v", err)
	}
	model, err = service.SetImageModelAvailability(ctx, "admin-1", model.ID, true)
	if err != nil {
		t.Fatalf("SetImageModelAvailability() error = %v", err)
	}

	record, err := service.Submit(ctx, application.SubmitRequest{
		OwnerID: "user-1", Timezone: "Asia/Shanghai", ModelID: model.ID,
		Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "a red dot", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if record.State != domain.StatePending || credits.reserved != 125 {
		t.Fatalf("record = %+v, reserved = %d", record, credits.reserved)
	}
	if _, err := service.Submit(ctx, application.SubmitRequest{OwnerID: "user-1", Timezone: "Asia/Shanghai", ModelID: model.ID, Generation: record.Request}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second Submit() error = %v, want conflict", err)
	}

	worked, err := service.ProcessNext(ctx)
	if err != nil || !worked {
		t.Fatalf("ProcessNext() = %v, %v", worked, err)
	}
	got, err := service.GetRecord(ctx, "user-1", record.ID)
	if err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}
	if got.State != domain.StateSucceeded || len(got.Images) != 1 || got.ConsumptionAmount != 125 {
		t.Fatalf("record = %+v, want one settled image", got)
	}
	if credits.consumed != 125 || credits.reserved != 0 {
		t.Fatalf("credits = %+v", credits)
	}
	if _, ok := objects.data[got.Images[0].ObjectKey]; !ok {
		t.Fatalf("generated object %q was not stored", got.Images[0].ObjectKey)
	}
}

func TestStopPendingRecordReleasesReservation(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000}
	service := mustService(t, repository, credits, &fakeProvider{}, newMemoryObjects())
	model := publishedModel()
	repository.models[model.ID] = model
	record, err := service.Submit(context.Background(), application.SubmitRequest{
		OwnerID: "user-1", Timezone: "UTC", ModelID: model.ID,
		Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Stop(context.Background(), "user-1", record.ID)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got.State != domain.StateCancelled || credits.reserved != 0 {
		t.Fatalf("record = %+v, credits = %+v", got, credits)
	}
}

func TestSubmitWithSameRequestIDReturnsOriginalRecord(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000}
	service := mustService(t, repository, credits, &fakeProvider{}, newMemoryObjects())
	model := publishedModel()
	repository.models[model.ID] = model
	request := application.SubmitRequest{OwnerID: "user-1", RequestID: "request-1", Timezone: "UTC", ModelID: model.ID, Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}}
	first, err := service.Submit(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Submit(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || credits.reserved != 125 || len(repository.records) != 1 {
		t.Fatalf("first=%+v second=%+v credits=%+v", first, second, credits)
	}
}

func TestSubmitRejectsSameRequestIDForDifferentSubmission(t *testing.T) {
	repository := newMemoryRepository()
	service := mustService(t, repository, &fakeCredits{available: 1_000}, &fakeProvider{}, newMemoryObjects())
	model := publishedModel()
	repository.models[model.ID] = model
	request := application.SubmitRequest{OwnerID: "user-1", RequestID: "request-1", Timezone: "UTC", ModelID: model.ID, Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}}
	if _, err := service.Submit(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	request.Generation.Prompt = "different"
	if _, err := service.Submit(context.Background(), request); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Submit() error = %v, want conflict", err)
	}
}

func TestMaterialImageModelRevisionRequiresNewVerification(t *testing.T) {
	repository := newMemoryRepository()
	service := mustService(t, repository, &fakeCredits{}, &fakeProvider{}, newMemoryObjects())
	current := publishedModel()
	current.Version = 3
	repository.models[current.ID] = current
	revised, err := service.ReviseImageModel(context.Background(), current.ID, current.Version, application.CreateImageModelRequest{
		AdministratorID: "admin-1", DisplayName: "Studio 2", ConnectionID: current.ConnectionID, ConnectionVersion: current.ConnectionVersion,
		ConnectionName: current.ConnectionName, ModelID: current.ModelID, Modes: current.Modes, Sizes: current.Sizes, Qualities: current.Qualities,
		Formats: current.Formats, Backgrounds: current.Backgrounds, DefaultSize: current.DefaultSize, DefaultQuality: current.DefaultQuality,
		DefaultFormat: current.DefaultFormat, DefaultBackground: current.DefaultBackground, Rates: current.Rates,
	})
	if err != nil {
		t.Fatal(err)
	}
	if revised.RevisionID == current.RevisionID || revised.PredecessorID != current.RevisionID || revised.State != domain.ModelUnverified || !revised.VerifiedAt.IsZero() {
		t.Fatalf("revised model = %+v", revised)
	}
}

func TestDeletedImageModelCannotBeRevisedVerifiedOrEnabled(t *testing.T) {
	repository := newMemoryRepository()
	service := mustService(t, repository, &fakeCredits{}, &fakeProvider{}, newMemoryObjects())
	model := publishedModel()
	model.State = domain.ModelDeleted
	repository.models[model.ID] = model
	request := application.CreateImageModelRequest{AdministratorID: "admin-1", DisplayName: model.DisplayName, ConnectionID: model.ConnectionID, ConnectionVersion: model.ConnectionVersion, ConnectionName: model.ConnectionName, ModelID: model.ModelID, Modes: model.Modes, Sizes: model.Sizes, Qualities: model.Qualities, Formats: model.Formats, Backgrounds: model.Backgrounds, DefaultSize: model.DefaultSize, DefaultQuality: model.DefaultQuality, DefaultFormat: model.DefaultFormat, DefaultBackground: model.DefaultBackground, Rates: model.Rates}
	if _, err := service.ReviseImageModel(context.Background(), model.ID, model.Version, request); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("ReviseImageModel() error = %v, want conflict", err)
	}
	if _, err := service.VerifyImageModel(context.Background(), "admin-1", model.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("VerifyImageModel() error = %v, want conflict", err)
	}
	if _, err := service.SetImageModelAvailability(context.Background(), "admin-1", model.ID, true); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("SetImageModelAvailability() error = %v, want conflict", err)
	}
}

func TestStopRunningRecordDiscardsLateProviderOutputs(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000}
	provider := &fakeProvider{outputs: [][]byte{onePixelPNG(t)}, started: make(chan struct{}), release: make(chan struct{})}
	objects := newMemoryObjects()
	service := mustService(t, repository, credits, provider, objects)
	model := publishedModel()
	repository.models[model.ID] = model
	record, err := service.Submit(context.Background(), application.SubmitRequest{
		OwnerID: "user-1", Timezone: "UTC", ModelID: model.ID,
		Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, processErr := service.ProcessNext(context.Background())
		done <- processErr
	}()
	<-provider.started
	if _, err := service.Stop(context.Background(), "user-1", record.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	close(provider.release)
	if err := <-done; err != nil {
		t.Fatalf("ProcessNext() error = %v", err)
	}
	got, err := service.GetRecord(context.Background(), "user-1", record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != domain.StateCancelled || len(got.Images) != 0 || credits.consumed != 0 || len(objects.data) != 0 {
		t.Fatalf("late output was retained: record=%+v credits=%+v objects=%v", got, credits, objects.data)
	}
}

func TestStopRunningRecordCancelsProviderRequest(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000}
	provider := &fakeProvider{started: make(chan struct{}), release: make(chan struct{}), respectContext: true, cancelled: make(chan struct{})}
	service := mustService(t, repository, credits, provider, newMemoryObjects())
	model := publishedModel()
	repository.models[model.ID] = model
	record, err := service.Submit(context.Background(), application.SubmitRequest{OwnerID: "user-1", Timezone: "UTC", ModelID: model.ID, Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, processErr := service.ProcessNext(context.Background()); done <- processErr }()
	<-provider.started
	if _, err := service.Stop(context.Background(), "user-1", record.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("provider request was not cancelled")
	}
	select {
	case <-provider.cancelled:
	default:
		t.Fatal("provider did not observe context cancellation")
	}
}

func TestProcessNextMarksAbandonedDispatchedRecordOutcomeUnknown(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000, reserved: 125}
	service := mustService(t, repository, credits, &fakeProvider{}, newMemoryObjects())
	record := domain.ImageGenerationRecord{ID: "record-1", OwnerID: "user-1", Model: publishedModel(), Request: domain.GenerationRequest{Size: "1x1", Quality: "high"}, RequestedCount: 1, ReservationAmount: 125, ReservationCreditDay: "2026-09-08", State: domain.StateRunning, DispatchMarked: true, Version: 3}
	repository.records[record.ID] = record

	worked, err := service.ProcessNext(context.Background())
	if err != nil || !worked {
		t.Fatalf("ProcessNext() = %v, %v", worked, err)
	}
	got := repository.records[record.ID]
	if got.State != domain.StateOutcomeUnknown || credits.reserved != 0 || credits.consumed != 0 {
		t.Fatalf("record=%+v credits=%+v", got, credits)
	}
}

func TestProcessNextPreservesAmbiguousProviderOutcome(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000}
	provider := &fakeProvider{err: &application.ProviderFailure{Code: "image_outcome_unknown", OutcomeUnknown: true, Cause: errors.New("connection reset")}}
	service := mustService(t, repository, credits, provider, newMemoryObjects())
	model := publishedModel()
	repository.models[model.ID] = model
	record, err := service.Submit(context.Background(), application.SubmitRequest{OwnerID: "user-1", Timezone: "UTC", ModelID: model.ID, Generation: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := repository.records[record.ID]
	if got.State != domain.StateOutcomeUnknown || got.SafeError != "image_outcome_unknown" || credits.reserved != 0 {
		t.Fatalf("record=%+v credits=%+v", got, credits)
	}
}

func TestProcessNextReclaimsAbandonedPreDispatchRecord(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 1_000, reserved: 125}
	provider := &fakeProvider{outputs: [][]byte{onePixelPNG(t)}}
	service := mustService(t, repository, credits, provider, newMemoryObjects())
	record := domain.ImageGenerationRecord{ID: "record-1", OwnerID: "user-1", Model: publishedModel(), Request: domain.GenerationRequest{Mode: domain.ModeGenerate, Prompt: "x", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}, RequestedCount: 1, ReservationAmount: 125, ReservationCreditDay: "2026-09-08", State: domain.StateRunning, Version: 2}
	repository.records[record.ID] = record

	worked, err := service.ProcessNext(context.Background())
	if err != nil || !worked {
		t.Fatalf("ProcessNext() = %v, %v", worked, err)
	}
	if got := repository.records[record.ID]; got.State != domain.StateSucceeded {
		t.Fatalf("record=%+v", got)
	}
}

func TestCleanupExpiredRemovesPrivateBytesAndMetadata(t *testing.T) {
	repository := newMemoryRepository()
	repository.expired = []application.ExpiredObject{{Kind: "upload", UploadID: "upload-1", Key: "private/upload-1"}}
	serviceObjects := newMemoryObjects()
	serviceObjects.data["private/upload-1"] = []byte("private")
	service, err := application.New(repository, &fakeProvider{}, serviceObjects, func() time.Time { return time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	removed, err := service.CleanupExpired(context.Background())
	if err != nil || removed != 1 || len(serviceObjects.data) != 0 || len(repository.expired) != 0 {
		t.Fatalf("CleanupExpired() = %d, %v; objects=%v expired=%v", removed, err, serviceObjects.data, repository.expired)
	}
}

func TestRegenerateCopiesReferencesIntoNewRecordNamespace(t *testing.T) {
	repository := newMemoryRepository()
	credits := &fakeCredits{available: 10_000}
	objects := newMemoryObjects()
	provider := &fakeProvider{outputs: [][]byte{onePixelPNG(t)}}
	service := mustService(t, repository, credits, provider, objects)
	model := publishedModel()
	model.Modes = append(model.Modes, domain.ModeEdit)
	repository.models[model.ID] = model
	upload, err := service.UploadReference(context.Background(), "user-1", onePixelPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	original, err := service.Submit(context.Background(), application.SubmitRequest{OwnerID: "user-1", Timezone: "UTC", ModelID: model.ID, ReferenceUploadIDs: []string{upload.ID}, Generation: domain.GenerationRequest{Mode: domain.ModeEdit, Prompt: "edit", Size: "1x1", Quality: "high", Format: "png", Background: "opaque", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessNext(context.Background()); err != nil {
		t.Fatal(err)
	}
	regenerated, err := service.Regenerate(context.Background(), "user-1", "UTC", original.ID, "regeneration-1")
	if err != nil {
		t.Fatal(err)
	}
	first := repository.records[original.ID].Request.References[0].ObjectKey
	second := regenerated.Request.References[0].ObjectKey
	if first == second || first == upload.Image.ObjectKey || second == upload.Image.ObjectKey {
		t.Fatalf("references were not independently copied: upload=%q first=%q second=%q", upload.Image.ObjectKey, first, second)
	}
}

func mustService(t *testing.T, repository *memoryRepository, credits *fakeCredits, provider *fakeProvider, objects *memoryObjects) *application.Service {
	t.Helper()
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	repository.credits = credits
	service, err := application.New(repository, provider, objects, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type memoryRepository struct {
	mu      sync.Mutex
	models  map[string]domain.ImageModelRevision
	records map[string]domain.ImageGenerationRecord
	uploads map[string]domain.ReferenceUpload
	credits *fakeCredits
	expired []application.ExpiredObject
	next    int
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{models: map[string]domain.ImageModelRevision{}, records: map[string]domain.ImageGenerationRecord{}, uploads: map[string]domain.ReferenceUpload{}}
}

func (repo *memoryRepository) SaveReferenceUpload(_ context.Context, upload domain.ReferenceUpload) (domain.ReferenceUpload, error) {
	repo.uploads[upload.ID] = upload
	return upload, nil
}
func (repo *memoryRepository) ListPromptCandidates(context.Context) ([]domain.PromptOptimizationCandidate, error) {
	return nil, nil
}
func (repo *memoryRepository) ReplacePromptCandidates(context.Context, string, []domain.PromptOptimizationCandidate) error {
	return nil
}
func (repo *memoryRepository) GetReferenceUploads(_ context.Context, ownerID string, ids []string) ([]domain.ReferenceUpload, error) {
	result := []domain.ReferenceUpload{}
	for _, id := range ids {
		if upload, ok := repo.uploads[id]; ok && upload.OwnerID == ownerID {
			result = append(result, upload)
		}
	}
	return result, nil
}

func (repo *memoryRepository) DeleteReferenceUpload(_ context.Context, ownerID, id string) (domain.ReferenceUpload, error) {
	upload, ok := repo.uploads[id]
	if !ok || upload.OwnerID != ownerID || upload.BoundAt != nil {
		return domain.ReferenceUpload{}, domain.ErrNotFound
	}
	delete(repo.uploads, id)
	return upload, nil
}

func (repo *memoryRepository) SaveModel(_ context.Context, model domain.ImageModelRevision) (domain.ImageModelRevision, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.next++
	if model.ID == "" {
		model.ID = "model-" + string(rune('0'+repo.next))
	}
	if model.RevisionID == "" {
		model.RevisionID = model.ID + "-revision"
	}
	repo.models[model.ID] = model
	return model, nil
}

func (repo *memoryRepository) GetModel(_ context.Context, id string) (domain.ImageModelRevision, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	model, ok := repo.models[id]
	if !ok {
		return model, domain.ErrNotFound
	}
	return model, nil
}

func (repo *memoryRepository) ListModels(_ context.Context, availableOnly bool) ([]domain.ImageModelRevision, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := []domain.ImageModelRevision{}
	for _, model := range repo.models {
		if !availableOnly || model.State == domain.ModelAvailable {
			result = append(result, model)
		}
	}
	return result, nil
}

func (repo *memoryRepository) DeleteModel(_ context.Context, id string, expectedVersion int64, _ time.Time) error {
	model, ok := repo.models[id]
	if !ok {
		return domain.ErrNotFound
	}
	if model.Version != expectedVersion {
		return domain.ErrVersionConflict
	}
	for _, record := range repo.records {
		if record.Model.ID == id && !record.State.Terminal() {
			return domain.ErrConflict
		}
	}
	model.State, model.Version = domain.ModelDeleted, model.Version+1
	repo.models[id] = model
	return nil
}

func (repo *memoryRepository) SubmitRecord(ctx context.Context, record domain.ImageGenerationRecord, timezone string) (domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, existing := range repo.records {
		if existing.OwnerID == record.OwnerID && existing.RequestID == record.RequestID {
			if existing.RequestFingerprint != record.RequestFingerprint {
				return record, domain.ErrConflict
			}
			return existing, nil
		}
		if existing.OwnerID == record.OwnerID && !existing.State.Terminal() {
			return record, domain.ErrConflict
		}
	}
	creditDay, err := repo.credits.Reserve(ctx, record.OwnerID, record.ID, timezone, record.ReservationAmount)
	if err != nil {
		return record, err
	}
	record.ReservationCreditDay = creditDay
	repo.next++
	if record.ID == "" {
		record.ID = "record-" + string(rune('0'+repo.next))
	}
	repo.records[record.ID] = record
	return record, nil
}

func (repo *memoryRepository) SettleRecord(ctx context.Context, record domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if err := repo.credits.Settle(ctx, record.ID, record.ReservationAmount, record.ConsumptionAmount); err != nil {
		return record, err
	}
	repo.records[record.ID] = record
	return record, nil
}

func (repo *memoryRepository) GetRecord(_ context.Context, ownerID, id string) (domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	record, ok := repo.records[id]
	if !ok || record.OwnerID != ownerID {
		return record, domain.ErrNotFound
	}
	return record, nil
}

func (repo *memoryRepository) GetRecordByRequestID(_ context.Context, ownerID, requestID string) (domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for _, record := range repo.records {
		if record.OwnerID == ownerID && record.RequestID == requestID {
			return record, nil
		}
	}
	return domain.ImageGenerationRecord{}, domain.ErrNotFound
}

func (repo *memoryRepository) ListRecords(_ context.Context, ownerID string, limit int) ([]domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	result := make([]domain.ImageGenerationRecord, 0, limit)
	for _, record := range repo.records {
		if record.OwnerID == ownerID && len(result) < limit {
			result = append(result, record)
		}
	}
	return result, nil
}
func (repo *memoryRepository) ListEvents(context.Context, string, string, int64, int) ([]application.GenerationEvent, error) {
	return nil, nil
}
func (repo *memoryRepository) DeleteRecord(_ context.Context, ownerID, id string, _ time.Time) (domain.ImageGenerationRecord, error) {
	record, ok := repo.records[id]
	if !ok || record.OwnerID != ownerID {
		return record, domain.ErrNotFound
	}
	if !record.State.Terminal() {
		return record, domain.ErrConflict
	}
	delete(repo.records, id)
	return record, nil
}

func (repo *memoryRepository) FinalizeDeletedRecord(context.Context, string) error { return nil }

func (repo *memoryRepository) ClaimNext(_ context.Context, at time.Time) (domain.ImageGenerationRecord, bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for id, record := range repo.records {
		if record.State == domain.StatePending || (record.State == domain.StateRunning && !record.DispatchMarked) {
			if record.State == domain.StateRunning {
				record.State = domain.StatePending
				record.StartedAt = nil
			}
			if err := record.Start(at); err != nil {
				return record, false, err
			}
			repo.records[id] = record
			return record, true, nil
		}
	}
	return domain.ImageGenerationRecord{}, false, nil
}

func (repo *memoryRepository) ResolveExpiredDispatch(_ context.Context, at time.Time) (bool, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	for id, record := range repo.records {
		if record.State == domain.StateRunning && record.DispatchMarked {
			if err := record.Unknown(at); err != nil {
				return false, err
			}
			if err := repo.credits.Settle(context.Background(), record.ID, record.ReservationAmount, 0); err != nil {
				return false, err
			}
			repo.records[id] = record
			return true, nil
		}
	}
	return false, nil
}

func (repo *memoryRepository) ListExpiredObjects(context.Context, time.Time, int) ([]application.ExpiredObject, error) {
	return append([]application.ExpiredObject(nil), repo.expired...), nil
}

func (repo *memoryRepository) ForgetExpiredObject(_ context.Context, item application.ExpiredObject, _ time.Time) error {
	for index, candidate := range repo.expired {
		if candidate == item {
			repo.expired = append(repo.expired[:index], repo.expired[index+1:]...)
			break
		}
	}
	return nil
}

func (repo *memoryRepository) TrackObject(context.Context, string, time.Time) error { return nil }
func (repo *memoryRepository) UntrackObject(context.Context, string) error          { return nil }

func (repo *memoryRepository) UpdateRecord(_ context.Context, record domain.ImageGenerationRecord) (domain.ImageGenerationRecord, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.records[record.ID] = record
	return record, nil
}

type fakeCredits struct{ available, reserved, consumed int64 }

func (credits *fakeCredits) Reserve(_ context.Context, _, _, _ string, amount int64) (string, error) {
	if amount > credits.available-credits.reserved {
		return "", application.ErrInsufficientCredits
	}
	credits.reserved += amount
	return "2026-09-08", nil
}
func (credits *fakeCredits) Settle(_ context.Context, _ string, reserved, consumed int64) error {
	credits.reserved -= reserved
	credits.consumed += consumed
	return nil
}

type fakeProvider struct {
	outputs        [][]byte
	started        chan struct{}
	release        chan struct{}
	respectContext bool
	cancelled      chan struct{}
	err            error
}

func (provider *fakeProvider) Create(ctx context.Context, _ application.ProviderRequest) (application.ProviderResult, error) {
	if provider.started != nil {
		close(provider.started)
	}
	if provider.release != nil {
		if provider.respectContext {
			select {
			case <-provider.release:
			case <-ctx.Done():
				if provider.cancelled != nil {
					close(provider.cancelled)
				}
				return application.ProviderResult{}, ctx.Err()
			}
		} else {
			<-provider.release
		}
	}
	return application.ProviderResult{Images: provider.outputs}, provider.err
}

type memoryObjects struct{ data map[string][]byte }

func newMemoryObjects() *memoryObjects { return &memoryObjects{data: map[string][]byte{}} }
func (objects *memoryObjects) Put(_ context.Context, key string, body io.Reader, _ application.ObjectMetadata) error {
	data, err := io.ReadAll(body)
	if err == nil {
		objects.data[key] = data
	}
	return err
}
func (objects *memoryObjects) Delete(_ context.Context, key string) error {
	delete(objects.data, key)
	return nil
}
func (objects *memoryObjects) Get(_ context.Context, key string) (io.ReadCloser, application.ObjectMetadata, error) {
	data, ok := objects.data[key]
	if !ok {
		return nil, application.ObjectMetadata{}, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), application.ObjectMetadata{Size: int64(len(data))}, nil
}

func onePixelPNG(t *testing.T) []byte {
	t.Helper()
	encoded := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func publishedModel() domain.ImageModelRevision {
	return domain.ImageModelRevision{
		ID: "model-1", RevisionID: "revision-1", DisplayName: "Studio", State: domain.ModelAvailable,
		ConnectionID: "connection-1", ConnectionVersion: 1, ConnectionName: "OpenAI", ModelID: "gpt-image-1", Protocol: domain.ProtocolOpenAIImages,
		Modes: []domain.Mode{domain.ModeGenerate}, Sizes: []string{"1x1"}, Qualities: []string{"high"}, Formats: []string{"png"}, Backgrounds: []string{"opaque"},
		DefaultSize: "1x1", DefaultQuality: "high", DefaultFormat: "png", DefaultBackground: "opaque",
		Rates: map[string]int64{domain.RateKey("1x1", "high"): 125}, VerifiedAt: time.Unix(1, 0),
	}
}
