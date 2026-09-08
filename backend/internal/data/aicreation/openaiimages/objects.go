package openaiimages

import (
	"context"
	"io"

	"agent-platform/backend/internal/objectstore"
)

type ObjectSources struct{ provider objectstore.Provider }

func NewObjectSources(provider objectstore.Provider) ObjectSources {
	return ObjectSources{provider: provider}
}

func (sources ObjectSources) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	reader, _, err := sources.provider.Get(ctx, key)
	return reader, err
}
