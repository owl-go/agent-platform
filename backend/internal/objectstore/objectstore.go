package objectstore

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	ErrNotFound         = errors.New("object not found")
	ErrChecksumMismatch = errors.New("object checksum mismatch")
	ErrInvalidKey       = errors.New("invalid object key")
	ErrInvalidExpiry    = errors.New("invalid signed URL expiry")
)

type PutOptions struct {
	Size        int64
	SHA256      string
	ContentType string
	Metadata    map[string]string
}

type Object struct {
	Key          string
	Size         int64
	SHA256       string
	ContentType  string
	Metadata     map[string]string
	LastModified time.Time
}

type SignedURL struct {
	URL       string
	ExpiresAt time.Time
}

type LifecycleQuery struct {
	Prefix string
	Before time.Time
}

type Provider interface {
	Put(ctx context.Context, key string, body io.Reader, options PutOptions) (Object, error)
	Get(ctx context.Context, key string) (io.ReadCloser, Object, error)
	Stat(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expiresIn time.Duration) (SignedURL, error)
	DeleteExpired(ctx context.Context, query LifecycleQuery) (int, error)
}
