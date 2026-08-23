package api

import (
	"context"
	"strings"
	"testing"
	"time"

	artifactv1 "agent-platform/backend/api/artifact/v1"
	artifactdomain "agent-platform/backend/internal/biz/artifact/domain"
	"agent-platform/backend/internal/biz/authz"
	identitydomain "agent-platform/backend/internal/biz/identity/domain"
	"agent-platform/backend/internal/objectstore"
)

type artifactServiceStub struct {
	artifact   artifactdomain.Artifact
	listCalled bool
	presignTTL time.Duration
	presignURL objectstore.SignedURL
}

func (stub *artifactServiceStub) Get(context.Context, string) (artifactdomain.Artifact, error) {
	return stub.artifact, nil
}
func (stub *artifactServiceStub) GetInScope(context.Context, string, authz.ReadScope) (artifactdomain.Artifact, error) {
	return stub.artifact, nil
}
func (stub *artifactServiceStub) ListByRun(context.Context, string) ([]artifactdomain.Artifact, error) {
	stub.listCalled = true
	return []artifactdomain.Artifact{stub.artifact}, nil
}
func (stub *artifactServiceStub) PresignDownload(_ context.Context, _ artifactdomain.Artifact, ttl time.Duration) (objectstore.SignedURL, error) {
	stub.presignTTL = ttl
	return stub.presignURL, nil
}

type artifactRunAccessFunc func(context.Context, string, string) error

func (function artifactRunAccessFunc) AuthorizeRunRead(ctx context.Context, token, runID string) error {
	return function(ctx, token, runID)
}

type artifactScopeFunc func(context.Context, string) (authz.ReadScope, error)

func (function artifactScopeFunc) ResolveReadScope(ctx context.Context, token string) (authz.ReadScope, error) {
	return function(ctx, token)
}

func TestRunArtifactListReauthorizesAndDoesNotExposeObjectKey(t *testing.T) {
	runID := "00000000-0000-4000-8000-000000000009"
	now := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	artifact, err := artifactdomain.New("00000000-0000-4000-8000-000000000010", runID, "diff", "runs/private/provider-key", 42, strings.Repeat("a", 64), "text/x-diff", map[string]string{"attempt": "1"}, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	store := &artifactServiceStub{artifact: artifact}
	service := &GeneratedServices{dependencies: Dependencies{
		Artifacts: store,
		Access: artifactRunAccessFunc(func(_ context.Context, _ string, gotRunID string) error {
			if gotRunID != runID {
				t.Fatalf("authorized Run = %q", gotRunID)
			}
			return nil
		}),
	}}

	response, err := service.ListRunArtifacts(context.Background(), &artifactv1.ListRunArtifactsRequest{RunId: runID})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Id != artifact.ID || response.Items[0].SizeBytes != 42 {
		t.Fatalf("Artifact response = %+v", response.Items)
	}
	if strings.Contains(response.String(), artifact.ObjectKey) {
		t.Fatalf("Artifact response leaked Object Key: %s", response)
	}
}

func TestRunArtifactListStopsBeforeStorageWhenAccessIsLost(t *testing.T) {
	store := &artifactServiceStub{}
	service := &GeneratedServices{dependencies: Dependencies{
		Artifacts: store,
		Access:    artifactRunAccessFunc(func(context.Context, string, string) error { return identitydomain.ErrForbidden }),
	}}

	_, err := service.ListRunArtifacts(context.Background(), &artifactv1.ListRunArtifactsRequest{RunId: "00000000-0000-4000-8000-000000000009"})
	if err == nil || store.listCalled {
		t.Fatalf("ListRunArtifacts() error=%v storage_called=%v", err, store.listCalled)
	}
}

func TestArtifactDownloadUsesScopedLookupAndFiveMinuteURL(t *testing.T) {
	now := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	artifact, err := artifactdomain.New("00000000-0000-4000-8000-000000000010", "00000000-0000-4000-8000-000000000009", "diff", "runs/private/provider-key", 42, strings.Repeat("a", 64), "text/x-diff", nil, now.Add(time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	store := &artifactServiceStub{artifact: artifact, presignURL: objectstore.SignedURL{URL: "https://objects.example/short-lived", ExpiresAt: now.Add(5 * time.Minute)}}
	service := &GeneratedServices{dependencies: Dependencies{
		Artifacts: store,
		ResourceAccess: artifactScopeFunc(func(context.Context, string) (authz.ReadScope, error) {
			return authz.ReadScope{OrganizationID: "org-1", TeamIDs: []string{"team-1"}}, nil
		}),
	}}

	response, err := service.GetArtifactDownload(context.Background(), &artifactv1.GetArtifactDownloadRequest{ArtifactId: artifact.ID})
	if err != nil {
		t.Fatal(err)
	}
	if response.Url != store.presignURL.URL || store.presignTTL != 5*time.Minute {
		t.Fatalf("download=%+v ttl=%v", response, store.presignTTL)
	}
}
