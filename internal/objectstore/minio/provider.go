package minio

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"agent-platform/internal/objectstore"
	miniogo "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	defaultPartSize = 8 * 1024 * 1024
	checksumKey     = "agent-sha256"
	metadataKey     = "agent-metadata"
)

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	SessionToken string
	Bucket       string
	Secure       bool
	Prefix       string
	PartSize     uint64
}

type Provider struct {
	client   *miniogo.Client
	bucket   string
	prefix   string
	partSize uint64
	now      func() time.Time
}

func New(config Config) (*Provider, error) {
	if config.Endpoint == "" || config.Bucket == "" || config.AccessKey == "" || config.SecretKey == "" {
		return nil, fmt.Errorf("MinIO endpoint, bucket, access key, and secret key are required")
	}
	prefix, err := objectstore.NormalizePrefix(config.Prefix)
	if err != nil {
		return nil, err
	}
	client, err := miniogo.New(config.Endpoint, &miniogo.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, config.SessionToken),
		Secure: config.Secure,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}
	partSize := config.PartSize
	if partSize == 0 {
		partSize = defaultPartSize
	}
	if partSize < 5*1024*1024 {
		return nil, fmt.Errorf("MinIO multipart size must be at least 5 MiB")
	}
	return &Provider{
		client:   client,
		bucket:   config.Bucket,
		prefix:   prefix,
		partSize: partSize,
		now:      time.Now,
	}, nil
}

func (p *Provider) Put(ctx context.Context, key string, body io.Reader, options objectstore.PutOptions) (objectstore.Object, error) {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return objectstore.Object{}, err
	}
	upload, err := objectstore.PrepareUpload(ctx, body, options)
	if err != nil {
		return objectstore.Object{}, err
	}
	defer upload.Cleanup()
	encodedMetadata, err := objectstore.EncodeMetadata(options.Metadata)
	if err != nil {
		return objectstore.Object{}, err
	}
	_, err = p.client.PutObject(ctx, p.bucket, physicalKey, upload.File, options.Size, miniogo.PutObjectOptions{
		ContentType:  options.ContentType,
		UserMetadata: map[string]string{checksumKey: options.SHA256, metadataKey: encodedMetadata},
		PartSize:     p.partSize,
	})
	if err != nil {
		return objectstore.Object{}, mapError("put MinIO object", err)
	}
	return p.Stat(ctx, key)
}

func (p *Provider) Get(ctx context.Context, key string) (io.ReadCloser, objectstore.Object, error) {
	metadata, err := p.Stat(ctx, key)
	if err != nil {
		return nil, objectstore.Object{}, err
	}
	physicalKey, _ := p.physicalKey(key)
	reader, err := p.client.GetObject(ctx, p.bucket, physicalKey, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, objectstore.Object{}, mapError("get MinIO object", err)
	}
	return reader, metadata, nil
}

func (p *Provider) Stat(ctx context.Context, key string) (objectstore.Object, error) {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return objectstore.Object{}, err
	}
	info, err := p.client.StatObject(ctx, p.bucket, physicalKey, miniogo.StatObjectOptions{})
	if err != nil {
		return objectstore.Object{}, mapError("stat MinIO object", err)
	}
	checksum := metadataValue(info.UserMetadata, checksumKey)
	metadata, err := objectstore.DecodeMetadata(metadataValue(info.UserMetadata, metadataKey))
	if err != nil {
		return objectstore.Object{}, err
	}
	return objectstore.Object{
		Key:          key,
		Size:         info.Size,
		SHA256:       checksum,
		ContentType:  info.ContentType,
		Metadata:     metadata,
		LastModified: info.LastModified,
	}, nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	physicalKey, err := p.physicalKey(key)
	if err != nil {
		return err
	}
	if err := p.client.RemoveObject(ctx, p.bucket, physicalKey, miniogo.RemoveObjectOptions{}); err != nil {
		return mapError("delete MinIO object", err)
	}
	return nil
}

func (p *Provider) PresignGet(ctx context.Context, key string, expiresIn time.Duration) (objectstore.SignedURL, error) {
	if err := validateExpiry(expiresIn); err != nil {
		return objectstore.SignedURL{}, err
	}
	if _, err := p.Stat(ctx, key); err != nil {
		return objectstore.SignedURL{}, err
	}
	physicalKey, _ := p.physicalKey(key)
	signed, err := p.client.PresignedGetObject(ctx, p.bucket, physicalKey, expiresIn, url.Values{})
	if err != nil {
		return objectstore.SignedURL{}, mapError("sign MinIO object", err)
	}
	return objectstore.SignedURL{URL: signed.String(), ExpiresAt: p.now().UTC().Add(expiresIn)}, nil
}

func (p *Provider) DeleteExpired(ctx context.Context, query objectstore.LifecycleQuery) (int, error) {
	if query.Before.IsZero() {
		return 0, fmt.Errorf("lifecycle cutoff is required")
	}
	prefix := p.prefixed(query.Prefix)
	deleted := 0
	for info := range p.client.ListObjects(ctx, p.bucket, miniogo.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if info.Err != nil {
			return deleted, mapError("list MinIO objects", info.Err)
		}
		if !info.LastModified.Before(query.Before) {
			continue
		}
		if err := p.client.RemoveObject(ctx, p.bucket, info.Key, miniogo.RemoveObjectOptions{}); err != nil {
			return deleted, mapError("delete expired MinIO object", err)
		}
		deleted++
	}
	return deleted, nil
}

func (p *Provider) physicalKey(key string) (string, error) {
	if err := objectstore.ValidateKey(key); err != nil {
		return "", err
	}
	return p.prefixed(key), nil
}

func (p *Provider) prefixed(key string) string {
	if p.prefix == "" {
		return key
	}
	if key == "" {
		return p.prefix + "/"
	}
	return p.prefix + "/" + key
}

func validateExpiry(expiresIn time.Duration) error {
	if expiresIn <= 0 || expiresIn > 7*24*time.Hour {
		return objectstore.ErrInvalidExpiry
	}
	return nil
}

func mapError(operation string, err error) error {
	response := miniogo.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == 404 {
		return fmt.Errorf("%s: %w", operation, objectstore.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func metadataValue(metadata map[string]string, key string) string {
	for name, value := range metadata {
		if strings.EqualFold(name, key) {
			return value
		}
	}
	return ""
}
