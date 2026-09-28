package ingestion

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/objectstore"
)

type fakeRepository struct {
	job          *workspaceapplication.KnowledgeIngestionJob
	finished     error
	finishCalled bool
	previous     []string
}

func (fake *fakeRepository) SupersededKnowledgeRevisions(context.Context, string, string) ([]string, error) {
	if !fake.finishCalled {
		return nil, fmt.Errorf("cleanup before Ready commit")
	}
	return fake.previous, nil
}

func (fake *fakeRepository) ClaimKnowledgeIngestionJob(context.Context) (*workspaceapplication.KnowledgeIngestionJob, error) {
	job := fake.job
	fake.job = nil
	return job, nil
}
func (fake *fakeRepository) FinishKnowledgeIngestionJob(_ context.Context, _ workspaceapplication.KnowledgeIngestionJob, err error) error {
	fake.finished = err
	fake.finishCalled = true
	return nil
}

type fakeObjects struct{ content []byte }

func (fake fakeObjects) Put(context.Context, string, io.Reader, objectstore.PutOptions) (objectstore.Object, error) {
	panic("unexpected Put")
}
func (fake fakeObjects) Get(context.Context, string) (io.ReadCloser, objectstore.Object, error) {
	return io.NopCloser(bytes.NewReader(fake.content)), objectstore.Object{Size: int64(len(fake.content)), SHA256: "digest"}, nil
}
func (fake fakeObjects) Stat(context.Context, string) (objectstore.Object, error) {
	return objectstore.Object{}, nil
}
func (fake fakeObjects) Delete(context.Context, string) error { return nil }
func (fake fakeObjects) PresignGet(context.Context, string, time.Duration) (objectstore.SignedURL, error) {
	return objectstore.SignedURL{}, nil
}
func (fake fakeObjects) DeleteExpired(context.Context, objectstore.LifecycleQuery) (int, error) {
	return 0, nil
}

type fakeProvider struct {
	uploaded bool
	removed  []string
}

func (fake *fakeProvider) UpsertRevision(context.Context, string, string, string, []byte) error {
	fake.uploaded = true
	return nil
}
func (fake *fakeProvider) RemoveRevision(_ context.Context, _, revisionID string) error {
	fake.removed = append(fake.removed, revisionID)
	return nil
}

func TestProcessorAcceptsSourceOnlyAfterProviderIndexing(t *testing.T) {
	repository := &fakeRepository{job: &workspaceapplication.KnowledgeIngestionJob{ID: "job", KnowledgeBaseID: "base", RevisionID: "revision", ObjectKey: "knowledge/object", ContentType: "text/plain"}, previous: []string{"old-revision"}}
	provider := &fakeProvider{}
	processor, err := New(repository, fakeObjects{content: []byte("hello")}, provider)
	if err != nil {
		t.Fatal(err)
	}
	worked, err := processor.ProcessNext(context.Background())
	if err != nil || !worked || !provider.uploaded || repository.finished != nil || len(provider.removed) != 1 || provider.removed[0] != "old-revision" {
		t.Fatalf("ProcessNext() = worked=%t err=%v uploaded=%t finished=%v", worked, err, provider.uploaded, repository.finished)
	}
}
