package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-platform/backend/internal/biz/authz"
)

var ErrNotFound = errors.New("Artifact not found")

type Artifact struct {
	ID, RunID, Kind, ObjectKey, SHA256, ContentType string
	SizeBytes                                       int64
	Metadata                                        map[string]string
	ExpiresAt                                       *time.Time
	DeletedAt                                       *time.Time
	CreatedAt                                       time.Time
}

func New(id, runID, kind, objectKey string, size int64, sha256, contentType string, metadata map[string]string, expiresAt, now time.Time) (Artifact, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(kind) == "" || strings.TrimSpace(objectKey) == "" || size < 0 || len(sha256) != 64 || strings.TrimSpace(contentType) == "" || expiresAt.IsZero() || !expiresAt.After(now) {
		return Artifact{}, fmt.Errorf("invalid Artifact registration")
	}
	expires := expiresAt.UTC()
	return Artifact{ID: id, RunID: runID, Kind: kind, ObjectKey: objectKey, SizeBytes: size, SHA256: sha256, ContentType: contentType, Metadata: cloneMetadata(metadata), ExpiresAt: &expires, CreatedAt: now.UTC()}, nil
}

func Restore(artifact Artifact) (Artifact, error) {
	if artifact.ID == "" || artifact.RunID == "" || artifact.Kind == "" || artifact.ObjectKey == "" || artifact.SizeBytes < 0 || len(artifact.SHA256) != 64 || artifact.ContentType == "" || artifact.CreatedAt.IsZero() {
		return Artifact{}, fmt.Errorf("invalid persisted Artifact")
	}
	artifact.Metadata = cloneMetadata(artifact.Metadata)
	return artifact, nil
}

type Repository interface {
	Create(context.Context, Artifact) error
	Get(context.Context, string) (Artifact, error)
	GetInScope(context.Context, string, authz.ReadScope) (Artifact, error)
	ListByRun(context.Context, string) ([]Artifact, error)
}

func cloneMetadata(metadata map[string]string) map[string]string {
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}
