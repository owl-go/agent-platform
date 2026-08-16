package platform

import (
	"context"
	"fmt"
	"time"

	platformv1 "agent-platform/backend/api/platform/v1"
)

type ReadinessChecker interface {
	PingContext(context.Context) error
}

type Service struct {
	platformv1.UnimplementedHealthServiceServer
	readiness ReadinessChecker
}

func New(readiness ReadinessChecker) (*Service, error) {
	if readiness == nil {
		return nil, fmt.Errorf("Readiness Checker is required")
	}
	return &Service{readiness: readiness}, nil
}

func (service *Service) Health(context.Context, *platformv1.HealthRequest) (*platformv1.HealthResponse, error) {
	return &platformv1.HealthResponse{Status: "ok"}, nil
}

func (service *Service) Ready(ctx context.Context, _ *platformv1.ReadyRequest) (*platformv1.ReadyResponse, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	status := "ready"
	if err := service.readiness.PingContext(checkCtx); err != nil {
		status = "unavailable"
	}
	return &platformv1.ReadyResponse{Status: status}, nil
}
