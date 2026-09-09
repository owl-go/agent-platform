package domain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalid         = errors.New("AI Creation value is invalid")
	ErrConflict        = errors.New("AI Creation state conflicts with current state")
	ErrVersionConflict = errors.New("AI Creation version conflicts with current state")
	ErrNotFound        = errors.New("AI Creation resource not found")
)

type Mode string

const (
	ModeGenerate Mode = "generate"
	ModeEdit     Mode = "edit"
)

type ModelState string

const (
	ModelUnverified ModelState = "unverified"
	ModelAvailable  ModelState = "available"
	ModelDisabled   ModelState = "disabled"
	ModelDeleted    ModelState = "deleted"
)

const ProtocolOpenAIImages = "openai_images"

type ImageModelRevision struct {
	ID                 string
	RevisionID         string
	PredecessorID      string
	PredecessorVersion int64
	DisplayName        string
	Endpoint           string
	APIKeyConfigured   bool
	ModelID            string
	Protocol           string
	Modes              []Mode
	Sizes              []string
	Qualities          []string
	Formats            []string
	Backgrounds        []string
	DefaultSize        string
	DefaultQuality     string
	DefaultFormat      string
	DefaultBackground  string
	Rates              map[string]int64
	State              ModelState
	VerifiedAt         time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Version            int64
}

type GenerationRequest struct {
	Mode       Mode
	Prompt     string
	Size       string
	Quality    string
	Format     string
	Background string
	Count      int
	References []ReferenceImage
}

func (model ImageModelRevision) ValidateRequest(request GenerationRequest) error {
	if model.State != ModelAvailable || model.VerifiedAt.IsZero() {
		return fmt.Errorf("%w: Image Model is unavailable", ErrInvalid)
	}
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" || len([]rune(request.Prompt)) > 10_000 || request.Count < 1 || request.Count > 4 {
		return fmt.Errorf("%w: prompt and output count are invalid", ErrInvalid)
	}
	if !contains(model.Modes, request.Mode) || !validImageSize(request.Size) || !contains(model.Qualities, request.Quality) || !contains(model.Formats, request.Format) || !contains(model.Backgrounds, request.Background) {
		return fmt.Errorf("%w: unsupported Image Model option", ErrInvalid)
	}
	if request.Format == "jpeg" && request.Background == "transparent" {
		return fmt.Errorf("%w: JPEG cannot have a transparent background", ErrInvalid)
	}
	if request.Mode == ModeGenerate && len(request.References) != 0 {
		return fmt.Errorf("%w: text-to-image cannot include Reference Images", ErrInvalid)
	}
	if request.Mode == ModeEdit && (len(request.References) < 1 || len(request.References) > 10) {
		return fmt.Errorf("%w: image-to-image requires one to ten Reference Images", ErrInvalid)
	}
	return nil
}

func (model ImageModelRevision) MaximumReservation(size, quality string, count int) (int64, error) {
	if count < 1 || count > 4 {
		return 0, fmt.Errorf("%w: output count is invalid", ErrInvalid)
	}
	rate, ok := model.Rates[rateKey(size, quality)]
	if !ok && validImageSize(size) {
		rate, ok = model.Rates[rateKey(model.DefaultSize, quality)]
	}
	if !ok || rate < 0 {
		return 0, fmt.Errorf("%w: Image Credit Rate is missing", ErrInvalid)
	}
	return rate * int64(count), nil
}

func (model ImageModelRevision) ValidateConfiguration() error {
	if strings.TrimSpace(model.DisplayName) == "" || strings.TrimSpace(model.ModelID) == "" || model.Protocol != ProtocolOpenAIImages || !model.APIKeyConfigured {
		return fmt.Errorf("%w: incomplete Image Model identity", ErrInvalid)
	}
	if err := ValidateAPIEndpoint(model.Endpoint); err != nil {
		return err
	}
	if len(model.Modes) == 0 || len(model.Sizes) == 0 || len(model.Qualities) == 0 || len(model.Formats) == 0 || len(model.Backgrounds) == 0 {
		return fmt.Errorf("%w: Image Model capabilities are incomplete", ErrInvalid)
	}
	if !contains(model.Sizes, model.DefaultSize) || !contains(model.Qualities, model.DefaultQuality) || !contains(model.Formats, model.DefaultFormat) || !contains(model.Backgrounds, model.DefaultBackground) {
		return fmt.Errorf("%w: Image Model defaults are unsupported", ErrInvalid)
	}
	for _, size := range model.Sizes {
		for _, quality := range model.Qualities {
			if rate, ok := model.Rates[rateKey(size, quality)]; !ok || rate < 0 {
				return fmt.Errorf("%w: Image Credit Rates are incomplete", ErrInvalid)
			}
		}
	}
	return nil
}

func ValidateAPIEndpoint(value string) error {
	endpoint, err := url.Parse(strings.TrimSpace(value))
	if err != nil || endpoint == nil {
		return fmt.Errorf("%w: API Endpoint must be an absolute HTTP or HTTPS URL", ErrInvalid)
	}
	scheme := strings.ToLower(endpoint.Scheme)
	if (scheme != "http" && scheme != "https") || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return fmt.Errorf("%w: API Endpoint must be an absolute HTTP or HTTPS URL", ErrInvalid)
	}
	return nil
}

func RateKey(size, quality string) string { return rateKey(size, quality) }
func rateKey(size, quality string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(size)) + "." + base64.RawURLEncoding.EncodeToString([]byte(quality))
}

func validImageSize(size string) bool {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return false
	}
	width, widthErr := strconv.ParseInt(parts[0], 10, 32)
	height, heightErr := strconv.ParseInt(parts[1], 10, 32)
	return widthErr == nil && heightErr == nil && width > 0 && height > 0 && width <= 64_000_000 && height <= 64_000_000 && width*height <= 64_000_000
}

func contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

type State string

const (
	StatePending            State = "pending"
	StateRunning            State = "running"
	StateSucceeded          State = "succeeded"
	StatePartiallySucceeded State = "partially_succeeded"
	StateFailed             State = "failed"
	StateCancelled          State = "cancelled"
	StateOutcomeUnknown     State = "outcome_unknown"
)

func (state State) Terminal() bool {
	switch state {
	case StateSucceeded, StatePartiallySucceeded, StateFailed, StateCancelled, StateOutcomeUnknown:
		return true
	default:
		return false
	}
}

type ReferenceImage struct {
	Position    int
	ObjectKey   string
	SHA256      string
	MediaType   string
	Size        int64
	Width       int
	Height      int
	ExpiresAt   time.Time
	SourceID    string
	SourceImage int
}

type ReferenceUpload struct {
	ID        string
	OwnerID   string
	Image     ReferenceImage
	CreatedAt time.Time
	BoundAt   *time.Time
}

type PromptOptimizationCandidate struct {
	ProviderModelID  string
	DisplayName      string
	ProviderType     string
	ModelID          string
	Protocol         string
	Endpoint         string
	APIKeyConfigured bool
	Instruction      string
}

type GeneratedImage struct {
	Position  int
	ObjectKey string
	SHA256    string
	MediaType string
	Size      int64
	Width     int
	Height    int
	ExpiresAt time.Time
}

type ImageGenerationRecord struct {
	ID                   string
	RequestID            string
	RequestFingerprint   string
	OwnerID              string
	Model                ImageModelRevision
	OriginalPrompt       string
	Prompt               string
	Request              GenerationRequest
	RequestedCount       int
	ValidatedCount       int
	ReservationAmount    int64
	ConsumptionAmount    int64
	ReservationCreditDay string
	State                State
	DispatchMarked       bool
	CancellationAsked    bool
	SafeError            string
	Images               []GeneratedImage
	CreatedAt            time.Time
	StartedAt            *time.Time
	CompletedAt          *time.Time
	Version              int64
}

func (record *ImageGenerationRecord) Start(at time.Time) error {
	if record.State != StatePending {
		return ErrConflict
	}
	record.State = StateRunning
	record.StartedAt = &at
	record.Version++
	return nil
}

func (record *ImageGenerationRecord) Complete(validated int, at time.Time) error {
	if record.State != StateRunning || validated < 0 || validated > record.RequestedCount {
		return ErrConflict
	}
	record.ValidatedCount = validated
	switch {
	case validated == record.RequestedCount:
		record.State = StateSucceeded
	case validated > 0:
		record.State = StatePartiallySucceeded
	default:
		record.State = StateFailed
	}
	record.CompletedAt = &at
	record.Version++
	return nil
}

func (record *ImageGenerationRecord) Cancel(validated int, at time.Time) error {
	if record.State.Terminal() || (record.State != StatePending && record.State != StateRunning) || validated < 0 || validated > record.RequestedCount {
		return ErrConflict
	}
	record.CancellationAsked = true
	record.ValidatedCount = validated
	record.State = StateCancelled
	record.CompletedAt = &at
	record.Version++
	return nil
}

func (record *ImageGenerationRecord) Unknown(at time.Time) error {
	if record.State != StateRunning || !record.DispatchMarked {
		return ErrConflict
	}
	record.State = StateOutcomeUnknown
	record.CompletedAt = &at
	record.Version++
	return nil
}
