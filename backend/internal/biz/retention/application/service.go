package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"agent-platform/backend/internal/biz/retention/domain"
	"agent-platform/backend/internal/objectstore"
)

type Repository interface {
	ListExpiredArtifacts(context.Context, time.Time, time.Time, time.Time, int) ([]domain.Artifact, error)
	MarkArtifactDeleted(context.Context, string, time.Time) (bool, error)
	DeleteRunEvents(context.Context, time.Time, int) (int64, error)
	DeleteAuditEvents(context.Context, time.Time, int) (int64, error)
	DeleteIdempotencyKeys(context.Context, time.Time, int) (int64, error)
	ListExpiredWorkspaces(context.Context, time.Time, int) ([]domain.Workspace, error)
	MarkWorkspacePurged(context.Context, string, time.Time) (bool, error)
}

type WorkspaceRemover interface {
	Remove(context.Context, string) error
}

type Service struct {
	repository Repository
	objects    objectstore.Provider
	policy     domain.Policy
	now        func() time.Time
	workspaces WorkspaceRemover
}

func New(repository Repository, objects objectstore.Provider, policy domain.Policy) (*Service, error) {
	return NewWithWorkspaceRemover(repository, objects, nil, policy)
}

func NewWithWorkspaceRemover(repository Repository, objects objectstore.Provider, workspaces WorkspaceRemover, policy domain.Policy) (*Service, error) {
	if repository == nil || objects == nil {
		return nil, fmt.Errorf("Retention Repository and Object Store are required")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &Service{repository: repository, objects: objects, workspaces: workspaces, policy: policy, now: time.Now}, nil
}

func (service *Service) Sweep(ctx context.Context) (domain.Result, error) {
	now := service.now().UTC()
	workspaceCutoff := now.Add(-service.policy.WorkspacePeriod)
	artifacts, err := service.repository.ListExpiredArtifacts(ctx, now, now.Add(-service.policy.ArtifactPeriod), workspaceCutoff, service.policy.BatchSize)
	if err != nil {
		return domain.Result{}, fmt.Errorf("list expired Artifacts: %w", err)
	}
	result := domain.Result{}
	var sweepErrors []error
	for _, artifact := range artifacts {
		if err := service.objects.Delete(ctx, artifact.ObjectKey); err != nil && !errors.Is(err, objectstore.ErrNotFound) {
			sweepErrors = append(sweepErrors, fmt.Errorf("delete Artifact object %s: %w", artifact.ID, err))
			continue
		}
		marked, err := service.repository.MarkArtifactDeleted(ctx, artifact.ID, now)
		if err != nil {
			sweepErrors = append(sweepErrors, fmt.Errorf("mark Artifact %s deleted: %w", artifact.ID, err))
			continue
		}
		if marked {
			result.Artifacts++
		}
	}
	if service.workspaces != nil {
		workspaces, listErr := service.repository.ListExpiredWorkspaces(ctx, workspaceCutoff, service.policy.BatchSize)
		if listErr != nil {
			sweepErrors = append(sweepErrors, fmt.Errorf("list expired Workspaces: %w", listErr))
		} else {
			for _, workspace := range workspaces {
				if err := service.workspaces.Remove(ctx, workspace.Volume); err != nil {
					sweepErrors = append(sweepErrors, fmt.Errorf("delete Workspace for Session %s: %w", workspace.SessionID, err))
					continue
				}
				marked, err := service.repository.MarkWorkspacePurged(ctx, workspace.SessionID, now)
				if err != nil {
					sweepErrors = append(sweepErrors, fmt.Errorf("mark Workspace for Session %s purged: %w", workspace.SessionID, err))
					continue
				}
				if marked {
					result.Workspaces++
				}
			}
		}
	}
	result.RunEvents, err = service.repository.DeleteRunEvents(ctx, now.Add(-service.policy.RunEventPeriod), service.policy.BatchSize)
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("delete expired Run Events: %w", err))
	}
	result.AuditEvents, err = service.repository.DeleteAuditEvents(ctx, now.Add(-service.policy.AuditPeriod), service.policy.BatchSize)
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("delete expired Audit Events: %w", err))
	}
	result.IdempotencyKey, err = service.repository.DeleteIdempotencyKeys(ctx, now.Add(-service.policy.IdempotencyGrace), service.policy.BatchSize)
	if err != nil {
		sweepErrors = append(sweepErrors, fmt.Errorf("delete expired Idempotency Keys: %w", err))
	}
	return result, errors.Join(sweepErrors...)
}
