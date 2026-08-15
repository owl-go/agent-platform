package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"sync"
	"time"

	"agent-platform/internal/objectstore"
)

type Provider struct {
	mu      sync.RWMutex
	objects map[string]storedObject
	now     func() time.Time
}

type storedObject struct {
	contents []byte
	metadata objectstore.Object
}

func New() *Provider {
	return &Provider{objects: make(map[string]storedObject), now: time.Now}
}

func (p *Provider) Put(ctx context.Context, key string, body io.Reader, options objectstore.PutOptions) (objectstore.Object, error) {
	if err := objectstore.ValidateKey(key); err != nil {
		return objectstore.Object{}, err
	}
	upload, err := objectstore.PrepareUpload(ctx, body, options)
	if err != nil {
		return objectstore.Object{}, err
	}
	defer upload.Cleanup()
	contents, err := io.ReadAll(upload.File)
	if err != nil {
		return objectstore.Object{}, fmt.Errorf("read upload spool: %w", err)
	}
	metadata := objectstore.Object{
		Key:          key,
		Size:         options.Size,
		SHA256:       options.SHA256,
		ContentType:  options.ContentType,
		Metadata:     cloneMap(options.Metadata),
		LastModified: p.now().UTC(),
	}
	p.mu.Lock()
	p.objects[key] = storedObject{contents: bytes.Clone(contents), metadata: metadata}
	p.mu.Unlock()
	return cloneObject(metadata), nil
}

func (p *Provider) Get(_ context.Context, key string) (io.ReadCloser, objectstore.Object, error) {
	if err := objectstore.ValidateKey(key); err != nil {
		return nil, objectstore.Object{}, err
	}
	p.mu.RLock()
	stored, ok := p.objects[key]
	p.mu.RUnlock()
	if !ok {
		return nil, objectstore.Object{}, objectstore.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(stored.contents))), cloneObject(stored.metadata), nil
}

func (p *Provider) Stat(_ context.Context, key string) (objectstore.Object, error) {
	if err := objectstore.ValidateKey(key); err != nil {
		return objectstore.Object{}, err
	}
	p.mu.RLock()
	stored, ok := p.objects[key]
	p.mu.RUnlock()
	if !ok {
		return objectstore.Object{}, objectstore.ErrNotFound
	}
	return cloneObject(stored.metadata), nil
}

func (p *Provider) Delete(_ context.Context, key string) error {
	if err := objectstore.ValidateKey(key); err != nil {
		return err
	}
	p.mu.Lock()
	delete(p.objects, key)
	p.mu.Unlock()
	return nil
}

func (p *Provider) PresignGet(_ context.Context, key string, expiresIn time.Duration) (objectstore.SignedURL, error) {
	if expiresIn <= 0 {
		return objectstore.SignedURL{}, objectstore.ErrInvalidExpiry
	}
	if _, err := p.Stat(context.Background(), key); err != nil {
		return objectstore.SignedURL{}, err
	}
	expiresAt := p.now().UTC().Add(expiresIn)
	values := url.Values{"expires": []string{expiresAt.Format(time.RFC3339Nano)}}
	return objectstore.SignedURL{URL: "memory:///" + url.PathEscape(key) + "?" + values.Encode(), ExpiresAt: expiresAt}, nil
}

func (p *Provider) DeleteExpired(_ context.Context, query objectstore.LifecycleQuery) (int, error) {
	if query.Before.IsZero() {
		return 0, fmt.Errorf("lifecycle cutoff is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	keys := make([]string, 0)
	for key, stored := range p.objects {
		if len(query.Prefix) <= len(key) && key[:len(query.Prefix)] == query.Prefix && stored.metadata.LastModified.Before(query.Before) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		delete(p.objects, key)
	}
	return len(keys), nil
}

func cloneObject(object objectstore.Object) objectstore.Object {
	object.Metadata = cloneMap(object.Metadata)
	return object
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
