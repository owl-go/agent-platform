package domain_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"agent-platform/backend/internal/biz/aicreation/domain"
)

func TestRateKeyCanBeStoredInPostgresJSONB(t *testing.T) {
	encoded, err := json.Marshal(map[string]int64{domain.RateKey("1024x1024", "auto"): 100})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `\u0000`) {
		t.Fatalf("encoded rates contain PostgreSQL-unsupported NUL escape: %s", encoded)
	}
}

func TestAvailableImageModelAcceptsPublishedCapabilitiesAndCustomSize(t *testing.T) {
	model := availableModel()

	if err := model.ValidateRequest(domain.GenerationRequest{
		Mode: domain.ModeGenerate, Prompt: "a brass telescope on a desk", Size: "1024x1024",
		Quality: "high", Format: "png", Background: "transparent", Count: 2,
	}); err != nil {
		t.Fatalf("ValidateRequest() error = %v", err)
	}
	if err := model.ValidateRequest(domain.GenerationRequest{
		Mode: domain.ModeGenerate, Prompt: "a widescreen landscape", Size: "1920x1080",
		Quality: "high", Format: "png", Background: "opaque", Count: 1,
	}); err != nil {
		t.Fatalf("ValidateRequest(custom size) error = %v", err)
	}

	invalid := []domain.GenerationRequest{
		{Mode: domain.ModeEdit, Prompt: "change the sky", Size: "1024x1024", Quality: "high", Format: "png", Background: "opaque", Count: 1},
		{Mode: domain.ModeGenerate, Prompt: "x", Size: "1920*1080", Quality: "high", Format: "png", Background: "opaque", Count: 1},
		{Mode: domain.ModeGenerate, Prompt: "x", Size: "8001x8000", Quality: "high", Format: "png", Background: "opaque", Count: 1},
		{Mode: domain.ModeGenerate, Prompt: "x", Size: "1024x1024", Quality: "high", Format: "jpeg", Background: "transparent", Count: 1},
		{Mode: domain.ModeGenerate, Prompt: "x", Size: "1024x1024", Quality: "high", Format: "png", Background: "opaque", Count: 5},
	}
	for _, request := range invalid {
		if err := model.ValidateRequest(request); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("ValidateRequest(%+v) error = %v, want ErrInvalid", request, err)
		}
	}
}

func TestImageGenerationRecordHasMonotonicTerminalState(t *testing.T) {
	record := domain.ImageGenerationRecord{ID: "record-1", OwnerID: "user-1", State: domain.StatePending, RequestedCount: 2}
	if err := record.Start(time.Unix(10, 0)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := record.Complete(1, time.Unix(20, 0)); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if record.State != domain.StatePartiallySucceeded {
		t.Fatalf("State = %q, want %q", record.State, domain.StatePartiallySucceeded)
	}
	if err := record.Complete(2, time.Unix(30, 0)); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second Complete() error = %v, want ErrConflict", err)
	}
}

func TestCancellationRetainsValidatedOutputs(t *testing.T) {
	record := domain.ImageGenerationRecord{ID: "record-1", OwnerID: "user-1", State: domain.StateRunning, RequestedCount: 3}
	if err := record.Cancel(2, time.Unix(20, 0)); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if record.State != domain.StateCancelled || record.ValidatedCount != 2 {
		t.Fatalf("record = %+v, want cancelled with two outputs", record)
	}
}

func TestImageCreditReservationUsesFullRequestedBatch(t *testing.T) {
	model := availableModel()
	got, err := model.MaximumReservation("1024x1024", "high", 4)
	if err != nil {
		t.Fatalf("MaximumReservation() error = %v", err)
	}
	if got != 500 {
		t.Fatalf("MaximumReservation() = %d, want 500 hundredths", got)
	}
}

func TestCustomImageSizeUsesTheDefaultSizeRate(t *testing.T) {
	model := availableModel()
	got, err := model.MaximumReservation("1920x1080", "high", 2)
	if err != nil {
		t.Fatalf("MaximumReservation() error = %v", err)
	}
	if got != 250 {
		t.Fatalf("MaximumReservation() = %d, want 250 hundredths", got)
	}
}

func availableModel() domain.ImageModelRevision {
	return domain.ImageModelRevision{
		ID: "model-1", RevisionID: "revision-1", State: domain.ModelAvailable,
		Endpoint: "https://images.example.test/v1", APIKeyConfigured: true,
		ModelID: "gpt-image-1", Protocol: domain.ProtocolOpenAIImages,
		Modes: []domain.Mode{domain.ModeGenerate}, Sizes: []string{"1024x1024"},
		Qualities: []string{"high"}, Formats: []string{"png", "jpeg"},
		Backgrounds: []string{"opaque", "transparent"},
		DefaultSize: "1024x1024", DefaultQuality: "high", DefaultFormat: "png", DefaultBackground: "opaque",
		Rates: map[string]int64{domain.RateKey("1024x1024", "high"): 125}, VerifiedAt: time.Unix(1, 0),
	}
}
