package application

import (
	"context"
	"fmt"
	"strings"

	"agent-platform/backend/internal/biz/audit/domain"
)

type Service struct{ repository domain.Repository }

func New(repository domain.Repository) *Service { return &Service{repository: repository} }

func (service *Service) Search(ctx context.Context, query domain.Query) ([]domain.Event, error) {
	if service.repository == nil || strings.TrimSpace(query.OrganizationID) == "" || strings.TrimSpace(query.TeamID) == "" {
		return nil, fmt.Errorf("Audit Repository and organization/Team scope are required")
	}
	if query.Limit <= 0 || query.Limit > 200 {
		return nil, fmt.Errorf("Audit search limit must be between 1 and 200")
	}
	if query.CreatedFrom != nil && query.CreatedTo != nil && query.CreatedFrom.After(*query.CreatedTo) {
		return nil, fmt.Errorf("Audit search start time must not follow end time")
	}
	if query.Outcome != "" && query.Outcome != "succeeded" && query.Outcome != "failed" {
		return nil, fmt.Errorf("Audit outcome must be succeeded or failed")
	}
	return service.repository.Search(ctx, query)
}
