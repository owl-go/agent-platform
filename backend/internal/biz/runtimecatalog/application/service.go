package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/runtimecatalog/domain"

	"github.com/google/uuid"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

type uuidGenerator struct{}

func (uuidGenerator) NewID() string { return uuid.NewString() }

type Service struct {
	repository domain.Repository
	evidence   EvidenceVerifier
	clock      Clock
	ids        IDGenerator
}

type RegisterCommand struct {
	OrganizationID string
	Runtime        domain.Runtime
	CLIVersion     string
	AdapterVersion string
	ImageDigest    string
	Capabilities   map[string]bool
}

type ChangeStatusCommand struct {
	OrganizationID         string
	ID                     string
	ExpectedVersion        int64
	Status                 domain.Status
	BlockedReason          string
	ConformanceEvidenceKey string
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

var ErrInvalidPage = fmt.Errorf("invalid Runtime Image page")

type ListQuery struct {
	OrganizationID string
	PageSize       int
	Token          string
}

type Page struct {
	Items     []domain.RuntimeImage
	NextToken string
}

func New(repository domain.Repository) *Service {
	return NewWithDependencies(repository, systemClock{}, uuidGenerator{})
}

func NewWithEvidenceVerifier(repository domain.Repository, evidence EvidenceVerifier) *Service {
	return newService(repository, evidence, systemClock{}, uuidGenerator{})
}

func NewWithDependencies(repository domain.Repository, clock Clock, ids IDGenerator) *Service {
	return newService(repository, nil, clock, ids)
}

func NewWithEvidenceDependencies(repository domain.Repository, evidence EvidenceVerifier, clock Clock, ids IDGenerator) *Service {
	return newService(repository, evidence, clock, ids)
}

func newService(repository domain.Repository, evidence EvidenceVerifier, clock Clock, ids IDGenerator) *Service {
	return &Service{repository: repository, evidence: evidence, clock: clock, ids: ids}
}

func (service *Service) Register(ctx context.Context, command RegisterCommand) (domain.RuntimeImage, error) {
	if service.repository == nil || service.clock == nil || service.ids == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Catalog dependencies are required")
	}
	image, err := domain.Register(domain.Registration{
		ID: service.ids.NewID(), OrganizationID: command.OrganizationID, Runtime: command.Runtime, CLIVersion: command.CLIVersion,
		AdapterVersion: command.AdapterVersion, ImageDigest: command.ImageDigest,
		Capabilities: command.Capabilities, Now: service.clock.Now(),
	})
	if err != nil {
		return domain.RuntimeImage{}, err
	}
	if err := service.repository.Create(ctx, image); err != nil {
		return domain.RuntimeImage{}, err
	}
	return image, nil
}

func (service *Service) Get(ctx context.Context, organizationID, id string) (domain.RuntimeImage, error) {
	if service.repository == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Image Repository is required")
	}
	if strings.TrimSpace(organizationID) == "" || strings.TrimSpace(id) == "" {
		return domain.RuntimeImage{}, fmt.Errorf("Organization ID and Runtime Image ID are required")
	}
	return service.repository.Get(ctx, organizationID, id)
}

func (service *Service) List(ctx context.Context, query ListQuery) (Page, error) {
	if service.repository == nil {
		return Page{}, fmt.Errorf("Runtime Image Repository is required")
	}
	if strings.TrimSpace(query.OrganizationID) == "" {
		return Page{}, ErrInvalidPage
	}
	pageSize := query.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize < 1 || pageSize > maxPageSize {
		return Page{}, ErrInvalidPage
	}
	after, err := decodePageToken(query.Token)
	if err != nil {
		return Page{}, err
	}
	page, err := service.repository.List(ctx, domain.PageQuery{OrganizationID: query.OrganizationID, Limit: pageSize, After: after})
	if err != nil {
		return Page{}, err
	}
	nextToken := ""
	if page.HasMore {
		nextToken, err = encodePageToken(page.Items[len(page.Items)-1])
		if err != nil {
			return Page{}, err
		}
	}
	return Page{Items: page.Items, NextToken: nextToken}, nil
}

type pageToken struct {
	Runtime   domain.Runtime `json:"runtime"`
	CreatedAt time.Time      `json:"created_at"`
	ID        string         `json:"id"`
}

func encodePageToken(image domain.RuntimeImage) (string, error) {
	body, err := json.Marshal(pageToken{Runtime: image.Runtime, CreatedAt: image.CreatedAt, ID: image.ID})
	if err != nil {
		return "", fmt.Errorf("encode Runtime Image page: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodePageToken(token string) (*domain.PageCursor, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrInvalidPage
	}
	var cursor pageToken
	if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return nil, ErrInvalidPage
	}
	if cursor.Runtime != domain.Claude && cursor.Runtime != domain.Codex && cursor.Runtime != domain.Hermes && cursor.Runtime != domain.OpenClaw {
		return nil, ErrInvalidPage
	}
	return &domain.PageCursor{Runtime: cursor.Runtime, CreatedAt: cursor.CreatedAt.UTC(), ID: cursor.ID}, nil
}

func (service *Service) ChangeStatus(ctx context.Context, command ChangeStatusCommand) (domain.RuntimeImage, error) {
	if service.repository == nil || service.clock == nil {
		return domain.RuntimeImage{}, fmt.Errorf("Runtime Catalog dependencies are required")
	}
	if strings.TrimSpace(command.OrganizationID) == "" || strings.TrimSpace(command.ID) == "" || command.ExpectedVersion <= 0 {
		return domain.RuntimeImage{}, fmt.Errorf("Organization ID, Runtime Image ID, and expected version are required")
	}
	image, err := service.repository.Get(ctx, command.OrganizationID, command.ID)
	if err != nil {
		return domain.RuntimeImage{}, err
	}
	if image.Version != command.ExpectedVersion {
		return domain.RuntimeImage{}, domain.ErrConcurrentUpdate
	}
	verifiedEvidenceSHA256 := ""
	if command.Status == domain.Production {
		if service.evidence == nil {
			return domain.RuntimeImage{}, ErrEvidenceUnavailable
		}
		verified, err := service.evidence.Verify(ctx, command.ConformanceEvidenceKey, image)
		if err != nil {
			return domain.RuntimeImage{}, err
		}
		verifiedEvidenceSHA256 = verified.SHA256
	}
	originalVersion := image.Version
	if err := image.ChangeStatus(command.Status, command.BlockedReason, command.ConformanceEvidenceKey, verifiedEvidenceSHA256, service.clock.Now()); err != nil {
		return domain.RuntimeImage{}, err
	}
	if image.Version == originalVersion {
		return image, nil
	}
	if err := service.repository.UpdateStatus(ctx, image, originalVersion); err != nil {
		return domain.RuntimeImage{}, err
	}
	return image, nil
}
