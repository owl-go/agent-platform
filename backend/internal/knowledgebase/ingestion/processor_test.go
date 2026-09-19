package ingestion

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	workspaceapplication "agent-platform/backend/internal/biz/workspace/application"
	"agent-platform/backend/internal/knowledgebase/anythingllm"
	"agent-platform/backend/internal/objectstore"
)

type fakeRepository struct {
	job      *workspaceapplication.KnowledgeIngestionJob
	finished error
}

func (fake *fakeRepository) ClaimKnowledgeIngestionJob(context.Context) (*workspaceapplication.KnowledgeIngestionJob, error) {
	job := fake.job
	fake.job = nil
	return job, nil
}
func (fake *fakeRepository) FinishKnowledgeIngestionJob(_ context.Context, _ workspaceapplication.KnowledgeIngestionJob, err error) error {
	fake.finished = err
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

type fakeProvider struct{ uploaded bool }

func (fake *fakeProvider) EnsureWorkspace(context.Context, string) error { return nil }
func (fake *fakeProvider) DeleteWorkspace(context.Context, string) error { return nil }
func (fake *fakeProvider) UpsertRevision(context.Context, string, string, string, []byte) error {
	fake.uploaded = true
	return nil
}
func (fake *fakeProvider) RemoveRevision(context.Context, string, string) error { return nil }
func (fake *fakeProvider) Query(context.Context, string, int64, string, int, int) (anythingllm.Retrieval, error) {
	return anythingllm.Retrieval{}, nil
}

func TestProcessorAcceptsSourceOnlyAfterProviderIndexing(t *testing.T) {
	repository := &fakeRepository{job: &workspaceapplication.KnowledgeIngestionJob{ID: "job", KnowledgeBaseID: "base", RevisionID: "revision", ObjectKey: "knowledge/object", ContentType: "text/plain"}}
	provider := &fakeProvider{}
	processor, err := New(repository, fakeObjects{content: []byte("hello")}, provider)
	if err != nil {
		t.Fatal(err)
	}
	worked, err := processor.ProcessNext(context.Background())
	if err != nil || !worked || !provider.uploaded || repository.finished != nil {
		t.Fatalf("ProcessNext() = worked=%t err=%v uploaded=%t finished=%v", worked, err, provider.uploaded, repository.finished)
	}
}
